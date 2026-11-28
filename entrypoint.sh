#!/bin/sh
# Подготовка окружения и запуск awgd. Одинаково работает в Docker и в контейнере
# RouterOS (MikroTik), где нет ни sysctls, ни tmpfs, ни cap_add в конфигурации:
# всё, что Docker делает снаружи, здесь делается изнутри и по возможности.
set -eu

# Строки дублируются в RUN_DIR/entrypoint.log: awgd переносит их на страницу «Журнал».
EPLOG="${RUN_DIR:-/run/awg31}/entrypoint.log"
mkdir -p "${EPLOG%/*}" 2>/dev/null || true
: > "$EPLOG" 2>/dev/null || true
log() {
    printf '[entrypoint] %s\n' "$*" >&2
    printf '%s %s\n' "$(date +%s)" "$*" >> "$EPLOG" 2>/dev/null || true
}

# --- /dev/net/tun: amneziawg-go работает в userspace и без него не поднимется ---
ensure_tun() {
    if [ ! -c /dev/net/tun ]; then
        mkdir -p /dev/net
        if ! mknod /dev/net/tun c 10 200 2>/dev/null; then
            log "ERROR: /dev/net/tun is missing. Docker: --device /dev/net/tun (in compose: devices)"
            exit 1
        fi
        chmod 0600 /dev/net/tun 2>/dev/null || true
    fi
    if ! (exec 3<>/dev/net/tun) 2>/dev/null; then
        log "ERROR: /dev/net/tun exists but cannot be opened (NET_ADMIN is required)"
        exit 1
    fi
}

# --- sysctl внутри своего сетевого пространства (на RouterOS их больше негде задать) ---
set_sysctl() {
    key="$1"; want="$2"
    cur=$(sysctl -n "$key" 2>/dev/null || echo "")
    [ "$cur" = "$want" ] && return 0
    if sysctl -w "$key=$want" >/dev/null 2>&1; then
        return 0
    fi
    return 1
}

prepare_net() {
    if ! set_sysctl net.ipv4.ip_forward 1; then
        log "WARNING: net.ipv4.ip_forward != 1, gateway mode will not work (Docker: sysctls in compose)"
    fi
    # awg-quick ставит src_valid_mark сам при AllowedIPs = 0.0.0.0/0 и падает, если
    # /proc/sys только для чтения; задаём заранее, чтобы ошибка была понятной.
    if ! set_sysctl net.ipv4.conf.all.src_valid_mark 1; then
        log "WARNING: net.ipv4.conf.all.src_valid_mark != 1, a full tunnel (0.0.0.0/0) will not come up"
    fi
    # Группы туннелей (балансировка): ядро должно делить потоки по портам, иначе весь
    # трафик одного клиента уйдёт в один туннель. Нужна только группам, поэтому не ошибка.
    set_sysctl net.ipv4.fib_multipath_hash_policy 1 || true
    set_sysctl net.ipv6.fib_multipath_hash_policy 1 || true
}

# --- IPv6 (AWG_ENABLE_IPV6, по умолчанию on). Перенесено из a-amneziawg-mikrotik:
#     accept_ra=2 ставится ДО forwarding=1, иначе ядро выбросит маршруты из RA;
#     если у LAN-интерфейса нет глобального IPv6 (типично для veth RouterOS),
#     выводится ULA fd00:<IPv4>::/64 с шлюзом .1 — его же адрес ставит
#     router-setup.rsc на стороне роутера. ---
lan_iface() {
    if [ -n "${AWG_LAN_IFACE:-}" ]; then printf '%s\n' "$AWG_LAN_IFACE"; return; fi
    ip -o -4 route show default 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="dev"){sub(/@.*/,"",$(i+1)); print $(i+1); exit}}'
}

prepare_ipv6() {
    if [ "${AWG_ENABLE_IPV6:-on}" = "off" ]; then
        set_sysctl net.ipv6.conf.all.forwarding 0 || true
        set_sysctl net.ipv6.conf.all.disable_ipv6 1 || true
        set_sysctl net.ipv6.conf.default.disable_ipv6 1 || true
        return 0
    fi
    set_sysctl net.ipv6.conf.all.disable_ipv6 0 || true
    set_sysctl net.ipv6.conf.default.disable_ipv6 0 || true
    if [ "$(sysctl -n net.ipv6.conf.all.disable_ipv6 2>/dev/null || echo 1)" != "0" ]; then
        log "IPv6 is unavailable in the container: the tunnel will work over IPv4 only"
        return 0
    fi
    set_sysctl net.ipv6.conf.all.accept_ra 2 || true
    set_sysctl net.ipv6.conf.default.accept_ra 2 || true
    lan=$(lan_iface)
    case "$lan" in ''|*[!A-Za-z0-9_.-]*) lan="" ;; esac
    if [ -n "$lan" ]; then
        set_sysctl "net.ipv6.conf.$lan.disable_ipv6" 0 || true
        set_sysctl "net.ipv6.conf.$lan.accept_ra" 2 || true
        set_sysctl "net.ipv6.conf.$lan.accept_ra_defrtr" 1 || true
    fi
    set_sysctl net.ipv6.conf.all.forwarding 1 || log "WARNING: could not enable IPv6 forwarding"
    set_sysctl net.ipv6.conf.default.forwarding 1 || true

    [ -n "$lan" ] || return 0
    if ip -6 -o addr show dev "$lan" scope global 2>/dev/null | grep -q inet6; then
        return 0
    fi
    v4=$(ip -o -4 addr show dev "$lan" 2>/dev/null | awk '{print $4; exit}')
    gw=$(ip -o -4 route show default dev "$lan" 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="via"){print $(i+1); exit}}')
    [ -n "$v4" ] && [ -n "$gw" ] || return 0
    v4="${v4%%/*}"
    o123=$(printf '%s' "$v4" | awk -F. '{print $1":"$2":"$3}')
    o4=${v4##*.}
    g4=${gw##*.}
    ip -6 addr add "fd00:${o123}::${o4}/64" dev "$lan" 2>/dev/null || true
    ip -6 route add default via "fd00:${o123}::${g4}" dev "$lan" metric 2048 2>/dev/null || true
    log "IPv6: $lan has no global address, assigned ULA fd00:${o123}::${o4}/64 (gateway fd00:${o123}::${g4})"
}

# --- правила ip rule: в контейнере RouterOS встречаются лишние «lookup main» с
#     приоритетом ниже 32766, из-за которых awg-quick не может увести трафик в
#     туннель. Возвращаем стандартный набор. Перенесено из a-amneziawg-mikrotik. ---
fix_ip_rules() {
    ip -4 rule show 2>/dev/null | awk '
        /from all lookup main$/    && $1+0 < 32766 {gsub(":", "", $1); print $1}
        /from all lookup default$/ && $1+0 < 32767 {gsub(":", "", $1); print $1}
    ' | while read -r prio; do
        [ -n "$prio" ] && ip -4 rule del priority "$prio" 2>/dev/null || true
    done
    # без «lookup local» контейнер не отвечает ни роутеру, ни на 127.0.0.1 (CHR, 0.9.0)
    ip -4 rule show 2>/dev/null | grep -q 'lookup local' || { ip -4 rule add priority 0 lookup local 2>/dev/null && log "ip rule 'lookup local' was missing, restored"; } || true
    ip -6 rule show 2>/dev/null | grep -q 'lookup local' || ip -6 rule add priority 0 lookup local 2>/dev/null || true
    ip -4 rule show 2>/dev/null | grep -q 'from all lookup main' || ip -4 rule add lookup main priority 32766 2>/dev/null || true
    ip -4 rule show 2>/dev/null | grep -q 'from all lookup default' || ip -4 rule add lookup default priority 32767 2>/dev/null || true
}

# --- openresolv отказывается дописывать resolv.conf, созданный не им; awg-quick
#     вызывает resolvconf, если в профиле есть строка DNS. ---
fix_resolvconf() {
    [ -e /etc/resolv.conf ] && [ -w /etc/resolv.conf ] || return 0
    if [ ! -s /etc/resolv.conf ]; then
        printf '# Generated by resolvconf\n' > /etc/resolv.conf 2>/dev/null || true
    elif ! grep -q '^# Generated by resolvconf' /etc/resolv.conf; then
        tmp="/tmp/resolv.conf.$$"
        { printf '# Generated by resolvconf\n'; cat /etc/resolv.conf; } > "$tmp" && cat "$tmp" > /etc/resolv.conf || true
        rm -f "$tmp"
    fi
}

# --- рабочие копии конфигов с ключами: на RouterOS tmpfs нет, каталог живёт на
#     диске роутера, поэтому после перезапуска удаляем всё, что осталось. ---
clean_rundir() {
    dir="${RUN_DIR:-/run/awg31}"
    mkdir -p "$dir"
    chmod 0700 "$dir"
    find "$dir" -maxdepth 1 -type f -name '*.conf' -exec rm -f {} + 2>/dev/null || true
    if ! awk -v d="$dir" '$2 == d && $3 == "tmpfs" {f=1} END {exit !f}' /proc/mounts 2>/dev/null; then
        log "note: $dir is not tmpfs (normal on RouterOS): working copies of configs are written to disk and removed when the tunnel stops"
    fi
}

ensure_tun
prepare_net
prepare_ipv6
fix_ip_rules
fix_resolvconf
clean_rundir
log "architecture: $(uname -m)"

exec /usr/bin/awgd "$@"

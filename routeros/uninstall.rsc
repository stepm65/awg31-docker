# ============================================================================
#  uninstall.rsc — полное удаление awg31 с роутера (откат install-container.rsc и
#  router-setup.rsc). Идемпотентно: повторный запуск безопасен.
#
#  Удаляет только объекты awg31 (по точным именам и комментариям awg31_*): контейнер awg31, его
#  veth-awg31 с адресами, списки envs/mounts, правила mangle/nat/filter (IPv4 и IPv6), маршруты и
#  таблицу маршрутизации awg31, Netwatch awg31_health, список адресов awg31-local и записи списка
#  awg31 с комментарием awg31*. Остальные контейнеры, правила и маркировка роутера не затрагиваются.
#
#  Файлы (образ .tar, каталог данных awg31/data с профилями и ключами) и ВСЕ записи списка awg31
#  (в том числе добавленные вручную) удаляются только с флагом:
#    :global AWG31PURGE true
#  Если список адресов назывался иначе, задайте то же имя: :global AWG31LIST "my-vpn"
#  Запуск: /import uninstall.rsc
#
#  Не трогает (скрипты могли это изменить, прежнее состояние им неизвестно):
#   - правило fasttrack: router-setup.rsc добавил ему connection-mark=no-mark; посмотрите
#     /ip/firewall/filter/print where action=fasttrack-connection и уберите условие, если его не
#     было раньше: /ip/firewall/filter/set <номер> !connection-mark
#   - /ipv6/settings forward=yes, списки интерфейсов WAN/LAN и RAM-диск tmp1.
#  Установленное через /app удаляется командами /app/print и /app/remove.
# ============================================================================
{
:global AWG31PURGE
:local purge false
:if ([:typeof $AWG31PURGE] = "bool") do={ :set purge $AWG31PURGE }
:global AWG31LIST
:local list "awg31"
:if ([:typeof $AWG31LIST] = "str") do={ :if ([:len $AWG31LIST] > 0) do={ :set list $AWG31LIST } }
:put ("awg31 removal: address list " . $list . ", delete files and the whole list: " . $purge)

# --- 1. Netwatch (чтобы он не переключал маршрут во время удаления) ---
:do {
  :local n [:len [/tool/netwatch/find comment="awg31_health"]]
  /tool/netwatch/remove [find comment="awg31_health"]
  :put ("netwatch: removed " . $n)
} on-error={ :put "netwatch: skipped" }

# --- 2. Контейнер: остановить, дождаться остановки, удалить ---
:do {
  :local cid [/container/find name="awg31"]
  :if ([:len $cid] > 0) do={
    :do { /container/stop $cid } on-error={}
    :local t 0
    :while ((([:len [/container/find where name="awg31" status="running"]] > 0) || ([:len [/container/find where name="awg31" status="stopping"]] > 0)) && ($t < 60)) do={
      :delay 1s
      :set t ($t + 1)
    }
    :do { /container/remove [find name="awg31"] } on-error={ :put "container: remove failed, run /container/remove awg31 by hand" }
    :set t 0
    :while (([:len [/container/find name="awg31"]] > 0) && ($t < 60)) do={
      :delay 1s
      :set t ($t + 1)
    }
    :put "container awg31: removed"
  } else={
    :put "container awg31: not found"
  }
} on-error={ :put "container: skipped (container package missing?)" }

# --- 3. Правила: mangle, nat, filter (сначала mangle: таблица awg31 нужна им как routing-mark) ---
:do { :local n [:len [/ip/firewall/mangle/find comment~"^awg31_"]]; /ip/firewall/mangle/remove [find comment~"^awg31_"]; :put ("ip mangle: removed " . $n) } on-error={ :put "ip mangle: skipped" }
:do { :local n [:len [/ip/firewall/nat/find comment~"^awg31_"]]; /ip/firewall/nat/remove [find comment~"^awg31_"]; :put ("ip nat: removed " . $n) } on-error={ :put "ip nat: skipped" }
:do { :local n [:len [/ip/firewall/filter/find comment~"^awg31_"]]; /ip/firewall/filter/remove [find comment~"^awg31_"]; :put ("ip filter: removed " . $n) } on-error={ :put "ip filter: skipped" }
:do { :local n [:len [/ipv6/firewall/mangle/find comment~"^awg31_"]]; /ipv6/firewall/mangle/remove [find comment~"^awg31_"]; :put ("ipv6 mangle: removed " . $n) } on-error={ :put "ipv6 mangle: skipped (IPv6 disabled)" }
:do { :local n [:len [/ipv6/firewall/nat/find comment~"^awg31_"]]; /ipv6/firewall/nat/remove [find comment~"^awg31_"]; :put ("ipv6 nat: removed " . $n) } on-error={ :put "ipv6 nat: skipped" }
:do { :local n [:len [/ipv6/firewall/filter/find comment~"^awg31_"]]; /ipv6/firewall/filter/remove [find comment~"^awg31_"]; :put ("ipv6 filter: removed " . $n) } on-error={ :put "ipv6 filter: skipped" }

# --- 4. Маршруты и таблица маршрутизации awg31 ---
:do { /ip/route/remove [find comment~"^awg31_"] } on-error={}
:do { /ipv6/route/remove [find comment~"^awg31_"] } on-error={}
:do {
  /routing/table/remove [find name="awg31"]
  :put "routing table awg31: removed"
} on-error={ :put "routing table awg31: not removed (is it still used by a rule of yours?)" }

# --- 5. Списки адресов ---
:do { /ip/firewall/address-list/remove [find where list="awg31-local" dynamic=no] } on-error={}
:do { /ipv6/firewall/address-list/remove [find where list="awg31-local" dynamic=no] } on-error={}
:if ($purge) do={
  :do { /ip/firewall/address-list/remove [find where list=$list dynamic=no] } on-error={}
  :do { /ipv6/firewall/address-list/remove [find where list=$list dynamic=no] } on-error={}
  :put ("address list " . $list . ": removed (all entries)")
} else={
  :do { /ip/firewall/address-list/remove [find where list=$list comment~"^awg31"] } on-error={}
  :do { /ipv6/firewall/address-list/remove [find where list=$list comment~"^awg31"] } on-error={}
  :put ("address list " . $list . ": removed entries with an awg31 comment (your own entries are kept; AWG31PURGE true removes all)")
}

# --- 6. Параметры контейнера и veth ---
:do { /container/envs/remove [find list="awg31"] } on-error={}
:do { /container/mounts/remove [find list="awg31-data"] } on-error={}
:do { /ip/address/remove [find interface="veth-awg31"] } on-error={}
:do { /ipv6/address/remove [find interface="veth-awg31"] } on-error={}
:do { /interface/veth/remove [find name="veth-awg31"] } on-error={}
:put "container envs/mounts, veth-awg31 and its addresses: removed"

# --- 7. Файлы (только с AWG31PURGE true): образ, каталоги awg31 и данные ---
:if ($purge) do={
  :for pass from=1 to=3 do={
    :foreach f in=[/file/find where name~"^awg31" or name~"/awg31"] do={
      :do { /file/remove $f } on-error={}
    }
  }
  :local rest [:len [/file/find where name~"^awg31" or name~"/awg31"]]
  :if ($rest = 0) do={ :put "files: removed" } else={ :put ("files: " . $rest . " left, remove them in Winbox -> Files (or reboot: tmp1 is cleared)") }
} else={
  :put "files (image .tar, awg31/data with profiles and keys) are kept; to delete them: :global AWG31PURGE true, then /import uninstall.rsc"
}

# --- 8. Проверка ---
:local left 0
:do { :set left ($left + [:len [/container/find name="awg31"]]) } on-error={}
:set left ($left + [:len [/interface/veth/find name="veth-awg31"]])
:set left ($left + [:len [/ip/firewall/mangle/find comment~"^awg31_"]])
:set left ($left + [:len [/ip/firewall/nat/find comment~"^awg31_"]])
:set left ($left + [:len [/ip/firewall/filter/find comment~"^awg31_"]])
:set left ($left + [:len [/ip/route/find comment~"^awg31_"]])
:set left ($left + [:len [/routing/table/find name="awg31"]])
:set left ($left + [:len [/tool/netwatch/find comment="awg31_health"]])
:do { :set left ($left + [:len [/ipv6/firewall/mangle/find comment~"^awg31_"]]) } on-error={}
:do { :set left ($left + [:len [/ipv6/firewall/nat/find comment~"^awg31_"]]) } on-error={}
:do { :set left ($left + [:len [/ipv6/firewall/filter/find comment~"^awg31_"]]) } on-error={}
:do { :set left ($left + [:len [/ipv6/route/find comment~"^awg31_"]]) } on-error={}
:put "==="
:if ($left = 0) do={
  :put "DONE: no awg31 objects left (container, veth, mangle/nat/filter, routes, table, netwatch)"
} else={
  :put ("WARNING: " . $left . " awg31 objects left: /container/print, /interface/veth/print, /ip/firewall/mangle/print where comment~\"awg31\"")
}
:put "Not changed: fasttrack rule, /ipv6/settings, interface lists WAN/LAN, disk tmp1 (see the header of uninstall.rsc)"

# Переменные консоли удаляем последними: выше они ещё нужны
:do { /system/script/environment/remove [find where name~"^AWG31"] } on-error={}
}

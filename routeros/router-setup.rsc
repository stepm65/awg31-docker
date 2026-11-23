# ============================================================================
#  router-setup.rsc — маршрутизация RouterOS через контейнер awg31
#  Идемпотентно: повторный запуск ничего не дублирует.
#
#  Что делает:
#   - таблица маршрутизации awg31: default -> контейнер + blackhole-страховка
#     (если контейнер или туннель недоступен, трафик из списка не уходит мимо VPN);
#   - mangle: в туннель идёт только трафик из LAN к адресам из address-list «awg31»;
#   - fasttrack не перехватывает помеченные соединения;
#   - srcnat для самого контейнера, разрешение ему выхода в интернет (правила awg31_icmp и
#     awg31_out только для его veth) и запрет новых соединений к нему с WAN;
#   - заготовка Netwatch по пробе здоровья :8081 (выключена);
#   - то же для IPv6 (адрес контейнера fd00:192:168:253::2), если IPv6 включён.
#
#  container IP: по умолчанию 192.168.253.2 (так его создаёт install-container.rsc).
#  Для /app RouterOS назначает IP сам (см. /container/print) — задайте его заранее:
#    :global AWG31IP "172.18.0.2"
#    :global AWG31IP6 "fd00:172:18:0::2"   (или "" — без IPv6)
#
#  Свой address-list вместо «awg31» (тот же, что указан на странице «RouterOS»
#  в консоли контейнера; пригодится, если список у вас уже есть):
#    :global AWG31LIST "my-vpn"
#
#  Весь интернет-трафик LAN через контейнер (тогда решают правила на странице
#  «Маршрутизация»: geosite, geoip, домены), а не только список awg31:
#    :global AWG31ALL true
#  Частные сети (RFC 1918, CGNAT, link-local, multicast) в контейнер не идут.
#  Запуск: /import router-setup.rsc
#
#  ВНИМАНИЕ: скрипт не проверялся на живом роутере; логика повторяет
#  router-setup.rsc из a-amneziawg-mikrotik.
# ============================================================================
{
:global AWG31IP
:local ip "192.168.253.2"
:if ([:typeof $AWG31IP] = "str") do={ :if ([:len $AWG31IP] > 0) do={ :set ip $AWG31IP } }
:global AWG31IP6
:local ip6 "fd00:192:168:253::2"
:if ([:typeof $AWG31IP6] = "str") do={ :set ip6 $AWG31IP6 }
:global AWG31ALL
:local all false
:if ([:typeof $AWG31ALL] = "bool") do={ :set all $AWG31ALL }
:global AWG31LIST
:local list "awg31"
:if ([:typeof $AWG31LIST] = "str") do={ :if ([:len $AWG31LIST] > 0) do={ :set list $AWG31LIST } }
:put ("container IP: " . $ip . " / " . $ip6)
:if ($all) do={ :put "mode: ALL LAN internet traffic goes through the container" } else={ :put ("mode: only addresses from the list " . $list) }

# --- 0. Список «не уводить»: частные и служебные сети ---
:foreach n in={"10.0.0.0/8";"172.16.0.0/12";"192.168.0.0/16";"100.64.0.0/10";"169.254.0.0/16";"224.0.0.0/4";"127.0.0.0/8"} do={
  :if ([:len [/ip/firewall/address-list/find list="awg31-local" address=$n]] = 0) do={
    /ip/firewall/address-list/add list="awg31-local" address=$n comment="awg31: do not send to container"
  }
}

# --- 1. Списки интерфейсов (стандартные LAN/WAN) ---
:if ([:len [/interface/list/find name="WAN"]] = 0) do={ /interface/list/add name="WAN"; :put "+ list WAN created (add your WAN ports to it!)" }
:if ([:len [/interface/list/find name="LAN"]] = 0) do={ /interface/list/add name="LAN"; :put "+ list LAN created (add your bridge/LAN to it!)" }

# --- 2. Таблица и маршруты ---
:if ([:len [/routing/table/find name="awg31"]] = 0) do={ /routing/table/add name="awg31" fib; :put "+ routing table awg31" }
:if ([:len [/ip/route/find comment="awg31_default"]] = 0) do={
  /ip/route/add dst-address=0.0.0.0/0 gateway=$ip routing-table="awg31" comment="awg31_default"
  :put ("+ default -> " . $ip . " in table awg31")
} else={
  /ip/route/set [find comment="awg31_default"] gateway=$ip
}
:if ([:len [/ip/route/find comment="awg31_bh"]] = 0) do={
  /ip/route/add blackhole dst-address=0.0.0.0/0 distance=254 routing-table="awg31" comment="awg31_bh"
  :put "+ blackhole in table awg31 (no tunnel = no leak around the VPN)"
}

# --- 3. fasttrack не должен съедать помеченные соединения ---
:do { /ip/firewall/filter/set [find where action=fasttrack-connection] connection-mark=no-mark } on-error={}

# --- 4. mangle: выборочная маршрутизация по address-list (по умолчанию «awg31») ---
:if ([:len [/ip/firewall/mangle/find comment="awg31_accept_container"]] = 0) do={
  /ip/firewall/mangle/add chain=prerouting action=accept src-address=$ip comment="awg31_accept_container"
}
:if ([:len [/ip/firewall/mangle/find comment="awg31_mark_conn"]] = 0) do={
  /ip/firewall/mangle/add chain=prerouting action=mark-connection in-interface-list=LAN connection-mark=no-mark \
    connection-state=new dst-address-type=!local dst-address-list=$list new-connection-mark="awg31" passthrough=yes \
    comment="awg31_mark_conn"
}
# режим «весь трафик»: вместо списка — всё, кроме awg31-local
:if ($all) do={
  /ip/firewall/mangle/set [find comment="awg31_mark_conn"] dst-address-list="!awg31-local"
} else={
  /ip/firewall/mangle/set [find comment="awg31_mark_conn"] dst-address-list=$list
}
:if ([:len [/ip/firewall/mangle/find comment="awg31_route"]] = 0) do={
  /ip/firewall/mangle/add chain=prerouting action=mark-routing in-interface-list=LAN connection-mark="awg31" \
    new-routing-mark="awg31" passthrough=no comment="awg31_route"
}
:if ([:len [/ip/firewall/mangle/find comment="awg31_accept_container"]] > 0) do={
  /ip/firewall/mangle/set [find comment="awg31_accept_container"] src-address=$ip
}

# --- 5. srcnat контейнера, выход контейнера в интернет и защита консоли от WAN ---
:if ([:len [/ip/firewall/nat/find comment="awg31_nat"]] = 0) do={
  /ip/firewall/nat/add chain=srcnat action=masquerade src-address=$ip out-interface-list=WAN comment="awg31_nat"
} else={
  /ip/firewall/nat/set [find comment="awg31_nat"] src-address=$ip
}
# Контейнеру нужен выход наружу (подписка панели, рукопожатия с VPS). Стандартный firewall
# режет ICMP к роутеру и forward с интерфейса вне LAN, поэтому разрешаем только veth контейнера.
# Не добавляйте veth контейнера в списки WAN или LAN: из WAN forward режется, а у LAN
# свои mangle-правила.
:local vif ""
:do {
  :local vid [/interface/veth/find where address~("^" . $ip . "/")]
  :if ([:len $vid] > 0) do={ :set vif [/interface/veth/get [:pick $vid 0] name] }
} on-error={}
:if ([:len $vif] > 0) do={
  :local first [/ip/firewall/filter/find]
  :if ([:len [/ip/firewall/filter/find comment="awg31_icmp"]] = 0) do={
    :if ([:len $first] > 0) do={
      /ip/firewall/filter/add chain=input action=accept protocol=icmp in-interface=$vif comment="awg31_icmp" place-before=[:pick $first 0]
    } else={
      /ip/firewall/filter/add chain=input action=accept protocol=icmp in-interface=$vif comment="awg31_icmp"
    }
  } else={
    /ip/firewall/filter/set [find comment="awg31_icmp"] in-interface=$vif
  }
  :if ([:len [/ip/firewall/filter/find comment="awg31_out"]] = 0) do={
    :if ([:len $first] > 0) do={
      /ip/firewall/filter/add chain=forward action=accept in-interface=$vif out-interface-list=WAN comment="awg31_out" place-before=[:pick $first 0]
    } else={
      /ip/firewall/filter/add chain=forward action=accept in-interface=$vif out-interface-list=WAN comment="awg31_out"
    }
  } else={
    /ip/firewall/filter/set [find comment="awg31_out"] in-interface=$vif
  }
  :put ("+ firewall: container (" . $vif . ") may reach the internet: awg31_icmp, awg31_out")
} else={
  :put "WARNING: container veth not found by address. If the container has no internet, allow forward from its veth to WAN in /ip/firewall/filter"
}
:if ([:len [/ip/firewall/filter/find comment="awg31_no_wan"]] = 0) do={
  :local first [/ip/firewall/filter/find]
  :if ([:len $first] > 0) do={
    /ip/firewall/filter/add chain=forward action=drop connection-state=new in-interface-list=WAN dst-address=$ip \
      comment="awg31_no_wan" place-before=[:pick $first 0]
  } else={
    /ip/firewall/filter/add chain=forward action=drop connection-state=new in-interface-list=WAN dst-address=$ip \
      comment="awg31_no_wan"
  }
  :put "+ new connections from WAN to the container are blocked (console and probe from LAN only)"
} else={
  /ip/firewall/filter/set [find comment="awg31_no_wan"] dst-address=$ip
}

# --- 5a. Режим сервера (выключено): входящий UDP 51821 с WAN -> контейнер:51820 ---
:if ([:len [/ip/firewall/nat/find comment="awg31_server_dstnat"]] = 0) do={
  /ip/firewall/nat/add chain=dstnat action=dst-nat in-interface-list=WAN protocol=udp dst-port=51821 \
    to-addresses=$ip to-ports=51820 disabled=yes comment="awg31_server_dstnat"
  :local first [/ip/firewall/filter/find comment="awg31_no_wan"]
  /ip/firewall/filter/add chain=forward action=accept connection-nat-state=dstnat in-interface-list=WAN dst-address=$ip \
    protocol=udp dst-port=51820 disabled=yes comment="awg31_server_accept" place-before=[:pick $first 0]
  :put "+ server mode rules (awg31_server_*, disabled)"
} else={
  /ip/firewall/nat/set [find comment="awg31_server_dstnat"] to-addresses=$ip
  /ip/firewall/filter/set [find comment="awg31_server_accept"] dst-address=$ip
}

# --- 6. IPv6: та же схема для таблицы awg31 (если IPv6 на роутере включён) ---
:if ([:len $ip6] > 0) do={
  :do {
    /ipv6/settings/set forward=yes
    :if ([:len [/ipv6/route/find comment="awg31_default"]] = 0) do={
      /ipv6/route/add dst-address=::/0 gateway=$ip6 routing-table="awg31" comment="awg31_default"
      /ipv6/route/add blackhole dst-address=::/0 distance=254 routing-table="awg31" comment="awg31_bh"
      :put ("+ IPv6 default -> " . $ip6 . " in table awg31 (+blackhole)")
    } else={
      /ipv6/route/set [find comment="awg31_default"] gateway=$ip6
    }
    :if ([:len [/ipv6/firewall/mangle/find comment="awg31_mark_conn"]] = 0) do={
      /ipv6/firewall/mangle/add chain=prerouting action=accept src-address=$ip6 comment="awg31_accept_container"
      /ipv6/firewall/mangle/add chain=prerouting action=mark-connection in-interface-list=LAN connection-mark=no-mark \
        connection-state=new dst-address-type=!local dst-address-list=$list new-connection-mark="awg31" passthrough=yes \
        comment="awg31_mark_conn"
      /ipv6/firewall/mangle/add chain=prerouting action=mark-routing in-interface-list=LAN connection-mark="awg31" \
        new-routing-mark="awg31" passthrough=no comment="awg31_route"
    } else={
      :if (!$all) do={ /ipv6/firewall/mangle/set [find comment="awg31_mark_conn"] dst-address-list=$list }
    }
    :if ([:len [/ipv6/firewall/nat/find comment="awg31_nat"]] = 0) do={
      /ipv6/firewall/nat/add chain=srcnat action=masquerade src-address=$ip6 out-interface-list=WAN comment="awg31_nat"
    }
    :if ([:len [/ipv6/firewall/filter/find comment="awg31_no_wan"]] = 0) do={
      /ipv6/firewall/filter/add chain=forward action=drop connection-state=new in-interface-list=WAN dst-address=$ip6 \
        comment="awg31_no_wan"
    }
    :put "+ IPv6: route, mangle, NAT and WAN protection"
  } on-error={ :put "IPv6 is disabled or unavailable on the router: IPv6 part skipped" }
}

# --- 7. Netwatch по пробе здоровья (выключен; включите, когда туннель заработает) ---
:if ([:len [/tool/netwatch/find comment="awg31_health"]] = 0) do={
  /tool/netwatch/add type=http-get host=$ip port=8081 interval=15s disabled=yes comment="awg31_health" \
    down-script="/ip/route/disable [find comment=\"awg31_default\"]" \
    up-script="/ip/route/enable [find comment=\"awg31_default\"]"
  :put "+ netwatch awg31_health (disabled=yes)"
} else={
  /tool/netwatch/set [find comment="awg31_health"] host=$ip
}

:put "==="
:put "DONE. Add addresses that must go through the VPN like this:"
:put ("  /ip/firewall/address-list/add list=" . $list . " address=example.com")
:put ("  /ipv6/firewall/address-list/add list=" . $list . " address=example.com")
:put "Netwatch: /tool/netwatch/enable [find comment=\"awg31_health\"]"
:put ("DNS through the container (for domain rules): /ip/dns/set servers=" . $ip)
:put "Server mode: /ip/firewall/nat/enable [find comment=\"awg31_server_dstnat\"]; /ip/firewall/filter/enable [find comment=\"awg31_server_accept\"]"
}

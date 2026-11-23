# ============================================================================
#  install-container.rsc — установка awg31 обычным контейнером RouterOS
#  (без /app). Подходит для arm64 и arm (ARMv7), RouterOS 7.20+ (envs/mounts
#  с параметром list=). Повторный запуск ничего не дублирует.
#
#  1. Отредактируйте значения в блоке «Параметры» ниже.
#  2. Загрузите файл на роутер (Winbox -> Files) и выполните:
#       /import install-container.rsc
#  3. Затем /import router-setup.rsc (маршрутизация по address-list).
#
#  ВНИМАНИЕ: скрипт не проверялся на живом роутере; синтаксис сверен с
#  документацией MikroTik. Если команда не подходит к вашей версии RouterOS,
#  сверьте имена параметров через «print» или «add ?».
# ============================================================================
{
# --- Параметры ---
# Диск для образа и данных: USB/SD, не внутренняя NAND.
:local disk "usb1"
# Для проверки без флешки: :local disk "" — тогда всё лежит на встроенной памяти роутера
# (каталог awg31/). Нужно около 60 МБ свободного места (/system/resource/print, free-hdd-space);
# для постоянной работы лучше USB/SD: на NAND много записей быстро её изнашивают.
# ВАЖНО: значения задаются ЗДЕСЬ, в файле. Команда `:local tarfile ...` в консоли в импортируемый
# скрипт не передаётся.
# Офлайн-установка: путь к .tar нужной архитектуры (awg31-0.7.0-linux-arm64.tar или
# awg31-0.7.0-linux-arm-v7.tar). Пусто — образ скачивается из реестра.
:local tarfile ""
# Образ в реестре (многоархитектурный: RouterOS берёт arm64 или arm сам).
# Если ваша версия не принимает реестр в имени, задайте
# /container/config/set registry-url=https://ghcr.io и укажите имя без ghcr.io/.
:local image "ghcr.io/stepm65/awg31-docker:0.7.0"
# Адреса veth: контейнер .2, роутер .1.
:local cip "192.168.253.2"
:local rip "192.168.253.1"
# IPv6 внутри veth (ULA). Тот же адрес контейнер выведет сам, если на veth нет IPv6.
:local cip6 "fd00:192:168:253::2"
:local rip6 "fd00:192:168:253::1"
:local dns "1.1.1.1"

# --- 0. Проверки ---
:local base "awg31"
:if ([:len $disk] > 0) do={ :set base ($disk . "/awg31") }
:if ([:len $disk] = 0) do={
  :local free [/system/resource/get free-hdd-space]
  :put ("встроенная память: свободно " . ($free / 1048576) . " МиБ")
  :if ($free < 62914560) do={ :error "на встроенной памяти меньше 60 МиБ: освободите место (удалите лишние файлы и пакеты) или используйте USB" }
}
:if (([:len $tarfile] > 0) && ([:len [/file/find name=$tarfile]] = 0)) do={ :error ("файл " . $tarfile . " не найден в /file: загрузите архив на роутер или поправьте путь") }
:if ([:len [/system/package/find name="container"]] = 0) do={ :error "нет пакета container (System -> Packages)" }
:if ([/system/device-mode/get container] = false) do={ :error "включите /system/device-mode/update container=yes и подтвердите кнопкой" }
:local arch [/system/resource/get architecture-name]
:put ("архитектура роутера: " . $arch)
:if (!($arch = "arm64" || $arch = "arm" || $arch = "x86_64" || $arch = "x86")) do={ :error ("архитектура " . $arch . " не поддерживается (нужны arm64, arm или x86)") }

# --- 1. veth и адрес роутера ---
:if ([:len [/interface/veth/find name="veth-awg31"]] = 0) do={
  /interface/veth/add name="veth-awg31" address=($cip . "/30," . $cip6 . "/64") gateway=$rip gateway6=$rip6
  :put "+ veth-awg31 (IPv4 + IPv6)"
}
:if ([:len [/ip/address/find interface="veth-awg31"]] = 0) do={
  /ip/address/add address=($rip . "/30") interface="veth-awg31"
  :put ("+ адрес роутера " . $rip . "/30")
}
:do {
  :if ([:len [/ipv6/address/find interface="veth-awg31"]] = 0) do={
    /ipv6/address/add address=($rip6 . "/64") interface="veth-awg31" advertise=no
    :put ("+ IPv6 роутера " . $rip6 . "/64")
  }
} on-error={ :put "IPv6 на роутере выключен: контейнер будет работать по IPv4" }

# --- 2. Переменные окружения (список awg31 пересоздаётся целиком) ---
/container/envs/remove [find list="awg31"]
/container/envs/add list="awg31" key="TZ" value="UTC"
/container/envs/add list="awg31" key="UI_TLS" value="auto"
/container/envs/add list="awg31" key="UI_TLS_SANS" value=$cip
/container/envs/add list="awg31" key="AWG_AUTO_ROUTE" value="on"
/container/envs/add list="awg31" key="AWG_KILLSWITCH" value="on"
/container/envs/add list="awg31" key="AWG_ENABLE_IPV6" value="on"
/container/envs/add list="awg31" key="HEALTH_LISTEN" value=":8081"
/container/envs/add list="awg31" key="HEALTH_MAX_HANDSHAKE_SEC" value="180"
/container/envs/add list="awg31" key="DNS_ENABLE" value="on"
/container/envs/add list="awg31" key="GOMEMLIMIT" value="64MiB"

# --- 3. Том для данных (профили, пароль, сертификат); пересоздаётся, чтобы подхватить новый путь ---
/container/mounts/remove [find list="awg31-data"]
/container/mounts/add list="awg31-data" src=($base . "/data") dst="/data"

# --- 4. Контейнер ---
:if ([:len [/container/find name="awg31"]] = 0) do={
  :if ([:len $tarfile] > 0) do={
    /container/add name="awg31" file=$tarfile interface="veth-awg31" root-dir=($base . "/root") \
      envlist="awg31" mountlists="awg31-data" hostname="awg31" dns=$dns start-on-boot=yes logging=yes
  } else={
    /container/add name="awg31" remote-image=$image interface="veth-awg31" root-dir=($base . "/root") \
      envlist="awg31" mountlists="awg31-data" hostname="awg31" dns=$dns start-on-boot=yes logging=yes
  }
  :put "+ контейнер awg31 (распаковка образа может занять несколько минут)"
}

# --- 5. Дождаться распаковки и запустить ---
:local tries 0
:while (([/container/get [find name="awg31"] status] = "extracting" || [/container/get [find name="awg31"] status] = "downloading") && $tries < 120) do={
  :delay 5s
  :set tries ($tries + 1)
}
:do { /container/start [find name="awg31"] } on-error={ :put "контейнер уже запущен или ещё не готов: проверьте /container/print" }

:put "==="
:put ("Веб-консоль: https://" . $cip . ":8443 (только из LAN)")
:put "Токен первичной настройки: /log/print where message~\"токен\""
:put "Дальше: /import router-setup.rsc"
}

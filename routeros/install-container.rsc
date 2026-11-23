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
# Без флешки есть два варианта проверки:
#  а) :local disk "" — всё на встроенной памяти (каталог awg31/). Нужно около 60 МиБ свободного
#     места (образ после распаковки ~35 МиБ плюс сам .tar); на NAND много записей её изнашивают;
#  б) в оперативной памяти: сначала `/disk/add type=tmpfs tmpfs-max-size=100M` (появится диск
#     tmp1), затем :local disk "tmp1" и :local datadir "awg31/data" (данные остаются на флеш,
#     они небольшие). После перезагрузки роутера содержимое tmp1 пропадает: выполните
#     `/container/remove [find name=awg31]` и запустите скрипт снова.
# Сообщения скрипта на английском: терминалы RouterOS (SSH, serial) часто не выводят кириллицу.
# ВАЖНО: значения задаются ЗДЕСЬ, в файле. Команда `:local tarfile ...` в консоли в импортируемый
# скрипт не передаётся.
# Офлайн-установка: путь к .tar нужной архитектуры (awg31-0.8.2-linux-arm64.tar или
# awg31-0.8.2-linux-arm-v7.tar). Пусто — образ скачивается из реестра.
:local tarfile ""
# Куда класть данные (профили, пароль, сертификат). Пусто — рядом с образом (<disk>/awg31/data).
:local datadir ""
# Образ в реестре (многоархитектурный: RouterOS берёт arm64 или arm сам).
# Если ваша версия не принимает реестр в имени, задайте
# /container/config/set registry-url=https://ghcr.io и укажите имя без ghcr.io/.
:local image "ghcr.io/stepm65/awg31-docker:0.8.2"
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
:local datapath ($base . "/data")
:if ([:len $datadir] > 0) do={ :set datapath $datadir }
# Диск ищем в /disk, а не в /file: одноимённый обычный каталог (например usb1, оставшийся от неудачного
# запуска) диском не является, и контейнер в нём создать нельзя.
:if ([:len $disk] > 0) do={
  :local diskOk false
  :do { :if ([:len [/disk/find where slot=$disk]] > 0) do={ :set diskOk true } } on-error={ :set diskOk ([:len [/file/find name=$disk]] > 0) }
  :if (!$diskOk) do={ :error ("disk " . $disk . " not found (see /disk/print). Edit :local disk at the top of this file: usb1 is only an example; a RAM disk is /disk/add type=tmpfs tmpfs-max-size=100M, then :local disk \"tmp1\"") }
}
:if ([:len $disk] = 0) do={
  :local free [/system/resource/get free-hdd-space]
  :put ("internal flash, free MiB: " . ($free / 1048576))
  :if ($free < 62914560) do={ :error "less than 60 MiB free on internal flash: free some space, use USB, or use a RAM disk (see the header of this file)" }
}
:if (([:len $tarfile] > 0) && ([:len [/file/find name=$tarfile]] = 0)) do={ :error ("file " . $tarfile . " not found in /file: upload the archive or fix the path") }
:if ([:len [/system/package/find name="container"]] = 0) do={ :error "container package is not installed (System -> Packages)" }
:if ([/system/device-mode/get container] = false) do={ :error "enable container mode: /system/device-mode/update container=yes, then confirm with the button or a power cycle" }
:local arch [/system/resource/get architecture-name]
:put ("router architecture: " . $arch)
:if (!($arch = "arm64" || $arch = "arm" || $arch = "x86_64" || $arch = "x86")) do={ :error ("architecture " . $arch . " is not supported (need arm64, arm or x86)") }

# --- 1. veth и адрес роутера ---
:if ([:len [/interface/veth/find name="veth-awg31"]] = 0) do={
  /interface/veth/add name="veth-awg31" address=($cip . "/30," . $cip6 . "/64") gateway=$rip gateway6=$rip6
  :put "+ veth-awg31 (IPv4 + IPv6)"
}
:if ([:len [/ip/address/find interface="veth-awg31"]] = 0) do={
  /ip/address/add address=($rip . "/30") interface="veth-awg31"
  :put ("+ router address " . $rip . "/30")
}
:do {
  :if ([:len [/ipv6/address/find interface="veth-awg31"]] = 0) do={
    /ipv6/address/add address=($rip6 . "/64") interface="veth-awg31" advertise=no
    :put ("+ router IPv6 " . $rip6 . "/64")
  }
} on-error={ :put "IPv6 is disabled on the router: the container will use IPv4 only" }

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
/container/mounts/add list="awg31-data" src=$datapath dst="/data"

# --- 4. Контейнер ---
:if ([:len [/container/find name="awg31"]] = 0) do={
  :if ([:len $tarfile] > 0) do={
    /container/add name="awg31" file=$tarfile interface="veth-awg31" root-dir=($base . "/root") \
      envlist="awg31" mountlists="awg31-data" hostname="awg31" dns=$dns start-on-boot=yes logging=yes
  } else={
    /container/add name="awg31" remote-image=$image interface="veth-awg31" root-dir=($base . "/root") \
      envlist="awg31" mountlists="awg31-data" hostname="awg31" dns=$dns start-on-boot=yes logging=yes
  }
  :put "+ container awg31 added (unpacking the image can take a few minutes)"
}

# --- 5. Дождаться распаковки и запустить ---
# Статус читаем через find, а не /container/get: имя status принимается не везде.
:local tries 0
:local busy true
:while ($busy && $tries < 120) do={
  :do {
    :set busy (([:len [/container/find where name="awg31" status="extracting"]] > 0) || ([:len [/container/find where name="awg31" status="downloading"]] > 0))
  } on-error={
    :set busy false
    :put "cannot read the container status: check /container/print and start it by hand"
  }
  :if ($busy) do={
    :delay 5s
    :set tries ($tries + 1)
  }
}
:do { /container/start [find name="awg31"] } on-error={ :put "the container is already running or not ready yet: check /container/print, then /container/start awg31" }

:put "==="
:put ("Web console: https://" . $cip . ":8443 (from LAN only)")
:put "One-time setup token (long hex string) is in the container log: /log/print where topics~\"container\""
:put "Next: /import router-setup.rsc"
}

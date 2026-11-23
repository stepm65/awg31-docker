"use strict";
// Весь интерфейс строится через DOM API и textContent: данные сервера никогда не
// попадают в разметку как HTML, поэтому XSS через имена профилей, ошибки или
// ссылки невозможен. Единственный разбор разметки — иконки из констант ниже.
// Вид и тексты повторяют AWG Console (amneziawg31-installer): RU/EN, тёмная тема.

/* ---------- иконки (Lucide-подобные, viewBox 24, как в панели) ---------- */
const ICONS = {
  route: '<path d="M6 19a2 2 0 1 0 0-4 2 2 0 0 0 0 4zM18 9a2 2 0 1 0 0-4 2 2 0 0 0 0 4z"/><path d="M6 15V9a4 4 0 0 1 4-4h6M18 9v6a4 4 0 0 1-4 4H8"/>',
  arrowUp: '<path d="M12 19V5m-6 6 6-6 6 6"/>',
  arrowDown: '<path d="M12 5v14m-6-6 6 6 6-6"/>',
  star: '<path d="m12 3 2.7 5.6 6.1.9-4.4 4.3 1 6.1L12 17l-5.4 2.9 1-6.1L3.2 9.5l6.1-.9z"/>',
  home: '<path d="m3 10 9-7 9 7v10a1 1 0 0 1-1 1h-5v-7H9v7H4a1 1 0 0 1-1-1z"/>',
  server: '<rect x="3" y="3" width="18" height="8" rx="2"/><rect x="3" y="13" width="18" height="8" rx="2"/><path d="M7 7h.01M7 17h.01"/>',
  settings: '<path d="M4 7h9m4 0h3M4 17h3m4 0h9"/><circle cx="15" cy="7" r="2"/><circle cx="9" cy="17" r="2"/>',
  shield: '<path d="M12 22s8-4 8-11V5l-8-3-8 3v6c0 7 8 11 8 11z"/><path d="m9 12 2 2 4-4"/>',
  lock: '<rect x="4" y="10" width="16" height="12" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3"/>',
  plus: '<path d="M12 5v14M5 12h14"/>',
  play: '<path d="m8 5 11 7-11 7z"/>',
  pause: '<path d="M9 5v14M15 5v14"/>',
  trash: '<path d="M3 6h18m-2 0-1 14H6L5 6m4 0V4h6v2M10 10v6M14 10v6"/>',
  restart: '<path d="M20 7v5h-5"/><path d="M20 12a8 8 0 1 0-2.3 5.7"/>',
  logout: '<path d="M10 17l5-5-5-5M15 12H3"/><path d="M12 3h7a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-7"/>',
  activity: '<path d="M3 12h4l3-7 4 14 3-7h4"/>',
  link: '<path d="M10 13a5 5 0 0 0 7.5.5l3-3a5 5 0 0 0-7-7l-1.7 1.7M14 11a5 5 0 0 0-7.5-.5l-3 3a5 5 0 0 0 7 7l1.7-1.7"/>',
  clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
  info: '<circle cx="12" cy="12" r="10"/><path d="M12 11v6M12 7h.01"/>',
  check: '<path d="m5 12 4 4L19 6"/>',
  arrow: '<path d="M5 12h14m-6-6 6 6-6 6"/>',
  chevron: '<path d="m9 18 6-6-6-6"/>',
  tag: '<path d="M3 12V4a1 1 0 0 1 1-1h8l9 9-9 9z"/><circle cx="7.5" cy="7.5" r="1.5"/>',
  alert: '<path d="M12 3 2 21h20L12 3zM12 9v5M12 18h.01"/>',
  key: '<circle cx="8" cy="15" r="4"/><path d="m11 12 10-10m-5 5 3 3"/>',
  file: '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6M8 13h8M8 17h8"/>',
  x: '<path d="M18 6L6 18M6 6l12 12"/>',
  layers: '<path d="m12 2 9 5-9 5-9-5 9-5Zm-9 10 9 5 9-5M3 17l9 5 9-5"/>',
  merge: '<circle cx="5" cy="6" r="2.5"/><circle cx="5" cy="18" r="2.5"/><circle cx="19" cy="12" r="2.5"/><path d="M7.5 6H11a4 4 0 0 1 4 4v1m-7.5 7H11a4 4 0 0 0 4-4v-1"/>',
};
const svgParser = new DOMParser();
function icon(name, size = 18) {
  const inner = ICONS[name] || ICONS.info;
  const doc = svgParser.parseFromString(
    '<svg xmlns="http://www.w3.org/2000/svg" class="icon" width="' + size + '" height="' + size +
    '" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">' +
    inner + "</svg>", "image/svg+xml");
  return document.importNode(doc.documentElement, true);
}

/* ---------- тексты RU/EN ---------- */
const I18N = {
  ru: {
    console: "AWG CONSOLE", refresh: "Обновить", logout: "Выход", save: "Сохранить", delete: "Удалить",
    up: "Поднять", down: "Опустить", running: "Запущен", stopped: "Остановлен", never: "не было",
    ago_s: "{n} с назад", ago_m: "{n} мин назад", ago_h: "{n} ч назад", ago_d: "{n} дн назад",
    loading: "Загрузка…", yes: "да", no: "нет", back: "К списку", confirmDelete: "Подтвердить удаление",
    version: "Версия", language: "Язык", close: "Закрыть",
    navInfra: "ИНФРАСТРУКТУРА", navProfiles: "ПРОФИЛИ", navSystem: "СИСТЕМА",
    navOverview: "Обзор", navAll: "Все профили", navMaint: "Обслуживание",
    // вход и настройка
    loginTitle: "Вход администратора", loginLead: "Управляйте профилями AmneziaWG 3.1, подпиской панели и автозапуском туннелей в одной консоли.",
    f1: "Профили .conf и автозапуск после перезагрузки", f2: "Подписка с вашей панели по HTTPS", f3: "Строгая проверка конфигов и TLS",
    accessEyebrow: "AMNEZIAWG 3.1 / ACCESS", accessTitle: "Доступ к консоли", accessLead: "Введите логин и пароль администратора.",
    user: "Логин", password: "Пароль", signIn: "Войти",
    setupTitle: "Первичная настройка", setupLead: "Токен настройки одноразовый и напечатан в журнале контейнера: docker logs awg31.",
    setupEyebrow: "AMNEZIAWG 3.1 / SETUP", setupToken: "Токен настройки", passwordNew: "Новый пароль (не короче 10 символов)",
    passwordRepeat: "Повторите пароль", createAdmin: "Создать администратора", passwordsDiffer: "Пароли не совпадают.",
    // обзор
    overviewTitle: "Обзор", statTunnels: "Туннели", statTunnelsNote: "поднято / всего", statRx: "Получено", statTx: "Отправлено",
    statHandshake: "Рукопожатие", statHandshakeNote: "последнее по всем пирам", statSumNote: "сумма по всем туннелям", profilesTitle: "Профили AWG",
    manageAll: "Все профили", manage: "Управление", rx: "Получение", tx: "Отправка", handshake: "Рукопожатие",
    noProfiles: "Профилей пока нет", noProfilesHint: "Добавьте .conf или подключите подписку вашей панели.", addProfile: "Добавить профиль",
    tunnelError: "Не удалось получить состояние туннелей: {err}",
    // профили
    profilesHeading: "Профили", profilesSub: "Конфигурации AmneziaWG 3.1 и их подписки.",
    colName: "Имя", colAddress: "Адрес", colState: "Состояние", colTraffic: "Трафик", colSub: "Подписка", colActions: "Действия",
    subOn: "каждые {n}", subOff: "вручную", subNone: "нет", autostart: "Автозапуск", profileMissing: "Профиль не найден",
    tabOverview: "Обзор", tabConfig: "Конфигурация", tabSub: "Подписка",
    profileEyebrow: "ПРОФИЛИ / {name}", profileTitle: "Профиль {name}", noConfigYet: "Конфигурации ещё нет: она придёт по подписке или вставьте .conf.",
    peerEndpoint: "Endpoint", peerHandshake: "Рукопожатие", peerTraffic: "Трафик", peerAllowed: "AllowedIPs", peerKey: "Ключ пира",
    cfgAddresses: "Адреса", cfgEndpoints: "Endpoints", cfgObf: "Обфускация", cfgPsk: "PresharedKey", cfgPeers: "Пиров",
    on: "включена", off: "нет", present: "есть", absent: "нет", autostartInfo: "После перезапуска контейнера туннель {state}.",
    autostartYes: "поднимется сам", autostartNo: "останется опущенным",
    // конфиг
    cfgTitle: "Заменить конфигурацию", cfgHint: "Приватные ключи нигде не показываются, поэтому конфиг заменяется целиком: вставьте новый .conf.",
    cfgHooks: "Строки PreUp/PostUp/PreDown/PostDown запрещены: NAT и подгонку MSS контейнер настраивает сам.",
    cfgPlaceholder: "[Interface]\nPrivateKey = …\nAddress = …\n\n[Peer]\nPublicKey = …\nEndpoint = …\nAllowedIPs = …", cfgSaved: "Конфигурация сохранена.",
    cfgSavedRestart: "Конфигурация сохранена. Перезапустите туннель, чтобы применить её.", cfgEmpty: "Вставьте текст конфигурации.",
    // подписка
    subTitle: "Подписка панели", subHint: "Контейнер сам забирает актуальный .conf по ссылке из вашей панели AWG Console (Подписка для роутера).",
    subUrl: "Ссылка подписки", subUrlKeep: "Оставьте пустым, чтобы сохранить текущую ссылку: она скрыта, потому что токен панели лежит прямо в ней.",
    subUrlNew: "https://ваш-сервер/sub/awg0/…", subToken: "Токен (если сервер требует отдельный)", subTokenSet: "•••••• сохранён", subClearToken: "Удалить сохранённый токен",
    subPin: "Отпечаток сервера (пин)", subPinHint: "Для самоподписанного сертификата: sha256//… из панели или SHA-256 сертификата (64 hex).",
    subInterval: "Автообновление", subIntervalOff: "Выключено (только вручную)", subInsecure: "Не проверять сертификат TLS (небезопасно)",
    subInsecureHint: "Только если нет пина. Подмена сервера станет возможной.", subSave: "Сохранить подписку", subSync: "Обновить сейчас", subRemove: "Удалить подписку",
    subSaved: "Подписка сохранена.", subSynced: "Профиль обновлён из подписки.", subSyncedSame: "Подписка проверена: изменений нет.", subRemoved: "Подписка удалена.",
    subLast: "Последняя проверка", subLastError: "Ошибка", subNotice: "Замечание", subPinState: "Пин", subTlsInsecure: "проверка отключена", subTlsOk: "проверка включена",
    minutes: "{n} мин", hours: "{n} ч", days: "{n} сут",
    // новый профиль
    newTitle: "Новый профиль", newSub: "Вставьте готовый .conf или подключите подписку панели.", profileName: "Имя профиля",
    nameHint: "Латиница, цифры, _ и -, до 15 символов: это и имя интерфейса.", modeConf: "Вставить .conf", modeSub: "Подписка панели", create: "Создать профиль",
    created: "Профиль создан.", needName: "Укажите имя профиля.",
    // действия
    upDone: "Туннель {name} поднят.", downDone: "Туннель {name} опущен.", deleted: "Профиль {name} удалён.",
    // обслуживание
    maintTitle: "Обслуживание", maintSub: "Пароль, версия и состояние защиты.", pwTitle: "Пароль администратора",
    pwCurrent: "Текущий пароль", pwNew: "Новый пароль", pwRepeat: "Повторите новый пароль", pwChange: "Сменить пароль",
    pwChanged: "Пароль изменён, остальные сессии завершены.", navFailover: "Резервирование", primary: "Основной", primaryHint: "Туннель, в который сейчас идёт трафик LAN", makePrimary: "Сделать основным",
    primaryDone: "Трафик LAN переведён в туннель {name}.",
    foTitle: "Резервирование", foSub: "Несколько туннелей одновременно и автоматическое переключение при сбое.",
    foCheckNow: "Проверить сейчас", foChecked: "Проверка выполнена.", foSaved: "Настройки резервирования сохранены.",
    foNoGateway: "Резервирование работает только в режиме шлюза: включите AWG_AUTO_ROUTE и проверьте LAN-интерфейс на странице «Обслуживание».",
    foMode: "Режим", fo_mode_off: "Вручную", fo_mode_standby: "Горячий резерв", fo_mode_switch: "Переключение",
    foModeNote_off: "Туннели поднимаются и выбираются основным вручную.",
    foModeNote_standby: "Все туннели из списка подняты; при сбое трафик мгновенно уходит в следующий.",
    foModeNote_switch: "Поднят только основной; при сбое он опускается и поднимается следующий.",
    foTrigger: "Признак сбоя", fo_trigger_hybrid: "Рукопожатие и проба", fo_trigger_handshake: "Только рукопожатие", fo_trigger_probe: "Только проба",
    foProbe: "Цель пробы (IP:порт)", foProbeHint: "TCP-соединение через каждый туннель. Только IP: имя пришлось бы разрешать через неисправный туннель.",
    foInterval: "Период проверки, с", foThreshold: "Неудач подряд до переключения", foCooldown: "Пауза после сбоя, с",
    foHsMax: "Макс. возраст рукопожатия, с", foGrace: "Ожидание первого рукопожатия, с",
    foFailback: "Возвращаться к более приоритетному туннелю", foFailbackHint: "Когда он снова исправен и пауза закончилась. Только для горячего резерва.",
    foSave: "Сохранить", foSettings: "Настройки", foPriority: "Порядок и состояние",
    foPrioritySub: "Первый — самый приоритетный. Исключённые туннели монитор не трогает.",
    foColRank: "№", foColCheck: "Проверка", foUp: "Выше", foDown: "Ниже", foExclude: "Исключить", foInclude: "Добавить",
    foSwitches: "Переключений", foNoSwitches: "с момента запуска не было", foLastCheck: "Последняя проверка", foEvery: "каждые {n} с",
    foCooldownLeft: "пауза ещё {n} с", foProbeOk: "проба {ms} мс", foProbeFail: "проба не прошла",
    foState_healthy: "Исправен", foState_starting: "Запускается", foState_failing: "Сбои", foState_unhealthy: "Неисправен",
    foState_cooldown: "Пауза", foState_down: "Не поднят", foState_excluded: "Не участвует",
    navPolicy: "Маршрутизация", polTitle: "Маршрутизация и DNS", polSub: "Правила по доменам, geosite, geoip и сетям: что идёт через VPN, что мимо, что блокируется.",
    polNoGateway: "Правила работают только в режиме шлюза: включите AWG_AUTO_ROUTE и проверьте LAN-интерфейс на странице «Обслуживание».",
    polState: "Маршрутизация", polDefault: "Остальной трафик", polRules: "Правил", polRoutes: "{n} маршрутов в таблице 210",
    polLearned: "Выучено адресов", polLearnedNote: "из ответов DNS для доменных правил", polQueries: "DNS-запросов",
    polDnsNote: "VPN {p} · мимо {d} · блок {x}", polDnsOff: "DNS-форвардер выключен",
    polAct_proxy: "через VPN", polAct_direct: "мимо VPN", polAct_drop: "блокировать",
    polRulesTitle: "Правила", polEnable: "Включить маршрутизацию по правилам", polEnableHint: "Без неё весь трафик LAN идёт в основной туннель.",
    polDefaultHint: "Для всего, что не совпало ни с одним правилом.", polGeoEvery: "Обновлять базы geo, часов", polGeoEveryHint: "0 — только вручную.",
    polRulesLabel: "Правила (по одному на строку)",
    polRulesHint: "ДЕЙСТВИЕ УСЛОВИЕ. Действия: PROXY, PROXY@туннель или PROXY@группа, DIRECT, DROP. Условия: domain:, full:, keyword:, geosite:, geoip:, сеть или IP. Для доменов — первое совпавшее правило, для адресов — самый точный префикс. # — комментарий.",
    polProxyDNS: "DNS через VPN", polDirectDNS: "DNS мимо VPN", polDnsHint: "IP, IP:порт или https://…/dns-query (DoH), через запятую.",
    polSaved: "Правила сохранены.", polSavedWarn: "Правила сохранены, но:",
    polTestTitle: "Проверить", polTestLabel: "Домен или IP-адрес", polTestBtn: "Проверить", polBy: "правило",
    polGeoTitle: "Базы geosite и geoip", polGeoUpdate: "Обновить базы", polGeoUpdated: "Базы geo обновлены.",
    polGeoFile: "Файл", polGeoSize: "Размер", polGeoDate: "Обновлён", polGeoLoaded: "Загружено категорий",
    polGeoHave: "Есть", polGeoNone: "Нет", polGeoMissing: "Нет в базе",
    navRouter: "RouterOS", rosTitle: "Интеграция с RouterOS", rosSub: "Контейнер сам сообщает роутеру, какой трафик отправлять в VPN: списком адресов или маршрутами BGP, и перенаправляет DNS нужных доменов на себя.",
    rosNoGateway: "Интеграция работает только в режиме шлюза: включите AWG_AUTO_ROUTE и проверьте LAN-интерфейс.",
    rosSyncNow: "Синхронизировать", rosSynced: "Синхронизация выполнена.", rosSaved: "Настройки RouterOS сохранены.", rosTest: "Проверить подключение", rosTestOk: "Подключение работает: {who}.",
    rosModeLabel: "Режим", rosMode_off: "Выключено", rosMode_address_list: "Address-List", rosMode_bgp: "BGP",
    rosModeHint_off: "Роутер настраивается только скриптами router-setup.rsc.",
    rosModeHint_address_list: "Сети PROXY и адреса из DNS попадают в address-list; mangle роутера ведёт их в контейнер.",
    rosModeHint_bgp: "Сети PROXY и адреса из DNS анонсируются роутеру по BGP с next-hop = контейнер. Только IPv4.",
    rosNoRouter: "роутер ещё не опрошен", rosStatic: "Сетей из правил", rosStaticNote: "PROXY: сети и geoip", rosDropped: "{n} не вошли в предел",
    rosDynamic: "Адресов из DNS", rosDynamicNote: "с ограниченным сроком", rosDomains: "Доменов в DNS FWD", rosFwdTo: "пересылаются на {ip}", rosDnsOff: "перенаправление выключено",
    rosAnnounced: "Анонсировано сетей", rosBgpErr: "Последняя ошибка", rosBgp_idle: "Не запущен", rosBgp_connecting: "Подключение", rosBgp_established: "Установлен", rosBgp_error: "Ошибка",
    rosSettings: "Подключение и синхронизация", rosURL: "Адрес роутера", rosURLHint: "https://IP; http — только для частного IP на veth.",
    rosUser: "Пользователь", rosPass: "Пароль", rosPassKept: "сохранён — оставьте пустым", rosPassHint: "Хранится в state.json (0600) и никогда не отдаётся обратно.",
    rosPin: "Пин сертификата", rosPinHint: "sha256//… (SPKI) или SHA-256 сертификата: для самоподписанного сертификата роутера.",
    rosList: "Имя address-list", rosListHint: "Любой список, в том числе уже существующий (по умолчанию awg31). Тот же — в :global AWG31LIST для router-setup.rsc. Чужие записи не трогаются.",
    rosFwd: "IP контейнера", rosFwdHint: "Пусто — определить по LAN-интерфейсу.", rosLimit: "Предел сетей", rosDynMin: "Срок адресов из DNS, мин", rosDomLimit: "Предел доменов",
    rosLocalAS: "AS контейнера", rosRemoteAS: "AS роутера", rosPeer: "IP роутера для BGP", rosPeerHint: "Пусто — шлюз LAN.",
    rosSyncDNS: "Перенаправлять DNS доменных правил на контейнер", rosSyncDNSHint: "Записи /ip/dns/static типа FWD для domain:, full: и geosite:. Тогда не нужно отдавать контейнеру весь DNS.",
    rosInsecure: "Не проверять сертификат роутера", rosInsecureHint: "Только если нет пина: иначе пароль можно перехватить.",
    rosOnRouter: "Что настроить на роутере", rosOnRouterSub: "Выполните один раз в терминале RouterOS (команды подставлены из настроек).",
    rosScrUser: "пользователь с минимальными правами для контейнера",
    rosScrPass: "пароль",
    rosScrRest: "REST API только для контейнера (https; сертификат самоподписанный — укажите пин)",
    rosScrBgp: "BGP: роутер принимает от контейнера только сети, без default",
    navGroups: "Группы туннелей", grpTitle: "Группы туннелей", grpSub: "Несколько VPS как один выход: соединения распределяются между участниками, упавший выводится из группы сам.",
    grpAdd: "Добавить группу", grpNone: "Групп пока нет", grpNoneHint: "Создайте группу, добавьте в неё туннели и используйте её основным выходом или в правилах: PROXY@имя_группы.",
    grpName: "Имя группы", grpMode: "Режим", grp_mode_balance: "Балансировка", grp_mode_ordered: "По порядку",
    grpModeNote_balance: "Новые соединения распределяются между исправными участниками по весам; один поток остаётся в одном туннеле.",
    grpModeNote_ordered: "Трафик идёт через первого исправного участника; если он упал, то через следующего.",
    grpMembers: "Участники", grpColMember: "Туннель", grpColWeight: "Вес", grpAddMember: "Туннель для добавления", grpAddBtn: "Добавить",
    grpNoMembers: "В группе нет туннелей.", grpNoFree: "Свободных профилей клиента нет.", grpInRoute: "В маршруте", grpNotSaved: "Не сохранён",
    grpRemove: "Убрать", grpRoute: "Маршрут", grpTable: "таблица {n}", grpActiveOf: "в маршруте {a} из {n}",
    grpEmpty: "Нет исправных участников: через группу сейчас ничего не передаётся (при включённом kill switch трафик блокируется).",
    grpSaved: "Группы сохранены.", grpSavedWarn: "Сохранено, но не всё поднялось",
    grpNoGateway: "Группы работают только в режиме шлюза: включите AWG_AUTO_ROUTE и проверьте LAN-интерфейс на странице «Обслуживание».",
    grpHashWarn: "fib_multipath_hash_policy сейчас «{v}»: ядро делит потоки по адресам, а не по портам, и весь трафик одного клиента пойдёт через один туннель. Нужно net.ipv4.fib_multipath_hash_policy=1 (sysctls контейнера).",
    grpStatCount: "Групп", grpStatCountNote: "настроено", grpStatHash: "Хэш потоков", grpHashPorts: "по портам", grpHashAddr: "по адресам", grpHashUnknown: "неизвестно",
    grpUsage: "Как использовать", grpMakePrimary: "Сделать основной", grpDeleteGroup: "Удалить группу",
    grpUsageText: "В правилах маршрутизации пишите PROXY@имя_группы (например, PROXY@vps domain:example.com). Кнопка «Сделать основной» направляет через группу весь трафик LAN. IPv6 идёт через одного участника группы.",
    foNoteGroup: "Основной — группа «{name}»: переключение по списку приоритета не выполняется, участников группа проверяет и выводит из маршрута сама.",
    netTitle: "Сеть", netGateway: "Режим шлюза", netLan: "LAN-интерфейс", netKill: "Kill switch", netIPv6: "IPv6 в туннеле", netTable: "Таблица маршрутов",
    netKillErr: "не включён", netOn: "включён", netOff: "выключен", netHint: "Задаётся переменными AWG_AUTO_ROUTE, AWG_LAN_IFACE, AWG_KILLSWITCH и AWG_ENABLE_IPV6. При kill switch трафик LAN без туннеля отбрасывается.",
    aboutTitle: "О контейнере", aboutUser: "Администратор",
    aboutTls: "Соединение", tlsOk: "защищено TLS", tlsNo: "без TLS: не публикуйте порт в сети", aboutServerErr: "Сообщения об ошибках приходят с сервера на русском.",
  },
  en: {
    console: "AWG CONSOLE", refresh: "Refresh", logout: "Sign out", save: "Save", delete: "Delete",
    up: "Bring up", down: "Bring down", running: "Running", stopped: "Stopped", never: "never",
    ago_s: "{n}s ago", ago_m: "{n}m ago", ago_h: "{n}h ago", ago_d: "{n}d ago",
    loading: "Loading…", yes: "yes", no: "no", back: "Back to list", confirmDelete: "Confirm deletion",
    version: "Version", language: "Language", close: "Close",
    navInfra: "INFRASTRUCTURE", navProfiles: "PROFILES", navSystem: "SYSTEM",
    navOverview: "Overview", navAll: "All profiles", navMaint: "Maintenance",
    loginTitle: "Administrator login", loginLead: "Manage AmneziaWG 3.1 profiles, the panel subscription and tunnel autostart from one console.",
    f1: "Profiles from .conf and autostart after a reboot", f2: "Subscription from your panel over HTTPS", f3: "Strict config and TLS validation",
    accessEyebrow: "AMNEZIAWG 3.1 / ACCESS", accessTitle: "Console access", accessLead: "Enter the administrator login and password.",
    user: "Login", password: "Password", signIn: "Sign in",
    setupTitle: "First-time setup", setupLead: "The setup token is single-use and printed in the container log: docker logs awg31.",
    setupEyebrow: "AMNEZIAWG 3.1 / SETUP", setupToken: "Setup token", passwordNew: "New password (at least 10 characters)",
    passwordRepeat: "Repeat the password", createAdmin: "Create administrator", passwordsDiffer: "The passwords do not match.",
    overviewTitle: "Overview", statTunnels: "Tunnels", statTunnelsNote: "up / total", statRx: "Received", statTx: "Sent",
    statHandshake: "Handshake", statHandshakeNote: "latest across all peers", statSumNote: "total across all tunnels", profilesTitle: "AWG profiles",
    manageAll: "All profiles", manage: "Manage", rx: "Received", tx: "Sent", handshake: "Handshake",
    noProfiles: "No profiles yet", noProfilesHint: "Add a .conf or connect your panel's subscription.", addProfile: "Add profile",
    tunnelError: "Could not read the tunnel state: {err}",
    profilesHeading: "Profiles", profilesSub: "AmneziaWG 3.1 configurations and their subscriptions.",
    colName: "Name", colAddress: "Address", colState: "State", colTraffic: "Traffic", colSub: "Subscription", colActions: "Actions",
    subOn: "every {n}", subOff: "manual", subNone: "none", autostart: "Autostart", profileMissing: "Profile not found",
    tabOverview: "Overview", tabConfig: "Configuration", tabSub: "Subscription",
    profileEyebrow: "PROFILES / {name}", profileTitle: "Profile {name}", noConfigYet: "No configuration yet: it arrives via the subscription, or paste a .conf.",
    peerEndpoint: "Endpoint", peerHandshake: "Handshake", peerTraffic: "Traffic", peerAllowed: "AllowedIPs", peerKey: "Peer key",
    cfgAddresses: "Addresses", cfgEndpoints: "Endpoints", cfgObf: "Obfuscation", cfgPsk: "PresharedKey", cfgPeers: "Peers",
    on: "on", off: "off", present: "set", absent: "none", autostartInfo: "After the container restarts the tunnel {state}.",
    autostartYes: "comes up by itself", autostartNo: "stays down",
    cfgTitle: "Replace the configuration", cfgHint: "Private keys are never shown, so the config is replaced as a whole: paste a new .conf.",
    cfgHooks: "PreUp/PostUp/PreDown/PostDown lines are not allowed: the container sets up NAT and MSS clamping itself.",
    cfgPlaceholder: "[Interface]\nPrivateKey = …\nAddress = …\n\n[Peer]\nPublicKey = …\nEndpoint = …\nAllowedIPs = …", cfgSaved: "Configuration saved.",
    cfgSavedRestart: "Configuration saved. Restart the tunnel to apply it.", cfgEmpty: "Paste the configuration text.",
    subTitle: "Panel subscription", subHint: "The container fetches the current .conf from your AWG Console panel link (Router subscription).",
    subUrl: "Subscription link", subUrlKeep: "Leave empty to keep the current link: it is hidden because the panel token is part of the link itself.",
    subUrlNew: "https://your-server/sub/awg0/…", subToken: "Token (only if the server needs a separate one)", subTokenSet: "•••••• saved", subClearToken: "Remove the saved token",
    subPin: "Server fingerprint (pin)", subPinHint: "For a self-signed certificate: sha256//… from the panel, or the certificate SHA-256 (64 hex).",
    subInterval: "Auto-refresh", subIntervalOff: "Off (manual only)", subInsecure: "Do not verify the TLS certificate (insecure)",
    subInsecureHint: "Only when there is no pin. The server can then be impersonated.", subSave: "Save subscription", subSync: "Refresh now", subRemove: "Remove subscription",
    subSaved: "Subscription saved.", subSynced: "Profile updated from the subscription.", subSyncedSame: "Subscription checked: nothing changed.", subRemoved: "Subscription removed.",
    subLast: "Last check", subLastError: "Error", subNotice: "Notice", subPinState: "Pin", subTlsInsecure: "verification off", subTlsOk: "verification on",
    minutes: "{n} min", hours: "{n} h", days: "{n} d",
    newTitle: "New profile", newSub: "Paste a ready .conf or connect the panel subscription.", profileName: "Profile name",
    nameHint: "Latin letters, digits, _ and -, up to 15 characters: it is also the interface name.", modeConf: "Paste .conf", modeSub: "Panel subscription", create: "Create profile",
    created: "Profile created.", needName: "Enter a profile name.",
    upDone: "Tunnel {name} is up.", downDone: "Tunnel {name} is down.", deleted: "Profile {name} deleted.",
    maintTitle: "Maintenance", maintSub: "Password, version and protection status.", pwTitle: "Administrator password",
    pwCurrent: "Current password", pwNew: "New password", pwRepeat: "Repeat the new password", pwChange: "Change password",
    pwChanged: "Password changed, other sessions were signed out.", navFailover: "Failover", primary: "Primary", primaryHint: "The tunnel LAN traffic currently goes through", makePrimary: "Make primary",
    primaryDone: "LAN traffic now goes through {name}.",
    foTitle: "Failover", foSub: "Several tunnels at once and automatic switching on failure.",
    foCheckNow: "Check now", foChecked: "Check done.", foSaved: "Failover settings saved.",
    foNoGateway: "Failover works only in gateway mode: turn on AWG_AUTO_ROUTE and check the LAN interface on the Maintenance page.",
    foMode: "Mode", fo_mode_off: "Manual", fo_mode_standby: "Hot standby", fo_mode_switch: "Switch over",
    foModeNote_off: "Tunnels are brought up and made primary by hand.",
    foModeNote_standby: "All listed tunnels are up; on failure traffic moves to the next one instantly.",
    foModeNote_switch: "Only the primary is up; on failure it goes down and the next one comes up.",
    foTrigger: "Failure signal", fo_trigger_hybrid: "Handshake and probe", fo_trigger_handshake: "Handshake only", fo_trigger_probe: "Probe only",
    foProbe: "Probe target (IP:port)", foProbeHint: "A TCP connection through each tunnel. IP only: a name would have to be resolved through a broken tunnel.",
    foInterval: "Check interval, s", foThreshold: "Failures in a row before switching", foCooldown: "Pause after a failure, s",
    foHsMax: "Max handshake age, s", foGrace: "Wait for the first handshake, s",
    foFailback: "Return to a higher-priority tunnel", foFailbackHint: "Once it is healthy again and its pause is over. Hot standby only.",
    foSave: "Save", foSettings: "Settings", foPriority: "Order and health",
    foPrioritySub: "The first one has the highest priority. Excluded tunnels are left alone.",
    foColRank: "#", foColCheck: "Check", foUp: "Up", foDown: "Down", foExclude: "Exclude", foInclude: "Add",
    foSwitches: "Switches", foNoSwitches: "none since start", foLastCheck: "Last check", foEvery: "every {n} s",
    foCooldownLeft: "paused for {n} s", foProbeOk: "probe {ms} ms", foProbeFail: "probe failed",
    foState_healthy: "Healthy", foState_starting: "Starting", foState_failing: "Failing", foState_unhealthy: "Unhealthy",
    foState_cooldown: "Paused", foState_down: "Not up", foState_excluded: "Excluded",
    navPolicy: "Routing", polTitle: "Routing and DNS", polSub: "Rules by domain, geosite, geoip and network: what goes through the VPN, what bypasses it, what is blocked.",
    polNoGateway: "Rules work only in gateway mode: turn on AWG_AUTO_ROUTE and check the LAN interface on the Maintenance page.",
    polState: "Routing", polDefault: "Everything else", polRules: "Rules", polRoutes: "{n} routes in table 210",
    polLearned: "Learned addresses", polLearnedNote: "from DNS answers for domain rules", polQueries: "DNS queries",
    polDnsNote: "VPN {p} · direct {d} · blocked {x}", polDnsOff: "DNS forwarder is off",
    polAct_proxy: "through VPN", polAct_direct: "bypass VPN", polAct_drop: "block",
    polRulesTitle: "Rules", polEnable: "Turn on rule-based routing", polEnableHint: "Without it all LAN traffic goes to the primary tunnel.",
    polDefaultHint: "For everything that matches no rule.", polGeoEvery: "Update geo databases, hours", polGeoEveryHint: "0 — manual only.",
    polRulesLabel: "Rules (one per line)",
    polRulesHint: "ACTION CONDITION. Actions: PROXY, PROXY@tunnel or PROXY@group, DIRECT, DROP. Conditions: domain:, full:, keyword:, geosite:, geoip:, a network or an IP. Domains use the first matching rule, addresses the most specific prefix. # starts a comment.",
    polProxyDNS: "DNS through VPN", polDirectDNS: "DNS bypassing VPN", polDnsHint: "IP, IP:port or https://…/dns-query (DoH), comma-separated.",
    polSaved: "Rules saved.", polSavedWarn: "Rules saved, but:",
    polTestTitle: "Test", polTestLabel: "Domain or IP address", polTestBtn: "Test", polBy: "rule",
    polGeoTitle: "geosite and geoip databases", polGeoUpdate: "Update databases", polGeoUpdated: "Geo databases updated.",
    polGeoFile: "File", polGeoSize: "Size", polGeoDate: "Updated", polGeoLoaded: "Loaded categories",
    polGeoHave: "Present", polGeoNone: "Missing", polGeoMissing: "Not in the database",
    navRouter: "RouterOS", rosTitle: "RouterOS integration", rosSub: "The container tells the router which traffic goes to the VPN, as an address list or BGP routes, and forwards DNS for the needed domains to itself.",
    rosNoGateway: "The integration works only in gateway mode: turn on AWG_AUTO_ROUTE and check the LAN interface.",
    rosSyncNow: "Sync now", rosSynced: "Sync done.", rosSaved: "RouterOS settings saved.", rosTest: "Test connection", rosTestOk: "Connection works: {who}.",
    rosModeLabel: "Mode", rosMode_off: "Off", rosMode_address_list: "Address-List", rosMode_bgp: "BGP",
    rosModeHint_off: "The router is set up by the router-setup.rsc scripts only.",
    rosModeHint_address_list: "PROXY networks and DNS-learned addresses go into an address list; the router's mangle sends them to the container.",
    rosModeHint_bgp: "PROXY networks and DNS-learned addresses are announced to the router over BGP with next-hop = container. IPv4 only.",
    rosNoRouter: "router not queried yet", rosStatic: "Networks from rules", rosStaticNote: "PROXY: networks and geoip", rosDropped: "{n} over the limit",
    rosDynamic: "Addresses from DNS", rosDynamicNote: "with a limited lifetime", rosDomains: "Domains in DNS FWD", rosFwdTo: "forwarded to {ip}", rosDnsOff: "forwarding is off",
    rosAnnounced: "Announced networks", rosBgpErr: "Last error", rosBgp_idle: "Not running", rosBgp_connecting: "Connecting", rosBgp_established: "Established", rosBgp_error: "Error",
    rosSettings: "Connection and sync", rosURL: "Router address", rosURLHint: "https://IP; http only for a private IP on the veth.",
    rosUser: "User", rosPass: "Password", rosPassKept: "saved — leave empty", rosPassHint: "Stored in state.json (0600) and never returned.",
    rosPin: "Certificate pin", rosPinHint: "sha256//… (SPKI) or the certificate's SHA-256: for the router's self-signed certificate.",
    rosList: "Address-list name", rosListHint: "Any list, including one you already have (awg31 by default). Use the same name in :global AWG31LIST for router-setup.rsc. Other entries are left alone.",
    rosFwd: "Container IP", rosFwdHint: "Empty — detect from the LAN interface.", rosLimit: "Network limit", rosDynMin: "Lifetime of DNS addresses, min", rosDomLimit: "Domain limit",
    rosLocalAS: "Container AS", rosRemoteAS: "Router AS", rosPeer: "Router IP for BGP", rosPeerHint: "Empty — the LAN gateway.",
    rosSyncDNS: "Forward DNS of domain rules to the container", rosSyncDNSHint: "FWD entries in /ip/dns/static for domain:, full: and geosite:, so the container does not have to serve all DNS.",
    rosInsecure: "Do not verify the router certificate", rosInsecureHint: "Only without a pin: otherwise the password can be intercepted.",
    rosOnRouter: "What to set up on the router", rosOnRouterSub: "Run once in the RouterOS terminal (values come from the settings).",
    rosScrUser: "least-privilege user for the container",
    rosScrPass: "password",
    rosScrRest: "REST API for the container only (https; self-signed certificate, so set the pin)",
    rosScrBgp: "BGP: the router accepts only networks from the container, no default route",
    navGroups: "Tunnel groups", grpTitle: "Tunnel groups", grpSub: "Several VPS servers as one exit: connections are spread across the members and a failed one leaves the group on its own.",
    grpAdd: "Add group", grpNone: "No groups yet", grpNoneHint: "Create a group, add tunnels to it and use it as the primary exit or in rules: PROXY@group_name.",
    grpName: "Group name", grpMode: "Mode", grp_mode_balance: "Balancing", grp_mode_ordered: "In order",
    grpModeNote_balance: "New connections are spread across the healthy members by weight; one flow stays in one tunnel.",
    grpModeNote_ordered: "Traffic goes through the first healthy member; if it fails, through the next one.",
    grpMembers: "Members", grpColMember: "Tunnel", grpColWeight: "Weight", grpAddMember: "Tunnel to add", grpAddBtn: "Add",
    grpNoMembers: "The group has no tunnels.", grpNoFree: "No free client profiles.", grpInRoute: "In the route", grpNotSaved: "Not saved",
    grpRemove: "Remove", grpRoute: "Route", grpTable: "table {n}", grpActiveOf: "{a} of {n} in the route",
    grpEmpty: "No healthy members: nothing goes through the group right now (with the kill switch on, traffic is blocked).",
    grpSaved: "Groups saved.", grpSavedWarn: "Saved, but not everything came up",
    grpNoGateway: "Groups work only in gateway mode: turn on AWG_AUTO_ROUTE and check the LAN interface on the Maintenance page.",
    grpHashWarn: "fib_multipath_hash_policy is \"{v}\" now: the kernel spreads flows by address, not by port, so all traffic of one client will use one tunnel. Set net.ipv4.fib_multipath_hash_policy=1 (container sysctls).",
    grpStatCount: "Groups", grpStatCountNote: "configured", grpStatHash: "Flow hash", grpHashPorts: "by ports", grpHashAddr: "by addresses", grpHashUnknown: "unknown",
    grpUsage: "How to use", grpMakePrimary: "Make primary", grpDeleteGroup: "Delete group",
    grpUsageText: "Write PROXY@group_name in the routing rules (for example, PROXY@vps domain:example.com). The Make primary button sends all LAN traffic through the group. IPv6 goes through one member of the group.",
    foNoteGroup: "The primary is the group \"{name}\": the priority list is not used for switching, the group checks its members and drops failed ones from the route itself.",
    netTitle: "Network", netGateway: "Gateway mode", netLan: "LAN interface", netKill: "Kill switch", netIPv6: "IPv6 in the tunnel", netTable: "Routing table",
    netKillErr: "not active", netOn: "on", netOff: "off", netHint: "Set by AWG_AUTO_ROUTE, AWG_LAN_IFACE, AWG_KILLSWITCH and AWG_ENABLE_IPV6. With the kill switch on, LAN traffic is dropped while no tunnel is up.",
    aboutTitle: "About the container", aboutUser: "Administrator",
    aboutTls: "Connection", tlsOk: "protected by TLS", tlsNo: "no TLS: do not expose this port to a network", aboutServerErr: "Error messages come from the server in Russian.",
  },
};
const LANG_KEY = "awg31_lang";
function pickLang() {
  try { const v = localStorage.getItem(LANG_KEY); if (v === "ru" || v === "en") return v; } catch (_) { /* хранилище недоступно */ }
  return (navigator.language || "ru").toLowerCase().startsWith("en") ? "en" : "ru";
}
let lang = pickLang();
function t(key, params) {
  let s = (I18N[lang] && I18N[lang][key]) || I18N.ru[key] || key;
  if (params) for (const [k, v] of Object.entries(params)) s = s.split("{" + k + "}").join(String(v));
  return s;
}
function setLang(l) {
  lang = l;
  try { localStorage.setItem(LANG_KEY, l); } catch (_) { /* не критично */ }
  document.documentElement.lang = l;
  render();
}

/* ---------- DOM-помощники ---------- */
function h(tag, attrs, ...kids) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs || {})) {
    if (v == null || v === false) continue;
    if (k === "class") el.className = v;
    else if (k.startsWith("on")) el.addEventListener(k.slice(2), v);
    else if (k === "value" || k === "checked" || k === "disabled" || k === "selected") el[k] = v;
    else if (v === true) el.setAttribute(k, "");
    else el.setAttribute(k, v);
  }
  for (const kid of kids.flat(Infinity)) {
    if (kid == null || kid === false) continue;
    el.append(kid.nodeType ? kid : document.createTextNode(String(kid)));
  }
  return el;
}

async function api(method, path, body) {
  const opts = { method, headers: {}, credentials: "same-origin" };
  if (body !== undefined) {
    opts.headers["Content-Type"] = "application/json";
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(path, opts);
  let data = {};
  try { data = await res.json(); } catch (_) { /* пустой ответ */ }
  if (!res.ok) {
    const err = new Error(data.error || ("HTTP " + res.status));
    err.status = res.status;
    err.data = data;
    throw err;
  }
  return data;
}

/* ---------- форматирование ---------- */
function fmtBytes(n) {
  const u = lang === "ru" ? ["Б", "КиБ", "МиБ", "ГиБ", "ТиБ"] : ["B", "KiB", "MiB", "GiB", "TiB"];
  let i = 0;
  n = Number(n) || 0;
  while (n >= 1024 && i < u.length - 1) { n /= 1024; i++; }
  return (i ? n.toFixed(1) : String(n)) + " " + u[i];
}
function fmtAgo(unix) {
  if (!unix) return t("never");
  const s = Math.max(0, Math.floor(Date.now() / 1000 - unix));
  let n = s, u = "s";
  if (s >= 86400) { n = Math.floor(s / 86400); u = "d"; }
  else if (s >= 3600) { n = Math.floor(s / 3600); u = "h"; }
  else if (s >= 60) { n = Math.floor(s / 60); u = "m"; }
  return t("ago_" + u).replace("{n}", String(n));
}
function fmtDate(iso) {
  const d = new Date(iso);
  if (!iso || isNaN(d) || d.getFullYear() < 2000) return t("never");
  return new Intl.DateTimeFormat(lang, { dateStyle: "medium", timeStyle: "short" }).format(d);
}
function fmtInterval(min) {
  if (!min) return t("subOff");
  if (min % 1440 === 0) return t("days", { n: min / 1440 });
  if (min % 60 === 0) return t("hours", { n: min / 60 });
  return t("minutes", { n: min });
}
function shortKey(k) { return k && k.length > 14 ? k.slice(0, 6) + "…" + k.slice(-6) : (k || "—"); }

/* ---------- состояние ---------- */
const S = {
  auth: null,        // {configured, authenticated, user}
  data: null,        // ответ /api/status
  route: { page: "overview" },
  flash: null,       // {kind: ok|error|warn, text}
  dirty: false,      // в форме есть несохранённый ввод: не перерисовывать страницу по таймеру
  confirm: "",       // ключ кнопки, ожидающей подтверждения
  busy: false,
  newMode: "conf",
  newName: "",       // имя нового профиля переживает переключение режима
  authError: "",
};
const root = document.getElementById("app");
let sideEl = null, mainEl = null;

function say(kind, text) { S.flash = { kind, text }; }

function parseRoute() {
  const parts = (location.hash || "#/").replace(/^#\/?/, "").split("/").filter(Boolean).map(decodeURIComponent);
  if (parts[0] === "profiles") return { page: "profiles" };
  if (parts[0] === "new") return { page: "new" };
  if (parts[0] === "maintenance") return { page: "maintenance" };
  if (parts[0] === "failover") return { page: "failover" };
  if (parts[0] === "policy") return { page: "policy" };
  if (parts[0] === "groups") return { page: "groups" };
  if (parts[0] === "router") return { page: "router" };
  if (parts[0] === "p" && parts[1]) return { page: "profile", name: parts[1], tab: ["config", "sub"].includes(parts[2]) ? parts[2] : "overview" };
  return { page: "overview" };
}
function go(path) {
  S.flash = null; S.dirty = false; S.confirm = "";
  if (location.hash === "#" + path) { S.route = parseRoute(); renderSide(); renderMain(); } else location.hash = path;
}
window.addEventListener("hashchange", () => {
  S.route = parseRoute(); S.flash = null; S.dirty = false; S.confirm = "";
  if (S.route.page === "failover") { S.foEdit = null; loadFailover(true); }
  if (S.route.page === "groups") { S.grpEdit = null; S.grpWarn = null; loadGroups(true); }
  if (S.route.page === "policy") { S.polEdit = null; S.polTest = null; loadPolicy(true); }
  if (S.route.page === "router") { S.rosEdit = null; loadRouter(true); }
  if (S.auth && S.auth.authenticated) { renderSide(); renderMain(); }
});
document.addEventListener("input", (e) => { if (e.target.closest && e.target.closest("#main form")) S.dirty = true; });

async function act(fn, ok) {
  if (S.busy) return;
  S.busy = true;
  try {
    const r = await fn();
    if (ok) say("ok", typeof ok === "function" ? ok(r) : ok);
    S.dirty = false;
  } catch (e) {
    S.busy = false;
    if (e.status === 401) { S.auth = { configured: true, authenticated: false }; render(); return; }
    // Ошибка: страницу не перерисовываем, чтобы не стереть введённое (конфиг, пароль).
    say("error", e.message);
    renderFlash();
    return;
  }
  S.busy = false; S.confirm = "";
  await refresh();
}

async function refresh(quiet) {
  try {
    S.data = await api("GET", "/api/status");
  } catch (e) {
    if (e.status === 401) { S.auth = { configured: true, authenticated: false }; render(); return; }
    if (!quiet) say("error", e.message);
  }
  if (quiet) { renderSide(); if (!S.dirty && !S.busy) renderMain(); } else { renderSide(); renderMain(); }
}

async function boot() {
  try { S.auth = await api("GET", "/api/auth/status"); } catch (e) { S.auth = { configured: true, authenticated: false }; S.authError = e.message; }
  S.route = parseRoute();
  if (S.auth.authenticated) await refresh(); else render();
  if (S.auth.authenticated) render();
}

/* ---------- каркас ---------- */
function topbar() {
  const authed = S.auth && S.auth.authenticated;
  return h("header", { class: "topbar" },
    h("a", { class: "brand", href: "#/", "aria-label": "AmneziaWG 3.1" },
      h("span", { class: "brand-emblem" }, icon("shield", 20)), h("span", {}, "AmneziaWG ", h("b", {}, "3.1"))),
    h("div", { class: "top-actions" },
      h("div", { class: "lang", role: "group", "aria-label": t("language") },
        ["ru", "en"].map((l) => h("button", { type: "button", "aria-pressed": String(lang === l), onclick: () => setLang(l) }, l.toUpperCase()))),
      authed && h("button", { class: "icon-btn", type: "button", title: t("refresh"), "aria-label": t("refresh"), onclick: () => refresh() }, icon("restart", 18)),
      authed && h("button", { class: "text-btn", type: "button", onclick: logout }, icon("logout", 18), h("span", { class: "lbl" }, t("logout")))));
}

async function logout() {
  try { await api("POST", "/api/auth/logout"); } catch (_) { /* уже вышли */ }
  S.auth = { configured: true, authenticated: false }; S.data = null; S.flash = null;
  render();
}

function render() {
  document.title = "AmneziaWG 3.1 · AWG Console";
  document.documentElement.lang = lang;
  if (!S.auth) { root.replaceChildren(topbar(), h("div", { class: "loading" }, t("loading"))); return; }
  if (!S.auth.configured) { root.replaceChildren(topbar(), authView("setup")); return; }
  if (!S.auth.authenticated) { root.replaceChildren(topbar(), authView("login")); return; }
  sideEl = h("nav", { class: "sidebar", "aria-label": "AWG Console" });
  mainEl = h("main", { class: "main", id: "main" });
  root.replaceChildren(topbar(), h("div", { class: "shell" }, sideEl, mainEl));
  renderSide(); renderMain();
}

function profiles() { return (S.data && S.data.profiles) || []; }
function agg(p) {
  const peers = (p.iface && p.iface.peers) || [];
  let rx = 0, tx = 0, hs = 0;
  for (const x of peers) { rx += x.rx_bytes || 0; tx += x.tx_bytes || 0; hs = Math.max(hs, x.latest_handshake || 0); }
  return { rx, tx, hs, peers };
}
function addrOf(p) { return (p.summary && p.summary.addresses && p.summary.addresses[0]) || "—"; }

function renderSide() {
  if (!sideEl) return;
  const r = S.route, list = profiles();
  const link = (page, ico, label, path) => h("a", { class: "side-link", href: "#" + path, "aria-current": r.page === page ? "page" : null, title: label }, icon(ico, 20), h("span", { class: "lbl" }, label));
  sideEl.replaceChildren(...[
    h("div", { class: "eyebrow side-eyebrow" }, t("console")),
    h("div", { class: "side-title" }, h("span", {}, t("navInfra"))),
    link("overview", "home", t("navOverview"), "/"),
    link("profiles", "server", t("navAll"), "/profiles"),
    link("failover", "layers", t("navFailover"), "/failover"),
    link("groups", "merge", t("navGroups"), "/groups"),
    link("policy", "route", t("navPolicy"), "/policy"),
    h("div", { class: "side-title side-group" }, h("span", {}, t("navProfiles")), h("span", { class: "badge" }, String(list.length))),
    list.map((p) => h("a", { class: "side-inst", href: "#/p/" + encodeURIComponent(p.name), "aria-current": r.page === "profile" && r.name === p.name ? "page" : null, title: p.name },
      h("span", { class: "dot" + (p.up ? " on" : "") }),
      h("span", { class: "names" }, h("span", { class: "n" }, p.name), h("span", { class: "a" }, addrOf(p))))),
    h("div", { class: "side-title" }, h("span", {}, t("navSystem"))),
    link("router", "link", t("navRouter"), "/router"),
    link("maintenance", "settings", t("navMaint"), "/maintenance"),
    h("div", { class: "side-foot" }, icon("tag", 16), h("span", { class: "ver" }, "v" + ((S.data && S.data.version) || "…"))),
  ].flat(Infinity));
}

function renderMain() {
  if (!mainEl) return;
  const r = S.route;
  const page = r.page === "profiles" ? pageProfiles() : r.page === "profile" ? pageProfile(r)
    : r.page === "new" ? pageNew() : r.page === "maintenance" ? pageMaintenance()
    : r.page === "failover" ? pageFailover() : r.page === "groups" ? pageGroups() : r.page === "policy" ? pagePolicy() : r.page === "router" ? pageRouter() : pageOverview();
  mainEl.replaceChildren(h("div", { id: "flash-slot" }, flashBox()), page);
}

// Обновляет только блок сообщения: введённое в форме при этом сохраняется.
function renderFlash() {
  const slot = document.getElementById("flash-slot");
  if (!slot) return;
  const box = flashBox();
  slot.replaceChildren(...(box ? [box] : []));
  if (box && S.flash.kind === "error") slot.scrollIntoView({ block: "nearest" });
}

function flashBox() {
  if (!S.flash) return null;
  const f = S.flash;
  return h("div", { class: "flash " + f.kind, role: f.kind === "error" ? "alert" : "status" },
    icon(f.kind === "ok" ? "check" : "alert", 18), h("div", { class: "msg" }, f.text),
    h("button", { type: "button", "aria-label": t("close"), onclick: () => { S.flash = null; renderFlash(); } }, icon("x", 16)));
}

function heading(eyebrow, title, sub, actions) {
  return h("div", { class: "page-heading" }, h("div", { class: "heading-row" },
    h("div", {}, h("div", { class: "eyebrow" }, eyebrow), h("h1", {}, title), sub && h("p", { class: "heading-sub" }, sub)),
    actions && h("div", { class: "actions" }, actions)));
}
function pill(p) { return h("span", { class: "pill " + (p.up ? "pill-on" : "pill-off") }, p.up ? t("running") : t("stopped")); }
function primaryBadge(p) { return p.primary && h("span", { class: "pill pill-warn", title: t("primaryHint") }, t("primary")); }
const doPrimary = (name) => act(() => api("POST", "/api/profiles/" + encodeURIComponent(name) + "/primary"), t("primaryDone", { name }));
function primaryBtn(p, small) {
  const net = (S.data && S.data.network) || {};
  if (!p.up || p.primary || !net.gateway || !net.lan_iface) return null;
  return h("button", { class: "btn btn-secondary" + (small ? " btn-small" : ""), type: "button", disabled: S.busy, onclick: () => doPrimary(p.name) }, icon("star", 16), t("makePrimary"));
}

/* ---------- действия ---------- */
const doUp = (name) => act(() => api("POST", "/api/profiles/" + encodeURIComponent(name) + "/up"), t("upDone", { name }));
const doDown = (name) => act(() => api("POST", "/api/profiles/" + encodeURIComponent(name) + "/down"), t("downDone", { name }));
function toggleBtn(p, small) {
  return h("button", { class: "btn btn-secondary" + (small ? " btn-small" : ""), type: "button", disabled: S.busy,
    onclick: () => (p.up ? doDown(p.name) : doUp(p.name)) }, icon(p.up ? "pause" : "play", 16), p.up ? t("down") : t("up"));
}
function deleteBtn(p, small) {
  const key = "del:" + p.name, armed = S.confirm === key;
  return h("button", { class: "btn btn-danger" + (small ? " btn-small" : ""), type: "button", disabled: S.busy, onclick: () => {
    if (!armed) { S.confirm = key; renderMain(); setTimeout(() => { if (S.confirm === key) { S.confirm = ""; if (!S.dirty) renderMain(); } }, 5000); return; }
    act(async () => { await api("DELETE", "/api/profiles/" + encodeURIComponent(p.name)); go("/profiles"); }, t("deleted", { name: p.name }));
  } }, icon("trash", 16), armed ? t("confirmDelete") : t("delete"));
}

/* ---------- страницы ---------- */
function statCard(label, value, note) {
  return h("div", { class: "stat" }, h("div", { class: "stat-label" }, label), h("div", { class: "stat-value" }, value), h("div", { class: "stat-note" }, note));
}

function pageOverview() {
  const list = profiles();
  const up = list.filter((p) => p.up).length;
  let rx = 0, tx = 0, hs = 0;
  for (const p of list) { const a = agg(p); rx += a.rx; tx += a.tx; hs = Math.max(hs, a.hs); }
  return h("div", {},
    heading(t("console"), t("overviewTitle")),
    S.data && S.data.tunnel_error && h("div", { class: "flash warn", role: "alert" }, icon("alert", 18), h("div", { class: "msg" }, t("tunnelError", { err: S.data.tunnel_error }))),
    h("div", { class: "grid-stats" },
      statCard(t("statTunnels"), up + " / " + list.length, t("statTunnelsNote")),
      statCard(t("statRx"), fmtBytes(rx), t("statSumNote")), statCard(t("statTx"), fmtBytes(tx), t("statSumNote")),
      statCard(t("statHandshake"), hs ? fmtAgo(hs) : "—", t("statHandshakeNote"))),
    h("div", { class: "section-title" }, h("h2", {}, t("profilesTitle")),
      h("a", { href: "#/profiles" }, t("manageAll") + " →")),
    list.length ? h("div", { class: "inst-grid" }, list.map(profileCard)) : emptyState());
}

function emptyState() {
  return h("div", { class: "card empty" }, icon("server", 32), h("strong", {}, t("noProfiles")), h("p", {}, t("noProfilesHint")),
    h("a", { class: "btn btn-primary", href: "#/new" }, icon("plus", 16), t("addProfile")));
}

function profileCard(p) {
  const a = agg(p);
  const ep = (a.peers[0] && a.peers[0].endpoint) || (p.summary && p.summary.endpoints && p.summary.endpoints[0]) || "";
  return h("article", { class: "inst" },
    h("div", { class: "inst-head" }, h("span", { class: "inst-icon" }, icon("server", 22)),
      h("div", { class: "t" }, h("h3", {}, p.name), h("div", { class: "s" }, [addrOf(p), ep].filter((x) => x && x !== "—").join(" · ") || "—")), primaryBadge(p), pill(p)),
    p.error && h("div", { class: "flash error", role: "alert" }, icon("alert", 18), h("div", { class: "msg" }, p.error)),
    h("div", { class: "metrics" },
      h("div", { class: "metric" }, h("div", { class: "l" }, h("span", { class: "legend rx" }), t("rx")), h("div", { class: "v" }, fmtBytes(a.rx))),
      h("div", { class: "metric" }, h("div", { class: "l" }, h("span", { class: "legend tx" }), t("tx")), h("div", { class: "v" }, fmtBytes(a.tx))),
      h("div", { class: "metric" }, h("div", { class: "l" }, t("handshake")), h("div", { class: "v" }, a.hs ? fmtAgo(a.hs) : "—"))),
    h("div", { class: "actions" },
      h("a", { class: "btn btn-primary btn-small", href: "#/p/" + encodeURIComponent(p.name) }, t("manage")), toggleBtn(p, true)));
}

function pageProfiles() {
  const list = profiles();
  return h("div", {},
    heading(t("navProfiles"), t("profilesHeading"), t("profilesSub"),
      h("a", { class: "btn btn-primary", href: "#/new" }, icon("plus", 16), t("addProfile"))),
    !list.length ? emptyState() : h("div", { class: "table-scroll" }, h("table", {},
      h("thead", {}, h("tr", {}, ["colName", "colAddress", "colState", "colTraffic", "colSub", "colActions"].map((k) => h("th", { scope: "col" }, t(k))))),
      h("tbody", {}, list.map((p) => {
        const a = agg(p), sub = p.subscription;
        return h("tr", {},
          h("td", {}, h("a", { class: "name", href: "#/p/" + encodeURIComponent(p.name) }, icon("server", 18), p.name)),
          h("td", {}, h("span", { class: "ip-pill" }, addrOf(p))),
          h("td", {}, h("div", { class: "actions" }, pill(p), primaryBadge(p)), h("div", { class: "sub" }, t("handshake") + ": " + (a.hs ? fmtAgo(a.hs) : t("never")))),
          h("td", { class: "mono" }, "↓ " + fmtBytes(a.rx), h("br"), "↑ " + fmtBytes(a.tx)),
          h("td", {}, sub ? fmtInterval(sub.interval_min) : t("subNone"),
            sub && sub.last_error && h("div", { class: "sub" }, h("span", { class: "pill pill-off" }, t("subLastError")))),
          h("td", {}, h("div", { class: "actions" }, toggleBtn(p, true), deleteBtn(p, true))));
      })))));
}

function pageProfile(r) {
  const p = profiles().find((x) => x.name === r.name);
  if (!p) return h("div", {}, heading(t("navProfiles"), t("profileMissing")), h("a", { class: "btn btn-secondary", href: "#/profiles" }, t("back")));
  const tab = (id, label) => h("button", { class: "tab", role: "tab", type: "button", "aria-selected": String(r.tab === id),
    onclick: () => go("/p/" + encodeURIComponent(p.name) + (id === "overview" ? "" : "/" + id)) }, label);
  const sub = (p.summary && [addrOf(p), (p.summary.endpoints || [])[0]].filter((x) => x && x !== "—").join(" · ")) || "";
  return h("div", {},
    heading(t("profileEyebrow", { name: p.name.toUpperCase() }), t("profileTitle", { name: p.name }), sub || t("noConfigYet"), [primaryBadge(p), pill(p), primaryBtn(p), toggleBtn(p), deleteBtn(p)]),
    h("div", { class: "tabs", role: "tablist" }, tab("overview", t("tabOverview")), tab("config", t("tabConfig")), tab("sub", t("tabSub"))),
    r.tab === "config" ? tabConfig(p) : r.tab === "sub" ? tabSub(p) : tabOverview(p));
}

function kv(rows) {
  return h("dl", { class: "kv" }, rows.map(([k, v]) => [h("dt", {}, k), h("dd", {}, v)]));
}

function tabOverview(p) {
  const a = agg(p), s = p.summary, sub = p.subscription;
  return h("div", { class: "stack" },
    sub && sub.notice && h("div", { class: "flash warn" }, icon("info", 18), h("div", { class: "msg" }, sub.notice)),
    p.error && h("div", { class: "flash error", role: "alert" }, icon("alert", 18), h("div", { class: "msg" }, p.error)),
    h("div", { class: "card" }, h("h2", {}, t("tabOverview")),
      s ? kv([
        [t("cfgAddresses"), (s.addresses || []).join(", ") || "—"], [t("cfgEndpoints"), (s.endpoints || []).join(", ") || "—"],
        [t("cfgPeers"), String(s.peers)], [t("cfgObf"), s.obfuscation ? t("on") : t("off")], [t("cfgPsk"), s.has_psk ? t("present") : t("absent")],
      ]) : h("p", { class: "card-hint" }, t("noConfigYet")),
      h("p", { class: "info-line" }, icon("info", 16), h("span", {}, t("autostartInfo", { state: p.autostart ? t("autostartYes") : t("autostartNo") })))),
    a.peers.length ? h("div", { class: "table-scroll" }, h("table", {},
      h("thead", {}, h("tr", {}, ["peerKey", "peerEndpoint", "peerHandshake", "peerTraffic", "peerAllowed"].map((k) => h("th", { scope: "col" }, t(k))))),
      h("tbody", {}, a.peers.map((x) => h("tr", {},
        h("td", { class: "mono" }, shortKey(x.public_key)), h("td", { class: "mono" }, x.endpoint || "—"),
        h("td", {}, x.latest_handshake ? fmtAgo(x.latest_handshake) : t("never")),
        h("td", { class: "mono" }, "↓ " + fmtBytes(x.rx_bytes) + "  ↑ " + fmtBytes(x.tx_bytes)),
        h("td", { class: "mono" }, (x.allowed_ips || []).join(", ") || "—")))))) : null);
}

function tabConfig(p) {
  const ta = h("textarea", { id: "cfg", name: "content", spellcheck: "false", autocomplete: "off", placeholder: t("cfgPlaceholder") });
  return h("div", { class: "card" }, h("h2", {}, t("cfgTitle")), h("p", { class: "card-hint" }, t("cfgHint")),
    h("form", { onsubmit: (e) => {
      e.preventDefault();
      if (!ta.value.trim()) { say("error", t("cfgEmpty")); renderFlash(); return; }
      act(() => api("PUT", "/api/profiles/" + encodeURIComponent(p.name), { content: ta.value }).then((r) => { ta.value = ""; return r; }),
        (r) => (r.restart_required ? t("cfgSavedRestart") : t("cfgSaved")));
    } },
      h("div", { class: "field", }, h("label", { for: "cfg" }, ".conf"), ta),
      h("p", { class: "info-line" }, icon("shield", 16), h("span", {}, t("cfgHooks"))),
      h("div", { class: "actions" }, h("button", { class: "btn btn-primary", type: "submit", disabled: S.busy }, icon("check", 16), t("save")))));
}

function intervalSelect(current) {
  const base = [0, 5, 15, 30, 60, 360, 1440];
  const vals = base.includes(current) ? base : [...base, current].sort((a, b) => a - b);
  return h("select", { id: "sub-int", name: "interval" }, vals.map((v) => h("option", { value: String(v), selected: v === current }, v === 0 ? t("subIntervalOff") : fmtInterval(v))));
}

function tabSub(p) {
  const sub = p.subscription;
  const url = h("input", { id: "sub-url", name: "url", type: "url", autocomplete: "off", spellcheck: "false", placeholder: sub && sub.url ? sub.url : t("subUrlNew") });
  const token = h("input", { id: "sub-token", name: "token", type: "password", autocomplete: "off", placeholder: sub && sub.token_set ? t("subTokenSet") : "" });
  const clear = h("input", { type: "checkbox", id: "sub-clear" });
  const pin = h("input", { id: "sub-pin", name: "pin", type: "text", autocomplete: "off", spellcheck: "false", class: "mono", value: (sub && sub.pin_sha256) || "", placeholder: "sha256//…" });
  const insecure = h("input", { type: "checkbox", id: "sub-insecure", checked: !!(sub && sub.insecure_tls) });
  const interval = intervalSelect(sub ? sub.interval_min : 15);
  const save = (e) => {
    e.preventDefault();
    act(() => api("PUT", "/api/subscriptions/" + encodeURIComponent(p.name), {
      url: url.value.trim(), token: token.value.trim(), clear_token: clear.checked, pin_sha256: pin.value.trim(),
      insecure_tls: insecure.checked, interval_min: parseInt(interval.value, 10) || 0 }), t("subSaved"));
  };
  const sync = () => act(async () => {
    try { return await api("POST", "/api/subscriptions/" + encodeURIComponent(p.name) + "/sync"); }
    catch (e) { throw e; }
  }, (r) => (r.changed ? t("subSynced") : t("subSyncedSame")));
  return h("div", { class: "stack" },
    sub && sub.notice && h("div", { class: "flash warn" }, icon("info", 18), h("div", { class: "msg" }, sub.notice)),
    sub && sub.last_error && h("div", { class: "flash error", role: "alert" }, icon("alert", 18), h("div", { class: "msg" }, sub.last_error)),
    h("div", { class: "card" }, h("h2", {}, t("subTitle")), h("p", { class: "card-hint" }, t("subHint")),
      sub && kv([[t("subLast"), fmtDate(sub.last_sync)], [t("subPinState"), sub.pin_sha256 ? shortKey(sub.pin_sha256) : (sub.insecure_tls ? t("subTlsInsecure") : t("subTlsOk"))]]),
      h("form", { onsubmit: save },
        h("div", { class: "field" }, h("label", { for: "sub-url" }, t("subUrl")), url, sub && h("span", { class: "hint" }, t("subUrlKeep"))),
        h("div", { class: "fields-grid" },
          h("div", { class: "field" }, h("label", { for: "sub-token" }, t("subToken")), token),
          h("div", { class: "field" }, h("label", { for: "sub-int" }, t("subInterval")), interval)),
        sub && sub.token_set && h("label", { class: "check", for: "sub-clear" }, clear, h("span", {}, t("subClearToken"))),
        h("div", { class: "field" }, h("label", { for: "sub-pin" }, t("subPin")), pin, h("span", { class: "hint" }, t("subPinHint"))),
        h("label", { class: "check", for: "sub-insecure" }, insecure, h("span", {}, t("subInsecure"), h("br"), t("subInsecureHint"))),
        h("div", { class: "actions" },
          h("button", { class: "btn btn-primary", type: "submit", disabled: S.busy }, icon("check", 16), t("subSave")),
          sub && h("button", { class: "btn btn-secondary", type: "button", disabled: S.busy, onclick: sync }, icon("restart", 16), t("subSync")),
          sub && h("button", { class: "btn btn-danger", type: "button", disabled: S.busy,
            onclick: () => act(() => api("DELETE", "/api/subscriptions/" + encodeURIComponent(p.name)), t("subRemoved")) }, icon("trash", 16), t("subRemove"))))));
}

function pageNew() {
  const name = h("input", { id: "new-name", name: "name", maxlength: "15", autocomplete: "off", spellcheck: "false", class: "mono", placeholder: "awg0",
    value: S.newName, oninput: (e) => { S.newName = e.target.value; } });
  const conf = h("textarea", { id: "new-conf", name: "content", spellcheck: "false", autocomplete: "off", placeholder: t("cfgPlaceholder") });
  const url = h("input", { id: "new-url", name: "url", type: "url", autocomplete: "off", spellcheck: "false", placeholder: t("subUrlNew") });
  const pin = h("input", { id: "new-pin", name: "pin", autocomplete: "off", spellcheck: "false", class: "mono", placeholder: "sha256//…" });
  const submit = (e) => {
    e.preventDefault();
    const n = name.value.trim();
    if (!n) { say("error", t("needName")); renderFlash(); return; }
    const enc = encodeURIComponent(n);
    act(async () => {
      if (S.newMode === "conf") {
        if (!conf.value.trim()) throw new Error(t("cfgEmpty"));
        await api("PUT", "/api/profiles/" + enc, { content: conf.value });
      } else {
        await api("PUT", "/api/subscriptions/" + enc, { url: url.value.trim(), pin_sha256: pin.value.trim(), interval_min: 15 });
        await api("POST", "/api/subscriptions/" + enc + "/sync");
      }
      S.newName = "";
      go("/p/" + enc);
    }, t("created"));
  };
  const mode = (id, label) => h("button", { type: "button", "aria-pressed": String(S.newMode === id), onclick: () => { S.newMode = id; renderMain(); } }, label);
  return h("div", {}, heading(t("navProfiles"), t("newTitle"), t("newSub")),
    h("div", { class: "card" }, h("form", { onsubmit: submit },
      h("div", { class: "field" }, h("label", { for: "new-name" }, t("profileName")), name, h("span", { class: "hint" }, t("nameHint"))),
      h("div", { class: "seg", role: "group" }, mode("conf", t("modeConf")), mode("sub", t("modeSub"))),
      S.newMode === "conf"
        ? [h("div", { class: "field" }, h("label", { for: "new-conf" }, ".conf"), conf), h("p", { class: "info-line" }, icon("shield", 16), h("span", {}, t("cfgHooks")))]
        : [h("div", { class: "field" }, h("label", { for: "new-url" }, t("subUrl")), url),
           h("div", { class: "field" }, h("label", { for: "new-pin" }, t("subPin")), pin, h("span", { class: "hint" }, t("subPinHint")))],
      h("div", { class: "actions" }, h("button", { class: "btn btn-primary", type: "submit", disabled: S.busy }, icon("plus", 16), t("create"))))));
}

function pageMaintenance() {
  const cur = h("input", { id: "pw-cur", type: "password", autocomplete: "current-password" });
  const nw = h("input", { id: "pw-new", type: "password", autocomplete: "new-password" });
  const rep = h("input", { id: "pw-rep", type: "password", autocomplete: "new-password" });
  const secure = location.protocol === "https:";
  return h("div", {}, heading(t("navSystem"), t("maintTitle"), t("maintSub")),
    h("div", { class: "card" }, h("h2", {}, t("pwTitle")),
      h("form", { onsubmit: (e) => {
        e.preventDefault();
        if (nw.value !== rep.value) { say("error", t("passwordsDiffer")); renderFlash(); return; }
        act(async () => { await api("POST", "/api/auth/password", { current_password: cur.value, new_password: nw.value }); cur.value = nw.value = rep.value = ""; }, t("pwChanged"));
      } },
        h("div", { class: "fields-grid" },
          h("div", { class: "field" }, h("label", { for: "pw-cur" }, t("pwCurrent")), cur), h("div", {}),
          h("div", { class: "field" }, h("label", { for: "pw-new" }, t("pwNew")), nw),
          h("div", { class: "field" }, h("label", { for: "pw-rep" }, t("pwRepeat")), rep)),
        h("div", { class: "actions" }, h("button", { class: "btn btn-primary", type: "submit", disabled: S.busy }, icon("key", 16), t("pwChange"))))),
    networkCard(),
    h("div", { class: "card" }, h("h2", {}, t("aboutTitle")), kv([
      [t("version"), "v" + ((S.data && S.data.version) || "—")], [t("aboutUser"), (S.data && S.data.user) || "—"],
      [t("aboutTls"), secure ? t("tlsOk") : t("tlsNo")]]),
      lang === "en" && h("p", { class: "info-line" }, icon("info", 16), h("span", {}, t("aboutServerErr")))));
}

function networkCard() {
  const n = (S.data && S.data.network) || {};
  const onOff = (v) => (v ? t("netOn") : t("netOff"));
  return h("div", { class: "card" }, h("h2", {}, t("netTitle")), kv([
    [t("netGateway"), onOff(n.gateway)],
    [t("netLan"), n.lan_iface || "—"],
    [t("netKill"), n.error ? t("netKillErr") + ": " + n.error : onOff(n.kill_switch)],
    [t("netIPv6"), onOff(n.ipv6)],
    [t("netTable"), n.gateway && n.lan_iface ? String(n.table) : "—"]]),
    h("p", { class: "info-line" }, icon("info", 16), h("span", {}, t("netHint"))));
}

/* ---------- резервирование (Multi-Tunnel + failover) ---------- */
async function loadFailover(quiet) {
  try {
    S.fo = await api("GET", "/api/failover");
    if (!S.foEdit || !S.dirty) S.foEdit = { settings: { ...S.fo.settings }, order: [...S.fo.order] };
  } catch (e) {
    if (!quiet) { say("error", e.message); renderFlash(); }
  }
  if (S.route.page === "failover" && !S.dirty && !S.busy) renderMain();
}

function foStatePill(hh) {
  const cls = { healthy: "pill-on", unhealthy: "pill-off", failing: "pill-warn", cooldown: "pill-warn", starting: "pill-idle", down: "pill-idle", excluded: "pill-idle" }[hh.state] || "pill-idle";
  return h("span", { class: "pill " + cls }, t("foState_" + hh.state));
}

function pageFailover() {
  if (!S.fo || !S.foEdit) { loadFailover(); return h("div", {}, heading(t("navInfra"), t("foTitle")), h("div", { class: "loading" }, t("loading"))); }
  const fo = S.fo, ed = S.foEdit, st = ed.settings;
  const markDirty = () => { S.dirty = true; };
  const num = (id, key, min, max) => h("input", { id, type: "number", min: String(min), max: String(max), value: String(st[key]),
    oninput: (e) => { st[key] = parseInt(e.target.value, 10) || 0; markDirty(); } });
  const sel = (id, key, opts) => h("select", { id, onchange: (e) => { st[key] = e.target.value; markDirty(); renderMain(); } },
    opts.map((o) => h("option", { value: o, selected: st[key] === o }, t("fo_" + key + "_" + o))));
  const health = {};
  for (const x of fo.tunnels || []) health[x.name] = x;
  const all = profiles().map((p) => p.name);
  const excluded = all.filter((n) => !ed.order.includes(n));
  const move = (i, d) => { const o = ed.order; [o[i], o[i + d]] = [o[i + d], o[i]]; S.dirty = true; renderMain(); };
  const save = () => act(async () => {
    S.fo = await api("PUT", "/api/failover", { settings: st, priority: ed.order });
    S.foEdit = { settings: { ...S.fo.settings }, order: [...S.fo.order] };
  }, t("foSaved"));
  const check = () => act(async () => { S.fo = await api("POST", "/api/failover/check"); S.foEdit = { settings: { ...S.fo.settings }, order: [...S.fo.order] }; }, t("foChecked"));

  const row = (name, i) => {
    const hh = health[name] || { name, state: "down", handshake_age: -1 };
    const p = profiles().find((x) => x.name === name) || { name };
    return h("tr", {},
      h("td", { class: "mono" }, i >= 0 ? String(i + 1) : "—"),
      h("td", {}, h("a", { class: "name", href: "#/p/" + encodeURIComponent(name) }, icon("server", 18), name), p.primary && h("div", { class: "sub" }, primaryBadge(p))),
      h("td", {}, foStatePill(hh), hh.reason && h("div", { class: "sub" }, hh.reason), hh.cooldown_left > 0 && h("div", { class: "sub" }, t("foCooldownLeft", { n: hh.cooldown_left }))),
      h("td", { class: "mono" }, hh.handshake_age >= 0 ? fmtAgo(Math.floor(Date.now() / 1000) - hh.handshake_age) : "—",
        hh.probe_ok != null && h("div", { class: "sub" }, hh.probe_ok ? t("foProbeOk", { ms: hh.probe_ms }) : t("foProbeFail"))),
      h("td", {}, h("div", { class: "actions" },
        i >= 0 && h("button", { class: "icon-btn", type: "button", title: t("foUp"), "aria-label": t("foUp"), disabled: i === 0, onclick: () => move(i, -1) }, icon("arrowUp", 16)),
        i >= 0 && h("button", { class: "icon-btn", type: "button", title: t("foDown"), "aria-label": t("foDown"), disabled: i === ed.order.length - 1, onclick: () => move(i, 1) }, icon("arrowDown", 16)),
        i >= 0 ? h("button", { class: "btn btn-secondary btn-small", type: "button", onclick: () => { ed.order.splice(i, 1); S.dirty = true; renderMain(); } }, t("foExclude"))
          : h("button", { class: "btn btn-secondary btn-small", type: "button", onclick: () => { ed.order.push(name); S.dirty = true; renderMain(); } }, icon("plus", 16), t("foInclude")),
        p.up && primaryBtn(p, true))));
  };

  return h("div", {},
    heading(t("navInfra"), t("foTitle"), t("foSub"),
      h("button", { class: "btn btn-secondary", type: "button", disabled: S.busy, onclick: check }, icon("activity", 16), t("foCheckNow"))),
    !fo.gateway && h("div", { class: "flash warn", role: "status" }, icon("info", 18), h("div", { class: "msg" }, t("foNoGateway"))),
    fo.note && h("div", { class: "flash warn", role: "status" }, icon("info", 18), h("div", { class: "msg" }, t("foNoteGroup", { name: fo.primary }))),
    h("div", { class: "grid-stats" },
      statCard(t("foMode"), t("fo_mode_" + fo.settings.mode), t("foModeNote_" + fo.settings.mode)),
      statCard(t("primary"), fo.primary || "—", t("primaryHint")),
      statCard(t("foSwitches"), String(fo.switches || 0), fo.last_switch_reason || t("foNoSwitches")),
      statCard(t("foLastCheck"), fo.last_check && new Date(fo.last_check).getFullYear() > 2000 ? fmtAgo(Math.floor(new Date(fo.last_check).getTime() / 1000)) : "—", t("foEvery", { n: fo.settings.interval_sec }))),
    h("div", { class: "card" }, h("h2", {}, t("foPriority")), h("p", { class: "heading-sub" }, t("foPrioritySub")),
      !all.length ? emptyState() : h("div", { class: "table-scroll" }, h("table", {},
        h("thead", {}, h("tr", {}, ["foColRank", "colName", "colState", "foColCheck", "colActions"].map((k) => h("th", { scope: "col" }, t(k))))),
        h("tbody", {}, ed.order.map((n, i) => row(n, i)), excluded.map((n) => row(n, -1)))))),
    h("div", { class: "card" }, h("h2", {}, t("foSettings")),
      h("form", { onsubmit: (e) => { e.preventDefault(); save(); } },
        h("div", { class: "fields-grid" },
          h("div", { class: "field" }, h("label", { for: "fo-mode" }, t("foMode")), sel("fo-mode", "mode", ["off", "standby", "switch"]),
            h("div", { class: "hint" }, t("foModeNote_" + st.mode))),
          h("div", { class: "field" }, h("label", { for: "fo-trigger" }, t("foTrigger")), sel("fo-trigger", "trigger", ["hybrid", "handshake", "probe"])),
          st.trigger !== "handshake" && h("div", { class: "field" }, h("label", { for: "fo-probe" }, t("foProbe")),
            h("input", { id: "fo-probe", type: "text", class: "mono", value: st.probe_target, spellcheck: "false", oninput: (e) => { st.probe_target = e.target.value.trim(); markDirty(); } }),
            h("div", { class: "hint" }, t("foProbeHint"))),
          h("div", { class: "field" }, h("label", { for: "fo-int" }, t("foInterval")), num("fo-int", "interval_sec", 5, 3600)),
          h("div", { class: "field" }, h("label", { for: "fo-thr" }, t("foThreshold")), num("fo-thr", "threshold", 1, 20)),
          h("div", { class: "field" }, h("label", { for: "fo-cd" }, t("foCooldown")), num("fo-cd", "cooldown_sec", 0, 86400)),
          h("div", { class: "field" }, h("label", { for: "fo-hs" }, t("foHsMax")), num("fo-hs", "handshake_max_age", 130, 3600)),
          h("div", { class: "field" }, h("label", { for: "fo-grace" }, t("foGrace")), num("fo-grace", "grace_sec", 5, 600))),
        h("label", { class: "check" }, h("input", { type: "checkbox", id: "fo-fb", checked: st.failback, onchange: (e) => { st.failback = e.target.checked; markDirty(); } }),
          h("span", {}, t("foFailback"), h("br"), t("foFailbackHint"))),
        h("div", { class: "actions" }, h("button", { class: "btn btn-primary", type: "submit", disabled: S.busy }, icon("check", 16), t("foSave"))))));
}

/* ---------- группы туннелей ---------- */
const cloneGroups = (gs) => (gs || []).map((g) => ({ name: g.name, mode: g.mode, members: (g.members || []).map((m) => ({ name: m.name, weight: m.weight || 1 })) }));

async function loadGroups(quiet) {
  try {
    S.grp = await api("GET", "/api/groups");
    if (!S.grpEdit || !S.dirty) S.grpEdit = { groups: cloneGroups(S.grp.groups) };
  } catch (e) {
    if (!quiet) { say("error", e.message); renderFlash(); }
  }
  if (S.route.page === "groups" && !S.dirty && !S.busy) renderMain();
}

const doGroupPrimary = (name) => act(async () => {
  await api("POST", "/api/groups/" + encodeURIComponent(name) + "/primary");
  S.grp = await api("GET", "/api/groups");
  S.grpEdit = { groups: cloneGroups(S.grp.groups) };
}, t("primaryDone", { name }));

function pageGroups() {
  if (!S.grp || !S.grpEdit) { loadGroups(); return h("div", {}, heading(t("navInfra"), t("grpTitle")), h("div", { class: "loading" }, t("loading"))); }
  const v = S.grp, groups = S.grpEdit.groups, health = v.health || {};
  const dirty = () => { S.dirty = true; };
  const statusOf = (name) => (v.status || []).find((x) => x.name === name);
  const taken = (n) => groups.some((g) => g.name === n) || profiles().some((p) => p.name === n);
  const addGroup = () => {
    let i = 1;
    while (taken("group" + i)) i++;
    groups.push({ name: "group" + i, mode: "balance", members: [] });
    dirty(); renderMain();
  };
  const save = () => act(async () => {
    const body = groups.map((g) => ({ name: g.name.trim(), mode: g.mode, members: g.members.map((m) => ({ name: m.name, weight: m.weight || 1 })) }));
    const r = await api("PUT", "/api/groups", { groups: body });
    S.grp = r.view;
    S.grpEdit = { groups: cloneGroups(S.grp.groups) };
    S.grpWarn = r.warnings || [];
  }, t("grpSaved"));
  const needHash = groups.some((g) => g.mode === "balance" && g.members.length > 1) && v.hash_policy !== "1";

  const memberState = (ms, hh) => {
    if (!ms) return h("span", { class: "pill pill-idle" }, t("grpNotSaved"));
    if (!ms.up) return h("span", { class: "pill pill-idle" }, t("foState_down"));
    return hh ? h("span", {}, foStatePill(hh), hh.reason && h("div", { class: "sub" }, hh.reason)) : h("span", { class: "pill pill-idle" }, t("foState_starting"));
  };

  const groupCard = (g, gi) => {
    const st = statusOf(g.name);
    const free = (v.profiles || []).filter((n) => !g.members.some((m) => m.name === n));
    const move = (i, d) => { const o = g.members; [o[i], o[i + d]] = [o[i + d], o[i]]; dirty(); renderMain(); };
    const delKey = "delgrp:" + gi, armed = S.confirm === delKey;
    const bal = g.mode === "balance";
    const addMember = () => {
      const el = document.getElementById("grp-add-" + gi);
      if (!el || !el.value) return;
      g.members.push({ name: el.value, weight: 1 });
      dirty(); renderMain();
    };
    const row = (m, i) => {
      const ms = st && (st.members || []).find((x) => x.name === m.name);
      return h("tr", {},
        !bal && h("td", { class: "mono" }, String(i + 1)),
        h("td", {}, h("a", { class: "name", href: "#/p/" + encodeURIComponent(m.name) }, icon("server", 18), m.name)),
        bal && h("td", {}, h("input", { type: "number", min: "1", max: "100", class: "mono cell-input", "aria-label": t("grpColWeight") + " " + m.name, value: String(m.weight),
          oninput: (e) => { m.weight = parseInt(e.target.value, 10) || 1; dirty(); } })),
        h("td", {}, memberState(ms, health[m.name]), ms && ms.active && h("div", { class: "sub" }, h("span", { class: "pill pill-on" }, t("grpInRoute")))),
        h("td", {}, h("div", { class: "actions" },
          !bal && h("button", { class: "icon-btn", type: "button", title: t("foUp"), "aria-label": t("foUp"), disabled: i === 0, onclick: () => move(i, -1) }, icon("arrowUp", 16)),
          !bal && h("button", { class: "icon-btn", type: "button", title: t("foDown"), "aria-label": t("foDown"), disabled: i === g.members.length - 1, onclick: () => move(i, 1) }, icon("arrowDown", 16)),
          h("button", { class: "btn btn-secondary btn-small", type: "button", onclick: () => { g.members.splice(i, 1); dirty(); renderMain(); } }, t("grpRemove")))));
    };
    const cols = [!bal && "foColRank", "grpColMember", bal && "grpColWeight", "colState", "colActions"].filter(Boolean);
    return h("div", { class: "card", "data-group": g.name },
      h("div", { class: "heading-row" },
        h("h2", {}, icon("merge", 20), " ", g.name || "—"),
        h("div", { class: "actions" },
          st && st.primary && h("span", { class: "pill pill-warn", title: t("primaryHint") }, t("primary")),
          st && h("span", { class: "pill " + (st.active ? "pill-on" : "pill-off") }, t("grpActiveOf", { a: st.active, n: (st.members || []).length })),
          st && !st.primary && v.gateway && h("button", { class: "btn btn-secondary btn-small", type: "button", disabled: S.busy || S.dirty, onclick: () => doGroupPrimary(g.name) }, icon("star", 16), t("grpMakePrimary")),
          h("button", { class: "btn btn-danger btn-small", type: "button", disabled: S.busy, onclick: () => {
            if (!armed) { S.confirm = delKey; renderMain(); setTimeout(() => { if (S.confirm === delKey) { S.confirm = ""; renderMain(); } }, 5000); return; }
            S.confirm = ""; groups.splice(gi, 1); dirty(); renderMain();
          } }, icon("trash", 16), armed ? t("confirmDelete") : t("grpDeleteGroup")))),
      h("div", { class: "fields-grid" },
        h("div", { class: "field" }, h("label", { for: "grp-name-" + gi }, t("grpName")),
          h("input", { id: "grp-name-" + gi, type: "text", class: "mono", value: g.name, maxlength: "32", spellcheck: "false", autocomplete: "off",
            oninput: (e) => { g.name = e.target.value; dirty(); } })),
        h("div", { class: "field" }, h("label", { for: "grp-mode-" + gi }, t("grpMode")),
          h("select", { id: "grp-mode-" + gi, onchange: (e) => { g.mode = e.target.value; dirty(); renderMain(); } },
            ["balance", "ordered"].map((o) => h("option", { value: o, selected: g.mode === o }, t("grp_mode_" + o)))),
          h("div", { class: "hint" }, t("grpModeNote_" + g.mode)))),
      h("h3", {}, t("grpMembers")),
      g.members.length ? h("div", { class: "table-scroll" }, h("table", {},
        h("thead", {}, h("tr", {}, cols.map((k) => h("th", { scope: "col" }, t(k))))),
        h("tbody", {}, g.members.map(row)))) : h("p", { class: "info-line" }, icon("info", 16), h("span", {}, t("grpNoMembers"))),
      st && st.members && st.members.length > 0 && st.active === 0 && h("p", { class: "info-line" }, icon("alert", 16), h("span", {}, t("grpEmpty"))),
      st && st.route && h("p", { class: "info-line" }, icon("route", 16), h("span", {}, t("grpRoute") + ": ", h("span", { class: "mono" }, st.route), " · " + t("grpTable", { n: st.table }))),
      free.length ? h("div", { class: "actions add-row" },
        h("select", { id: "grp-add-" + gi, class: "cell-input", "aria-label": t("grpAddMember") }, free.map((n) => h("option", { value: n }, n))),
        h("button", { class: "btn btn-secondary btn-small", type: "button", onclick: addMember }, icon("plus", 16), t("grpAddBtn")))
        : h("p", { class: "hint" }, t("grpNoFree")));
  };

  const primary = v.primary || "";
  return h("div", {},
    heading(t("navInfra"), t("grpTitle"), t("grpSub"),
      h("button", { class: "btn btn-secondary", type: "button", disabled: S.busy, onclick: addGroup }, icon("plus", 16), t("grpAdd"))),
    !v.gateway && h("div", { class: "flash warn", role: "status" }, icon("info", 18), h("div", { class: "msg" }, t("grpNoGateway"))),
    needHash && h("div", { class: "flash warn", role: "status" }, icon("alert", 18), h("div", { class: "msg" }, t("grpHashWarn", { v: v.hash_policy || "?" }))),
    S.grpWarn && S.grpWarn.length > 0 && h("div", { class: "flash warn", role: "alert" }, icon("alert", 18), h("div", { class: "msg" }, t("grpSavedWarn") + ": " + S.grpWarn.join("; "))),
    h("div", { class: "grid-stats" },
      statCard(t("grpStatCount"), String((v.groups || []).length), t("grpStatCountNote")),
      statCard(t("primary"), primary || "—", t("primaryHint")),
      statCard(t("grpStatHash"), v.hash_policy === "1" ? t("grpHashPorts") : v.hash_policy ? t("grpHashAddr") : t("grpHashUnknown"), "fib_multipath_hash_policy = " + (v.hash_policy || "?"))),
    groups.length === 0 ? h("div", {},
      h("div", { class: "card empty" }, icon("merge", 32), h("strong", {}, t("grpNone")), h("p", {}, t("grpNoneHint")),
        h("button", { class: "btn btn-primary", type: "button", onclick: addGroup }, icon("plus", 16), t("grpAdd"))),
      (v.groups || []).length > 0 && h("div", { class: "actions" }, h("button", { class: "btn btn-primary", type: "button", disabled: S.busy, onclick: save }, icon("check", 16), t("foSave"))))
      : h("form", { onsubmit: (e) => { e.preventDefault(); save(); } },
        groups.map(groupCard),
        h("div", { class: "actions" }, h("button", { class: "btn btn-primary", type: "submit", disabled: S.busy }, icon("check", 16), t("foSave")))),
    (groups.length > 0 || (v.groups || []).length > 0) && h("div", { class: "card" }, h("h2", {}, t("grpUsage")), h("p", { class: "heading-sub" }, t("grpUsageText"))));
}

/* ---------- маршрутизация по правилам и DNS ---------- */
async function loadPolicy(quiet) {
  try {
    S.pol = await api("GET", "/api/policy");
    if (!S.polEdit || !S.dirty) S.polEdit = { ...S.pol.settings };
  } catch (e) {
    if (!quiet) { say("error", e.message); renderFlash(); }
  }
  if (S.route.page === "policy" && !S.dirty && !S.busy) renderMain();
}

function pagePolicy() {
  if (!S.pol || !S.polEdit) { loadPolicy(); return h("div", {}, heading(t("navInfra"), t("polTitle")), h("div", { class: "loading" }, t("loading"))); }
  const v = S.pol, st = v.stats || {}, ed = S.polEdit, g = v.geo || {};
  const dirty = () => { S.dirty = true; };
  const text = (id, key) => h("input", { id, type: "text", class: "mono", value: ed[key] || "", spellcheck: "false",
    oninput: (e) => { ed[key] = e.target.value; dirty(); } });
  const save = () => act(async () => {
    try {
      S.pol = await api("PUT", "/api/policy", { ...ed, geo_update_hours: parseInt(ed.geo_update_hours, 10) || 0 });
    } catch (e) {
      throw new Error(e.data && e.data.errors ? e.data.errors.join(" · ") : e.message);
    }
    S.polEdit = { ...S.pol.settings };
    if (S.pol.errors && S.pol.errors.length) throw new Error(t("polSavedWarn") + " " + S.pol.errors.join("; "));
  }, t("polSaved"));
  const updateGeo = () => act(async () => { S.pol = await api("POST", "/api/geo/update"); S.polEdit = { ...S.pol.settings }; }, t("polGeoUpdated"));
  const runTest = async () => {
    const q = (document.getElementById("pol-q") || {}).value || "";
    try { S.polTest = await api("POST", "/api/policy/test", { query: q }); } catch (e) { S.polTest = { error: e.message }; }
    renderMain();
  };
  const geoRow = (label, f, loaded) => h("tr", {},
    h("td", { class: "mono" }, label),
    h("td", {}, f && f.present ? h("span", { class: "pill pill-on" }, t("polGeoHave")) : h("span", { class: "pill pill-idle" }, t("polGeoNone"))),
    h("td", { class: "mono" }, f && f.present ? fmtBytes(f.size) : "—"),
    h("td", { class: "mono" }, f && f.present ? fmtDate(f.updated) : "—"),
    h("td", { class: "mono" }, Object.entries(loaded || {}).map(([k, n]) => k + " (" + n + ")").join(", ") || "—"));
  const actName = (a) => (a ? t("polAct_" + String(a).toLowerCase()) : "—");
  const tr = S.polTest;

  return h("div", {},
    heading(t("navInfra"), t("polTitle"), t("polSub")),
    !v.gateway && h("div", { class: "flash warn", role: "status" }, icon("info", 18), h("div", { class: "msg" }, t("polNoGateway"))),
    h("div", { class: "grid-stats" },
      statCard(t("polState"), st.enabled ? t("netOn") : t("netOff"), t("polDefault") + ": " + actName(st.default_action)),
      statCard(t("polRules"), String(st.rules || 0), t("polRoutes", { n: st.routes || 0 })),
      statCard(t("polLearned"), String(st.learned || 0), t("polLearnedNote")),
      statCard(t("polQueries"), String(st.queries || 0), v.dns ? t("polDnsNote", { p: st.proxy || 0, d: st.direct || 0, x: st.dropped || 0 }) : t("polDnsOff"))),
    st.last_error && h("div", { class: "flash error", role: "alert" }, icon("alert", 18), h("div", { class: "msg" }, st.last_error)),
    h("div", { class: "card" }, h("h2", {}, t("polRulesTitle")),
      h("form", { onsubmit: (e) => { e.preventDefault(); save(); } },
        h("label", { class: "check" }, h("input", { type: "checkbox", checked: !!ed.enabled, onchange: (e) => { ed.enabled = e.target.checked; dirty(); } }),
          h("span", {}, t("polEnable"), h("br"), t("polEnableHint"))),
        h("div", { class: "fields-grid" },
          h("div", { class: "field" }, h("label", { for: "pol-def" }, t("polDefault")),
            h("select", { id: "pol-def", onchange: (e) => { ed.default_action = e.target.value; dirty(); } },
              ["proxy", "direct"].map((o) => h("option", { value: o, selected: ed.default_action === o }, actName(o)))),
            h("div", { class: "hint" }, t("polDefaultHint"))),
          h("div", { class: "field" }, h("label", { for: "pol-upd" }, t("polGeoEvery")),
            h("input", { id: "pol-upd", type: "number", min: "0", max: "720", value: String(ed.geo_update_hours ?? 24), oninput: (e) => { ed.geo_update_hours = e.target.value; dirty(); } }),
            h("div", { class: "hint" }, t("polGeoEveryHint")))),
        h("div", { class: "field" }, h("label", { for: "pol-rules" }, t("polRulesLabel")),
          h("textarea", { id: "pol-rules", spellcheck: "false", placeholder: "PROXY geosite:youtube\nDIRECT geoip:ru\nPROXY@awg1 domain:example.com\nDROP keyword:ads\nDIRECT 192.168.0.0/16", oninput: (e) => { ed.rules = e.target.value; dirty(); } }, ed.rules || ""),
          h("div", { class: "hint" }, t("polRulesHint"))),
        h("div", { class: "fields-grid" },
          h("div", { class: "field" }, h("label", { for: "pol-pdns" }, t("polProxyDNS")), text("pol-pdns", "proxy_dns"), h("div", { class: "hint" }, t("polDnsHint"))),
          h("div", { class: "field" }, h("label", { for: "pol-ddns" }, t("polDirectDNS")), text("pol-ddns", "direct_dns")),
          h("div", { class: "field" }, h("label", { for: "pol-gs" }, "geosite.dat"), text("pol-gs", "geosite_url")),
          h("div", { class: "field" }, h("label", { for: "pol-gi" }, "geoip.dat"), text("pol-gi", "geoip_url"))),
        h("div", { class: "actions" }, h("button", { class: "btn btn-primary", type: "submit", disabled: S.busy }, icon("check", 16), t("foSave"))))),
    h("div", { class: "card" }, h("h2", {}, t("polTestTitle")),
      h("form", { onsubmit: (e) => { e.preventDefault(); runTest(); } },
        h("div", { class: "field" }, h("label", { for: "pol-q" }, t("polTestLabel")), h("input", { id: "pol-q", type: "text", class: "mono", placeholder: "www.youtube.com / 77.88.8.8", spellcheck: "false" })),
        h("div", { class: "actions" }, h("button", { class: "btn btn-secondary", type: "submit" }, icon("arrow", 16), t("polTestBtn")))),
      tr && (tr.error ? h("p", { class: "info-line" }, icon("alert", 16), h("span", {}, tr.error))
        : h("p", { class: "info-line" }, icon("info", 16), h("span", {}, h("b", {}, tr.query), " → ", h("b", {}, actName(tr.action) + (tr.target ? " (" + tr.target + ")" : "")), " · " + t("polBy") + " " + tr.matched_by)))),
    h("div", { class: "card" }, h("h2", {}, t("polGeoTitle")),
      h("div", { class: "table-scroll" }, h("table", {},
        h("thead", {}, h("tr", {}, ["polGeoFile", "colState", "polGeoSize", "polGeoDate", "polGeoLoaded"].map((k) => h("th", { scope: "col" }, t(k))))),
        h("tbody", {}, geoRow("geosite.dat", g.geosite, g.site_loaded), geoRow("geoip.dat", g.geoip, g.ip_loaded)))),
      g.missing && g.missing.length > 0 && h("p", { class: "info-line" }, icon("alert", 16), h("span", {}, t("polGeoMissing") + ": " + g.missing.join(", "))),
      g.last_error && h("p", { class: "info-line" }, icon("alert", 16), h("span", {}, g.last_error)),
      h("div", { class: "actions" }, h("button", { class: "btn btn-secondary", type: "button", disabled: S.busy, onclick: updateGeo }, icon("restart", 16), t("polGeoUpdate")))));
}

/* ---------- интеграция с RouterOS ---------- */
async function loadRouter(quiet) {
  try {
    S.ros = await api("GET", "/api/router");
    if (!S.rosEdit || !S.dirty) S.rosEdit = { ...S.ros.settings, pass: "" };
  } catch (e) {
    if (!quiet) { say("error", e.message); renderFlash(); }
  }
  if (S.route.page === "router" && !S.dirty && !S.busy) renderMain();
}

function rosScript(ed, st) {
  const cip = (st && st.container_ip) || ed.forward_to || "192.168.253.2";
  const lines = [
    "# " + t("rosScrUser"),
    "/user/group/add name=awg31 policy=read,write,rest-api",
    "/user/add name=" + (ed.user || "awg31") + " group=awg31 password=<" + t("rosScrPass") + ">",
    "# " + t("rosScrRest"),
    "/certificate/add name=awg31-www common-name=router",
    "/certificate/sign awg31-www",
    "/ip/service/set www-ssl certificate=awg31-www disabled=no address=" + cip + "/32",
  ];
  if (ed.mode === "bgp") {
    lines.push("# " + t("rosScrBgp"),
      "/routing/filter/rule/add chain=awg31-in rule=\"if (dst-len > 0 && dst-len <= 32) { accept } else { reject }\"",
      "/routing/bgp/connection/add name=awg31 as=" + (ed.bgp_remote_as || 65030) + " remote.address=" + cip + " remote.as=" + (ed.bgp_local_as || 65031) +
      " local.role=ebgp input.filter=awg31-in");
  }
  return lines.join("\n");
}

function pageRouter() {
  if (!S.ros || !S.rosEdit) { loadRouter(); return h("div", {}, heading(t("navSystem"), t("rosTitle")), h("div", { class: "loading" }, t("loading"))); }
  const v = S.ros, st = v.status || {}, ed = S.rosEdit, bgp = st.bgp || {};
  const dirty = () => { S.dirty = true; };
  const field = (id, key, label, opts = {}) => h("div", { class: "field" }, h("label", { for: id }, label),
    h("input", { id, type: opts.type || "text", class: opts.mono === false ? "" : "mono", value: ed[key] == null ? "" : String(ed[key]), placeholder: opts.ph || "",
      autocomplete: opts.type === "password" ? "new-password" : "off", spellcheck: "false",
      oninput: (e) => { ed[key] = opts.num ? (parseInt(e.target.value, 10) || 0) : e.target.value; dirty(); } }),
    opts.hint && h("div", { class: "hint" }, opts.hint));
  const body = () => { const b = { ...ed }; delete b.pass_set; return b; };
  const save = () => act(async () => {
    try { S.ros = await api("PUT", "/api/router", body()); } catch (e) { throw new Error(e.data && e.data.errors ? e.data.errors.join(" · ") : e.message); }
    S.rosEdit = { ...S.ros.settings, pass: "" };
  }, t("rosSaved"));
  const test = () => act(async () => {
    try { const r = await api("POST", "/api/router/test", body()); return r.router; } catch (e) { throw new Error(e.data && e.data.errors ? e.data.errors.join(" · ") : e.message); }
  }, (who) => t("rosTestOk", { who }));
  const syncNow = () => act(async () => { S.ros = await api("POST", "/api/router/sync"); }, t("rosSynced"));
  const modeName = (m) => t("rosMode_" + m);
  const bgpPill = { established: "pill-on", error: "pill-off", connecting: "pill-warn" }[bgp.state] || "pill-idle";

  return h("div", {},
    heading(t("navSystem"), t("rosTitle"), t("rosSub"),
      h("button", { class: "btn btn-secondary", type: "button", disabled: S.busy || ed.mode === "off", onclick: syncNow }, icon("restart", 16), t("rosSyncNow"))),
    !v.gateway && h("div", { class: "flash warn", role: "status" }, icon("info", 18), h("div", { class: "msg" }, t("rosNoGateway"))),
    h("div", { class: "grid-stats" },
      statCard(t("rosModeLabel"), modeName(st.mode || ed.mode), st.router || t("rosNoRouter")),
      statCard(t("rosStatic"), String(st.static || 0), st.dropped ? t("rosDropped", { n: st.dropped }) : t("rosStaticNote")),
      statCard(t("rosDynamic"), String(st.dynamic || 0), t("rosDynamicNote")),
      statCard(t("rosDomains"), String(st.domains || 0), st.forward_to ? t("rosFwdTo", { ip: st.forward_to }) : t("rosDnsOff"))),
    st.last_error && h("div", { class: "flash error", role: "alert" }, icon("alert", 18), h("div", { class: "msg" }, st.last_error)),
    ed.mode === "bgp" && h("div", { class: "card" }, h("h2", {}, "BGP"), kv([
      [t("colState"), h("span", { class: "pill " + bgpPill }, t("rosBgp_" + (bgp.state || "idle")))],
      [t("rosAnnounced"), String(bgp.announced || 0)],
      [t("rosBgpErr"), bgp.last_error || "—"]])),
    h("div", { class: "card" }, h("h2", {}, t("rosSettings")),
      h("form", { onsubmit: (e) => { e.preventDefault(); save(); } },
        h("div", { class: "fields-grid" },
          h("div", { class: "field" }, h("label", { for: "ros-mode" }, t("rosModeLabel")),
            h("select", { id: "ros-mode", onchange: (e) => { ed.mode = e.target.value; dirty(); renderMain(); } },
              ["off", "address_list", "bgp"].map((m) => h("option", { value: m, selected: ed.mode === m }, modeName(m)))),
            h("div", { class: "hint" }, t("rosModeHint_" + ed.mode))),
          field("ros-url", "url", t("rosURL"), { ph: "https://192.168.253.1", hint: t("rosURLHint") }),
          field("ros-user", "user", t("rosUser")),
          field("ros-pass", "pass", t("rosPass"), { type: "password", ph: v.settings.pass_set ? t("rosPassKept") : "", hint: t("rosPassHint") }),
          field("ros-pin", "pin_sha256", t("rosPin"), { ph: "sha256//…", hint: t("rosPinHint") }),
          field("ros-list", "address_list", t("rosList"), { hint: t("rosListHint") }),
          field("ros-fwd", "forward_to", t("rosFwd"), { ph: "192.168.253.2", hint: t("rosFwdHint") }),
          field("ros-lim", "static_limit", t("rosLimit"), { type: "number", num: true, mono: false }),
          field("ros-dyn", "dynamic_minutes", t("rosDynMin"), { type: "number", num: true, mono: false }),
          field("ros-dlim", "domain_limit", t("rosDomLimit"), { type: "number", num: true, mono: false }),
          ed.mode === "bgp" && field("ros-las", "bgp_local_as", t("rosLocalAS"), { type: "number", num: true, mono: false }),
          ed.mode === "bgp" && field("ros-ras", "bgp_remote_as", t("rosRemoteAS"), { type: "number", num: true, mono: false }),
          ed.mode === "bgp" && field("ros-peer", "bgp_peer", t("rosPeer"), { ph: "192.168.253.1", hint: t("rosPeerHint") })),
        h("label", { class: "check" }, h("input", { type: "checkbox", checked: !!ed.sync_dns, onchange: (e) => { ed.sync_dns = e.target.checked; dirty(); } }),
          h("span", {}, t("rosSyncDNS"), h("br"), t("rosSyncDNSHint"))),
        h("label", { class: "check" }, h("input", { type: "checkbox", checked: !!ed.insecure, onchange: (e) => { ed.insecure = e.target.checked; dirty(); } }),
          h("span", {}, t("rosInsecure"), h("br"), t("rosInsecureHint"))),
        h("div", { class: "actions" },
          h("button", { class: "btn btn-primary", type: "submit", disabled: S.busy }, icon("check", 16), t("foSave")),
          h("button", { class: "btn btn-secondary", type: "button", disabled: S.busy || ed.mode === "off", onclick: test }, icon("activity", 16), t("rosTest"))))),
    h("div", { class: "card" }, h("h2", {}, t("rosOnRouter")), h("p", { class: "heading-sub" }, t("rosOnRouterSub")),
      h("pre", { class: "code" }, rosScript(ed, st))));
}

/* ---------- вход и первичная настройка ---------- */
function authView(mode) {
  const setup = mode === "setup";
  const token = h("input", { id: "au-token", type: "text", autocomplete: "off", spellcheck: "false", class: "mono" });
  const user = h("input", { id: "au-user", type: "text", autocomplete: "username", value: "admin" });
  const pw = h("input", { id: "au-pw", type: "password", autocomplete: setup ? "new-password" : "current-password" });
  const rep = h("input", { id: "au-rep", type: "password", autocomplete: "new-password" });
  const err = h("div", { class: "flash error", role: "alert", hidden: true });
  const fail = (text) => { err.hidden = false; err.replaceChildren(icon("alert", 18), h("div", { class: "msg" }, text)); };
  const submit = async (e) => {
    e.preventDefault();
    err.hidden = true;
    if (setup && pw.value !== rep.value) { fail(t("passwordsDiffer")); return; }
    try {
      if (setup) await api("POST", "/api/auth/setup", { setup_token: token.value.trim(), user: user.value.trim(), password: pw.value });
      else await api("POST", "/api/auth/login", { user: user.value.trim(), password: pw.value });
      pw.value = rep.value = "";
      await boot();
    } catch (ex) { fail(ex.message); pw.value = ""; pw.focus(); }
  };
  return h("div", { class: "auth-wrap" },
    h("section", { class: "auth-hero" }, h("div", { class: "eyebrow" }, t("console")), h("h1", {}, setup ? t("setupTitle") : t("loginTitle")), h("p", {}, t("loginLead")),
      h("ul", {}, [["server", "f1"], ["link", "f2"], ["shield", "f3"]].map(([i, k]) => h("li", {}, icon(i, 18), t(k))))),
    h("section", { class: "auth-card" }, h("div", { class: "mark" }, icon("lock", 26)),
      h("div", { class: "eyebrow" }, setup ? t("setupEyebrow") : t("accessEyebrow")),
      h("h2", {}, setup ? t("setupTitle") : t("accessTitle")), h("p", { class: "lead" }, setup ? t("setupLead") : t("accessLead")),
      h("form", { onsubmit: submit },
        err,
        setup && h("div", { class: "field" }, h("label", { for: "au-token" }, t("setupToken")), token),
        h("div", { class: "field" }, h("label", { for: "au-user" }, t("user")), user),
        h("div", { class: "field" }, h("label", { for: "au-pw" }, setup ? t("passwordNew") : t("password")), pw),
        setup && h("div", { class: "field" }, h("label", { for: "au-rep" }, t("passwordRepeat")), rep),
        h("button", { class: "btn btn-primary btn-block", type: "submit" }, setup ? t("createAdmin") : t("signIn"), icon("arrow", 16)))));
}

/* ---------- запуск ---------- */
render();
boot();
setInterval(() => {
  if (S.auth && S.auth.authenticated && document.visibilityState === "visible" && !S.busy) {
    refresh(true);
    if (S.route && S.route.page === "failover") loadFailover(true);
    if (S.route && S.route.page === "groups") loadGroups(true);
    if (S.route && S.route.page === "policy") loadPolicy(true);
    if (S.route && S.route.page === "router") loadRouter(true);
  }
}, 5000);

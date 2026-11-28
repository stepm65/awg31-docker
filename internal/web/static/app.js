"use strict";
// Весь интерфейс строится через DOM API и textContent: данные сервера никогда не
// попадают в разметку как HTML, поэтому XSS через имена профилей, ошибки или
// ссылки невозможен. Единственный разбор разметки — иконки из констант ниже.
// Вид и тексты повторяют AWG Console (amneziawg31-installer): RU/EN, тёмная тема.

/* ---------- иконки (Lucide-подобные, viewBox 24, как в панели) ---------- */
const ICONS = {
  route: '<path d="M6 19a2 2 0 1 0 0-4 2 2 0 0 0 0 4zM18 9a2 2 0 1 0 0-4 2 2 0 0 0 0 4z"/><path d="M6 15V9a4 4 0 0 1 4-4h6M18 9v6a4 4 0 0 1-4 4H8"/>',
  arrowUp: '<path d="M12 19V5m-6 6 6-6 6 6"/>',
  dev_pc: '<rect x="3" y="4" width="18" height="12" rx="2"/><path d="M8 20h8M12 16v4"/>',
  dev_laptop: '<rect x="5" y="5" width="14" height="10" rx="1.5"/><path d="M3 19h18"/>',
  dev_phone: '<rect x="7" y="3" width="10" height="18" rx="2"/><path d="M11 17h2"/>',
  dev_tablet: '<rect x="5" y="3" width="14" height="18" rx="2"/><path d="M11 17h2"/>',
  dev_tv: '<rect x="3" y="6" width="18" height="12" rx="2"/><path d="m9 2 3 3 3-3"/>',
  dev_console: '<rect x="2.5" y="7" width="19" height="10" rx="5"/><path d="M7 12h3M8.5 10.5v3M15 11h.01M17 13h.01"/>',
  dev_speaker: '<rect x="6" y="3" width="12" height="18" rx="2"/><circle cx="12" cy="14" r="3"/><path d="M12 7h.01"/>',
  dev_nas: '<rect x="3" y="4" width="18" height="7" rx="1.5"/><rect x="3" y="13" width="18" height="7" rx="1.5"/><path d="M7 7.5h.01M7 16.5h.01"/>',
  dev_printer: '<path d="M7 9V3h10v6"/><rect x="3" y="9" width="18" height="8" rx="2"/><path d="M7 14h10v7H7z"/>',
  dev_camera: '<path d="M4 8h3l2-3h6l2 3h3v11H4z"/><circle cx="12" cy="13" r="3.5"/>',
  dev_router: '<rect x="3" y="13" width="18" height="7" rx="2"/><path d="M7 16.5h.01M11 16.5h.01M17 13V8M14.5 6a3.5 3.5 0 0 1 5 0"/>',
  dev_wifi: '<path d="M2 8.5a15 15 0 0 1 20 0M5 12a10 10 0 0 1 14 0M8.5 15.5a5 5 0 0 1 7 0"/><path d="M12 19h.01"/>',
  dev_iot: '<rect x="7" y="7" width="10" height="10" rx="1.5"/><path d="M10 3v4M14 3v4M10 17v4M14 17v4M3 10h4M3 14h4M17 10h4M17 14h4"/>',
  dev_other: '<circle cx="12" cy="12" r="8"/><circle cx="12" cy="12" r="1.5"/>',
  devices: '<rect x="2" y="4" width="14" height="10" rx="1.5"/><path d="M6 18h6M9 14v4"/><rect x="16" y="9" width="6" height="11" rx="1.5"/>',
  wrench: '<path d="M15 4a5 5 0 0 0-4.6 7L4 17.4V20h2.6l6.4-6.4A5 5 0 0 0 20 9l-3 3-3-1-1-3z"/>',
  download: '<path d="M12 4v11m-5-5 5 5 5-5M5 20h14"/>',
  upload: '<path d="M12 16V5m-5 5 5-5 5 5M5 20h14"/>',
  search: '<circle cx="11" cy="11" r="6.5"/><path d="m16 16 4.5 4.5"/>',
  filter: '<path d="M3 5h18l-7 8v6l-4 2v-8z"/>',
  arrowDown: '<path d="M12 5v14m-6-6 6 6 6-6"/>',
  star: '<path d="m12 3 2.7 5.6 6.1.9-4.4 4.3 1 6.1L12 17l-5.4 2.9 1-6.1L3.2 9.5l6.1-.9z"/>',
  more: '<circle cx="5" cy="12" r="1.3"/><circle cx="12" cy="12" r="1.3"/><circle cx="19" cy="12" r="1.3"/>',
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
  logs: '<path d="M4 6h16M4 12h16M4 18h10"/>',
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
  grip: '<circle cx="9" cy="6" r="1.4"/><circle cx="15" cy="6" r="1.4"/><circle cx="9" cy="12" r="1.4"/><circle cx="15" cy="12" r="1.4"/><circle cx="9" cy="18" r="1.4"/><circle cx="15" cy="18" r="1.4"/>',
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
    up: "Включить", down: "Выключить", running: "Запущен", stopped: "Остановлен", never: "не было",
    ago_s: "{n} с назад", ago_m: "{n} мин назад", ago_h: "{n} ч назад", ago_d: "{n} дн назад",
    loading: "Загрузка…", yes: "да", no: "нет", back: "К списку", confirmDelete: "Подтвердить удаление",
    version: "Версия", language: "Язык", close: "Закрыть",
    navInfra: "ИНФРАСТРУКТУРА", navProfiles: "ПРОФИЛИ", navSystem: "СИСТЕМА", navDevices: "Устройства", navMonitor: "Мониторинг", navTools: "Инструменты", navLog: "Журнал",
    logTitle: "Журнал", logSub: "Последние сообщения контейнера: запуск, туннели, переключения, ошибки. Хранятся в памяти до перезапуска контейнера (до 3000 строк).",
    logAll: "Все", logWarn: "Предупреждения", logErr: "Ошибки", logSearch: "Поиск по журналу", logFollow: "Следить за новыми", logDownload: "Скачать", logEmpty: "Записей нет.", logNoMatch: "Нет записей под фильтр.", logShown: "Показано {n} из {all}", logLevel_info: "инфо", logLevel_warn: "внимание", logLevel_error: "ошибка", logTime: "Время", logLevel: "Уровень", logText: "Сообщение",
    toolTitle: "Инструменты", toolSub: "Базы geodata, проверка маршрута, перенос правил и резервная копия.",
    toolGeoTitle: "Геоданные", toolGeoSources: "Источники", toolLoadedCats: "в правилах: {n}",
    toolExtraTitle: "Дополнительные наборы", toolExtraSub: "Любой .dat (например geo-slim или russia-blocked) под своим тегом. Не больше 8 наборов.",
    toolExtraEmpty: "Наборов нет.", toolExtraAdd: "Добавить набор", toolExtraSave: "Сохранить и скачать", toolExtraSaved: "Наборы сохранены и скачаны.",
    toolExtraHint: "В правилах категория пишется с тегом: {ex}:{tag}:категория.", toolKind: "Тип", toolTag: "тег", toolCats: "{n} кат.", toolNotLoaded: "не скачан",
    toolTestTitle: "Тест маршрута", toolDevice: "Устройство", toolAnyDevice: "Любое устройство", toolPort: "Порт",
    toolByRule: "по правилу «{name}» ({by})",
    toolRulesTitle: "Правила маршрутизации", toolRulesSub: "Текст правил файлом: для переноса на другой роутер. Импорт открывает правила в редакторе, сохраняете вы.",
    toolExport: "Экспорт правил", toolImport: "Импорт правил", toolImported: "Правила из файла загружены в редактор. Проверьте и сохраните.", toolTooBig: "Файл больше 64 КБ.",
    toolBackupTitle: "Резервная копия", toolBackupWarn: "В копии закрытые ключи профилей, ссылки подписок и пароль роутера. Храните её как пароль.",
    toolBackupGet: "Скачать копию", toolRestoreTitle: "Восстановление", toolRestoreSub: "Заменяет профили и все настройки копией. Логин и пароль администратора остаются текущими. Туннели перезапускаются.",
    toolRestoreFile: "Файл копии (.json)", toolRestore: "Восстановить", toolRestored: "Копия восстановлена.", toolPickFile: "Выберите файл копии.", toolNotBackup: "Это не файл резервной копии.", ovTunnels: "Туннели и группы", ovRules: "Правила маршрутизации", ovAllRules: "Все правила", ovAllDevices: "Все устройства",
    ovNoRules: "По правилам сейчас нет соединений.", ovConnsHint: "Соединений клиентов через туннель", ovBalance: "балансировка", ovOrdered: "по порядку", ssWorking: "работает", ssIdle: "нет туннелей", ssCpu: "Процессор", ssMem: "Память контейнера", ssConns: "Соединений клиентов", ssUptime: "Время работы",
    monTitle: "Мониторинг", monSub: "Трафик, устройства, правила и туннели по соединениям клиентов.",
    monTraffic: "Трафик", monWin15: "15 минут", monWin60: "1 час", monPeak: "пик", monDown: "загрузка", monUp: "отдача",
    monVpn: "через VPN", monNoData: "Замеров пока нет: первые точки появятся через несколько секунд.",
    monTopo: "Топология соединений", monDevices: "Устройства", monRules: "Правила", monOuts: "Выход",
    monConns: "соединений", monByConns: "По соединениям", monByBytes: "По объёму", monSearch: "Устройство, домен, правило или туннель",
    monDefault: "по умолчанию", monDirect: "Напрямую", monDrop: "Блок", monVpnOut: "VPN", monMore: "ещё {n}",
    monStatDev: "устройств", monStatRules: "правил", monStatOuts: "выходов", monStatConns: "соединений",
    monNoConntrack: "Таблица соединений (conntrack) недоступна: ядро не отдаёт /proc/net/nf_conntrack, и утилиты conntrack нет. Трафик по-прежнему считается.",
    monNoFlows: "Соединений клиентов сейчас нет. Здесь видно только то, что проходит через контейнер.",
    monConnTitle: "Соединения", monColDev: "Устройство", monColDst: "Назначение", monColRule: "Правило", monColOut: "Выход", monColBytes: "Объём", monColN: "Соед.", monShown: "Показаны {n} крупнейших из {m}: уточните поиск.",
    monSelHint: "Нажмите на устройство, правило или выход, чтобы подсветить его путь. Повторное нажатие снимает выбор.",
    monNoBytes: "Учёт объёма в conntrack выключен, видно только число соединений.",
    devTitle: "Устройства", devSub: "Имена и адреса устройств LAN: для правил по устройствам и мониторинга.",
    devListTitle: "Список устройств", devName: "Имя", devAddr: "IP или сеть", devKind: "Тип", devAdd: "Добавить устройство",
    devFromDhcp: "Из DHCP роутера", devSave: "Сохранить", devSaved: "Устройства сохранены.", devRemove: "Удалить {name}",
    devEmpty: "Устройств пока нет. Добавьте их вручную или из аренд DHCP роутера.",
    devHint: "Адрес закрепите на роутере (статическая аренда DHCP): правило по устройству действует, пока у него этот IP. Сеть, например 192.168.1.64/28, объединяет несколько устройств.",
    devLeasesTitle: "Аренды DHCP роутера", devLeasesHint: "Отметьте устройства и нажмите «Добавить выбранные». Уже добавленные адреса скрыты.",
    devLeasesAdd: "Добавить выбранные", devLeasesNone: "Новых аренд нет: все адреса уже в списке.", devLeasesStatic: "статическая",
    devUsed: "в правилах: {n}",
    devKind_pc: "Компьютер", devKind_laptop: "Ноутбук", devKind_phone: "Телефон", devKind_tablet: "Планшет", devKind_tv: "Телевизор",
    devKind_console: "Приставка", devKind_speaker: "Колонка", devKind_nas: "NAS", devKind_printer: "Принтер", devKind_camera: "Камера",
    devKind_router: "Роутер", devKind_wifi: "Точка доступа", devKind_iot: "Умный дом", devKind_other: "Другое",
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
    overviewTitle: "Обзор", statTunnels: "Туннели", statTunnelsNote: "включено / всего", statRx: "Получено", statTx: "Отправлено",
    statHandshake: "Рукопожатие", statHandshakeNote: "последнее по всем пирам", statSumNote: "сумма по всем туннелям", profilesTitle: "Профили AWG",
    manageAll: "Все профили", manage: "Управление", rx: "Получение", tx: "Отправка", handshake: "Рукопожатие",
    noProfiles: "Профилей пока нет", noProfilesHint: "Добавьте .conf или подключите подписку вашей панели.", addProfile: "Добавить профиль",
    tunnelError: "Не удалось получить состояние туннелей: {err}",
    statusTitle: "Состояние системы", level_ok: "В норме", level_warn: "Внимание", level_crit: "Сбой", level_info: "Нет данных",
    sumOk: "Все запущенные туннели работают", sumOkDetail: "Запущено {up} из {total}, у каждого свежее рукопожатие.",
    sumEmpty: "Профилей пока нет", sumEmptyDetail: "Добавьте профиль, и здесь появится состояние туннелей.",
    sumNoneUp: "Ни один туннель не запущен", sumNoneUpDetail: "Профилей: {total}. Включите нужный туннель в списке профилей.",
    sumWait: "Нет свежего рукопожатия", sumWaitDetail: "Ждут рукопожатия: {names}. Свежим считается рукопожатие не старше {min} мин.",
    sumErr: "Есть профили с ошибкой", sumErrDetail: "Проверьте: {names}.", sumSubErr: "Подписка не обновилась",
    sumTunnelErr: "Не удалось получить состояние туннелей", sumNetErr: "Ошибка маршрутизации",
    sumLost: "Нет связи с демоном", sumLostDetail: "Данные не обновлялись {s} с. Повторяю каждые 5 с.",
    stWaiting: "Ждёт рукопожатия", stError: "Ошибка", rateNow: "сейчас {v}", perSec: "/с",
    fltLabel: "Фильтр по состоянию", fltAll: "Все", fltLive: "Работают", fltWait: "Ждут рукопожатия", fltStopped: "Остановлены", fltBad: "С ошибкой",
    noMatch: "В этом фильтре профилей нет", showAll: "Показать все", shownOf: "Показано {n} из {m}", moreActions: "Ещё действия",
    // профили
    profilesHeading: "Профили", profilesSub: "Конфигурации AmneziaWG 3.1 и их подписки.",
    colName: "Имя", colAddress: "Адрес", colState: "Состояние", colTraffic: "Трафик", colSub: "Подписка", colActions: "Действия",
    subOn: "каждые {n}", subOff: "вручную", subNone: "нет", autostart: "Автозапуск", profileMissing: "Профиль не найден",
    tabOverview: "Обзор", tabConfig: "Конфигурация", tabSub: "Подписка",
    profileEyebrow: "ПРОФИЛИ / {name}", profileTitle: "Профиль {name}", noConfigYet: "Конфигурации ещё нет: она придёт по подписке или вставьте .conf.",
    peerEndpoint: "Endpoint", peerHandshake: "Рукопожатие", peerTraffic: "Трафик", peerAllowed: "AllowedIPs", peerKey: "Ключ пира",
    cfgAddresses: "Адреса", cfgEndpoints: "Endpoints", cfgObf: "Обфускация", cfgPsk: "PresharedKey", cfgPeers: "Пиров",
    on: "включена", off: "нет", present: "есть", absent: "нет", autostartInfo: "После перезапуска контейнера туннель {state}.",
    autostartYes: "включится сам", autostartNo: "останется выключенным",
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
    upDone: "Туннель {name} включён.", downDone: "Туннель {name} выключен.", deleted: "Профиль {name} удалён.",
    // обслуживание
    maintTitle: "Обслуживание", maintSub: "Пароль, версия и состояние защиты.", pwTitle: "Пароль администратора",
    pwCurrent: "Текущий пароль", pwNew: "Новый пароль", pwRepeat: "Повторите новый пароль", pwChange: "Сменить пароль",
    pwChanged: "Пароль изменён, остальные сессии завершены.", navFailover: "Резервирование", primary: "Основной", primaryHint: "Туннель, в который сейчас идёт трафик LAN", makePrimary: "Сделать основным",
    primaryDone: "Трафик LAN переведён в туннель {name}.",
    foTitle: "Резервирование", foSub: "Несколько туннелей одновременно и автоматическое переключение при сбое.",
    foCheckNow: "Проверить сейчас", foChecked: "Проверка выполнена.", foSaved: "Настройки резервирования сохранены.",
    foNoGateway: "Резервирование работает только в режиме шлюза: включите AWG_AUTO_ROUTE и проверьте LAN-интерфейс на странице «Обслуживание».",
    foMode: "Режим", fo_mode_off: "Вручную", fo_mode_standby: "Горячий резерв", fo_mode_switch: "Переключение",
    foModeNote_off: "Туннели включаются и выбираются основным вручную.",
    foModeNote_standby: "Все туннели из списка включены; при сбое трафик мгновенно уходит в следующий.",
    foModeNote_switch: "Включён только основной; при сбое он выключается и включается следующий.",
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
    foState_cooldown: "Пауза", foState_down: "Не включён", foState_excluded: "Не участвует",
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
    // редактор маршрутов
    rtAdd: "Добавить сервис", rtSearch: "Поиск по правилам", rtFltAll: "Все", rtFltProxy: "Через VPN", rtFltDirect: "Напрямую", rtFltDrop: "Блок", rtFltOff: "Выключены", rtFltDevs: "С устройствами", rtFltExtra: "С портами", rtHits: "Соединений по этому правилу сейчас: {n}",
    rtColDevices: "Устройства", rtAllDevices: "Все устройства", rtNDevices: "{n} устр.", rtDevicesOf: "Устройства правила «{name}»",
    rtScopeTitle: "Применить к устройствам", rtScopeAll: "Все устройства", rtScopeSearch: "Поиск устройства", rtScopeChosen: "Выбрано: {n}",
    rtScopeReset: "сбросить", rtScopeNoDevices: "Устройств пока нет: добавьте их на странице «Устройства».", rtScopeGoDevices: "Открыть «Устройства»",
    rtPorts: "Порты назначения", rtPortsHint: "Например 443 или 27000-27100 через запятую. Пусто — любые.", rtNet: "Протокол",
    rtNetAny: "Любой", rtNetTcp: "TCP", rtNetUdp: "UDP", rtNetBoth: "TCP и UDP", rtPortsBad: "Порты: число 1–65535 или диапазон, например 8000-8100",
    rtScopeMixed: "У условий этого правила разные ограничения. После «Готово» у всех будет одно.",
    rtScopeNote: "Правило с устройствами или портами проверяется раньше общих: для этих устройств оно главнее.",
    rtScopeEmpty: "Выключите «Все устройства» и выберите хотя бы одно устройство.",
    rtColService: "Сервис", rtColConds: "Условия", rtColDest: "Куда", rtColOn: "Вкл.",
    rtDestProxy: "Через VPN", rtTunnels: "Туннели", rtGroups: "Группы", rtDestDirect: "Напрямую", rtDestDrop: "Блокировать", rtMixed: "Разные", rtGone: "такого нет",
    rtDestOf: "Куда направить: {name}", rtToggleOn: "Включить правило: {name}", rtToggleOff: "Выключить правило: {name}", rtStateOn: "включено", rtStateOff: "выключено",
    rtGrip: "Переместить: {name}. Позиция {n} из {m}. Стрелки вверх и вниз меняют порядок.", rtGripLocked: "Порядок нельзя менять, пока включён поиск или фильтр",
    rtMoved: "{name}: позиция {n} из {m}", rtEdit: "Изменить", rtDup: "Дублировать", rtUp: "Выше", rtDown: "Ниже", rtCopy: "{name} (копия)",
    rtMore: "ещё {n}", rtCustom: "Свои правила ({n})", rtEmpty: "Без условий", rtNoCond: "нет условий",
    rtNoRules: "Правил пока нет", rtNoRulesHint: "Добавьте сервис: например, YouTube через VPN, а российские адреса напрямую.",
    rtNoMatch: "Ничего не найдено", rtNoMatchHint: "Измените поиск или фильтр.",
    rtOrderHint: "Порядок важен: для доменов побеждает первое совпавшее правило (сверху вниз), для адресов — самая точная сеть. Правила с устройствами или портами проверяются раньше общих. Всё остальное идёт по правилу «Остальной трафик».",
    rtBadLines: "Движок не понимает строк: {n}. Пока они есть, правила не сохранятся: исправьте их в тексте или удалите.", rtBadShow: "Показать текст", rtBadDrop: "Удалить эти строки",
    rtOffCond: "выключено", rtMissing: "категории нет в загруженных базах",
    rtUnsaved: "Есть несохранённые изменения", rtRevert: "Отменить", rtReverted: "Изменения отменены.",
    rtAdv: "Текст правил", rtAdvShow: "показать", rtAdvHide: "скрыть", rtAdvHint: "То же самое в виде текста. «#@ Имя | значок» — заголовок блока, «#~ ПРАВИЛО» — выключенная строка; движок видит их как комментарии.",
    rtSettings: "Списки и DNS", rtPresetLabel: "Источник списков", rtPresetLoyal: "Loyalsoldier (по умолчанию)", rtPresetIplist: "IPList (opencck.org)", rtPresetRunet: "runetfreedom (блокировки в РФ)",
    rtPresetHint: "Подставляет ссылки на geosite.dat и geoip.dat. После сохранения нажмите «Обновить базы». У IPList свои имена тегов: например geoip:telegram.org вместо geoip:telegram, каталог сервисов подставляет их сам. IPList распространяется под лицензией MIT. runetfreedom: теги как у Loyalsoldier плюс geosite:ru-blocked и geoip:ru-blocked (заблокированное в РФ, обновляется каждые 6 часов), geosite:ru-available-only-inside; база большая (~110 МБ на диске), берите диск, а не встроенную память. Лицензия GPL-3.0.",
    svcRu: "Россия (IP)", svcPrivate: "Локальные сети", rtDlgAdd: "Добавить сервис", rtDlgEdit: "Изменить: {name}", rtCatalog: "Каталог сервисов", rtCatalogHint: "Выбор подставляет название, значок и условия.",
    rtCatalogSearch: "Найти в каталоге", rtRetagToIpl: "В правилах {n} усл. записаны тегами Loyalsoldier (например geoip:telegram), у IPList они называются иначе.", rtRetagToLoyal: "В правилах {n} усл. записаны тегами IPList (например geoip:telegram.org), у Loyalsoldier они называются иначе.", rtRetagBtn: "Заменить теги", rtRetagDone: "Заменено условий: {n}. Сохраните правила.",
    rtSec_groups: "Группы", rtSec_sites: "Сайты", rtSec_svc: "Сервисы", rtSec_cats: "Категории", rtSec_countries: "Страны и сети",
    rtCatMore: "Показаны первые {m}, ещё {n}: уточните поиск.", rtCatNone: "Ничего не найдено.",
    rtCatalogFull: "Все категории загруженной базы ({n}). Выбор подставляет geosite и, если у категории есть адреса, geoip. Начните вводить название, чтобы найти нужную.",
    rtCatalogIplist: "Список из загруженной базы IPList: группа — набор сайтов (ai, messengers), сайт — один портал. Выбор подставляет geosite и geoip.",
    rtCatalogNoDb: "Полный список появится, когда загружена база выбранного источника: сохраните настройки и нажмите «Обновить базы».", rtName: "Название", rtConds: "Условия", rtKind: "Тип условия", rtKindAuto: "Автоопределение",
    rtKindDomain: "Домен и поддомены", rtKindFull: "Только это имя", rtKindKeyword: "Часть имени", rtKindCidr: "Сеть или IP",
    rtCondValue: "Домены, сети или категории", rtCondAdd: "Добавить", rtCondRemove: "Убрать условие {v}", rtCondNone: "Условий пока нет",
    rtCondNeed: "Добавьте хотя бы одно условие.", rtCondBad: "Не понял: {v}", rtCondDup: "уже есть: {v}",
    rtCondHint: "Можно вставить несколько записей через пробел, запятую или с новой строки. Подсказки для geosite и geoip появляются, когда базы загружены.",
    rtDest: "Куда направить трафик", rtDone: "Готово", rtCancel: "Отмена", rtAdded: "Добавлено: {name}",
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
    up: "Turn on", down: "Turn off", running: "Running", stopped: "Stopped", never: "never",
    ago_s: "{n}s ago", ago_m: "{n}m ago", ago_h: "{n}h ago", ago_d: "{n}d ago",
    loading: "Loading…", yes: "yes", no: "no", back: "Back to list", confirmDelete: "Confirm deletion",
    version: "Version", language: "Language", close: "Close",
    navInfra: "INFRASTRUCTURE", navProfiles: "PROFILES", navSystem: "SYSTEM", navDevices: "Devices", navMonitor: "Monitoring", navTools: "Tools", navLog: "Log",
    logTitle: "Log", logSub: "Recent container messages: startup, tunnels, failover, errors. Kept in memory until the container restarts (up to 3000 lines).",
    logAll: "All", logWarn: "Warnings", logErr: "Errors", logSearch: "Search the log", logFollow: "Follow new lines", logDownload: "Download", logEmpty: "No entries.", logNoMatch: "No entries match the filter.", logShown: "Showing {n} of {all}", logLevel_info: "info", logLevel_warn: "warning", logLevel_error: "error", logTime: "Time", logLevel: "Level", logText: "Message",
    toolTitle: "Tools", toolSub: "Geodata, route test, moving rules and backup.",
    toolGeoTitle: "Geodata", toolGeoSources: "Sources", toolLoadedCats: "in rules: {n}",
    toolExtraTitle: "Extra sets", toolExtraSub: "Any .dat (for example geo-slim or russia-blocked) under its own tag. Up to 8 sets.",
    toolExtraEmpty: "No sets.", toolExtraAdd: "Add set", toolExtraSave: "Save and download", toolExtraSaved: "Sets saved and downloaded.",
    toolExtraHint: "In rules write the category with the tag: {ex}:{tag}:category.", toolKind: "Type", toolTag: "tag", toolCats: "{n} cat.", toolNotLoaded: "not downloaded",
    toolTestTitle: "Route test", toolDevice: "Device", toolAnyDevice: "Any device", toolPort: "Port",
    toolByRule: "by rule \"{name}\" ({by})",
    toolRulesTitle: "Routing rules", toolRulesSub: "The rules text as a file: to move it to another router. Import opens the rules in the editor; you save them.",
    toolExport: "Export rules", toolImport: "Import rules", toolImported: "Rules from the file are loaded into the editor. Check and save them.", toolTooBig: "The file is larger than 64 KB.",
    toolBackupTitle: "Backup", toolBackupWarn: "The backup holds profile private keys, subscription links and the router password. Keep it like a password.",
    toolBackupGet: "Download backup", toolRestoreTitle: "Restore", toolRestoreSub: "Replaces profiles and all settings with the backup. The administrator login and password stay as they are. Tunnels restart.",
    toolRestoreFile: "Backup file (.json)", toolRestore: "Restore", toolRestored: "Backup restored.", toolPickFile: "Choose a backup file.", toolNotBackup: "This is not a backup file.", ovTunnels: "Tunnels and groups", ovRules: "Routing rules", ovAllRules: "All rules", ovAllDevices: "All devices",
    ovNoRules: "No connections by rules right now.", ovConnsHint: "Client connections through the tunnel", ovBalance: "balance", ovOrdered: "ordered", ssWorking: "running", ssIdle: "no tunnels", ssCpu: "CPU", ssMem: "Container memory", ssConns: "Client connections", ssUptime: "Uptime",
    monTitle: "Monitoring", monSub: "Traffic, devices, rules and tunnels by client connections.",
    monTraffic: "Traffic", monWin15: "15 minutes", monWin60: "1 hour", monPeak: "peak", monDown: "download", monUp: "upload",
    monVpn: "via VPN", monNoData: "No samples yet: the first points appear in a few seconds.",
    monTopo: "Connection topology", monDevices: "Devices", monRules: "Rules", monOuts: "Outbound",
    monConns: "connections", monByConns: "By connections", monByBytes: "By volume", monSearch: "Device, domain, rule or tunnel",
    monDefault: "default", monDirect: "Direct", monDrop: "Blocked", monVpnOut: "VPN", monMore: "{n} more",
    monStatDev: "devices", monStatRules: "rules", monStatOuts: "outbounds", monStatConns: "connections",
    monNoConntrack: "The connection table (conntrack) is not available: the kernel has no /proc/net/nf_conntrack and there is no conntrack tool. Traffic is still counted.",
    monNoFlows: "No client connections right now. Only traffic that goes through the container is visible here.",
    monConnTitle: "Connections", monColDev: "Device", monColDst: "Destination", monColRule: "Rule", monColOut: "Outbound", monColBytes: "Volume", monColN: "Conn.", monShown: "Showing the {n} largest of {m}: refine the search.",
    monSelHint: "Click a device, rule or outbound to highlight its path. Click again to clear.",
    monNoBytes: "Byte accounting in conntrack is off, only connection counts are shown.",
    devTitle: "Devices", devSub: "Names and addresses of LAN devices: for per-device rules and monitoring.",
    devListTitle: "Device list", devName: "Name", devAddr: "IP or network", devKind: "Type", devAdd: "Add device",
    devFromDhcp: "From router DHCP", devSave: "Save", devSaved: "Devices saved.", devRemove: "Remove {name}",
    devEmpty: "No devices yet. Add them by hand or from the router's DHCP leases.",
    devHint: "Pin the address on the router (static DHCP lease): a per-device rule works while the device has this IP. A network such as 192.168.1.64/28 groups several devices.",
    devLeasesTitle: "Router DHCP leases", devLeasesHint: "Tick devices and press \"Add selected\". Addresses already in the list are hidden.",
    devLeasesAdd: "Add selected", devLeasesNone: "No new leases: every address is already in the list.", devLeasesStatic: "static",
    devUsed: "in rules: {n}",
    devKind_pc: "Computer", devKind_laptop: "Laptop", devKind_phone: "Phone", devKind_tablet: "Tablet", devKind_tv: "TV",
    devKind_console: "Console", devKind_speaker: "Speaker", devKind_nas: "NAS", devKind_printer: "Printer", devKind_camera: "Camera",
    devKind_router: "Router", devKind_wifi: "Access point", devKind_iot: "Smart home", devKind_other: "Other",
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
    statusTitle: "System status", level_ok: "Healthy", level_warn: "Attention", level_crit: "Failure", level_info: "No data",
    sumOk: "All running tunnels are working", sumOkDetail: "{up} of {total} running, each with a fresh handshake.",
    sumEmpty: "No profiles yet", sumEmptyDetail: "Add a profile and the tunnel state will appear here.",
    sumNoneUp: "No tunnel is running", sumNoneUpDetail: "Profiles: {total}. Turn on the one you need from the profile list.",
    sumWait: "No fresh handshake", sumWaitDetail: "Waiting for a handshake: {names}. A handshake counts as fresh for {min} min.",
    sumErr: "Some profiles have an error", sumErrDetail: "Check: {names}.", sumSubErr: "A subscription failed to update",
    sumTunnelErr: "Could not read the tunnel state", sumNetErr: "Routing error",
    sumLost: "No connection to the daemon", sumLostDetail: "No update for {s} s. Retrying every 5 s.",
    stWaiting: "Waiting for handshake", stError: "Error", rateNow: "now {v}", perSec: "/s",
    fltLabel: "Filter by state", fltAll: "All", fltLive: "Working", fltWait: "Waiting for handshake", fltStopped: "Stopped", fltBad: "With errors",
    noMatch: "No profiles match this filter", showAll: "Show all", shownOf: "Showing {n} of {m}", moreActions: "More actions",
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
    // routing editor
    rtAdd: "Add service", rtSearch: "Search rules", rtFltAll: "All", rtFltProxy: "Via VPN", rtFltDirect: "Direct", rtFltDrop: "Blocked", rtFltOff: "Off", rtFltDevs: "With devices", rtFltExtra: "With ports", rtHits: "Connections by this rule right now: {n}",
    rtColDevices: "Devices", rtAllDevices: "All devices", rtNDevices: "{n} dev.", rtDevicesOf: "Devices of rule \"{name}\"",
    rtScopeTitle: "Apply to devices", rtScopeAll: "All devices", rtScopeSearch: "Find a device", rtScopeChosen: "Selected: {n}",
    rtScopeReset: "reset", rtScopeNoDevices: "No devices yet: add them on the \"Devices\" page.", rtScopeGoDevices: "Open \"Devices\"",
    rtPorts: "Destination ports", rtPortsHint: "For example 443 or 27000-27100, comma-separated. Empty — any.", rtNet: "Protocol",
    rtNetAny: "Any", rtNetTcp: "TCP", rtNetUdp: "UDP", rtNetBoth: "TCP and UDP", rtPortsBad: "Ports: a number 1–65535 or a range such as 8000-8100",
    rtScopeMixed: "The conditions of this rule have different limits. After \"Done\" they all get one.",
    rtScopeNote: "A rule with devices or ports is checked before general rules: for these devices it wins.",
    rtScopeEmpty: "Turn off \"All devices\" and pick at least one device.",
    rtColService: "Service", rtColConds: "Conditions", rtColDest: "Goes to", rtColOn: "On",
    rtDestProxy: "Via VPN", rtTunnels: "Tunnels", rtGroups: "Groups", rtDestDirect: "Direct", rtDestDrop: "Block", rtMixed: "Mixed", rtGone: "not found",
    rtDestOf: "Send to: {name}", rtToggleOn: "Turn on rule: {name}", rtToggleOff: "Turn off rule: {name}", rtStateOn: "on", rtStateOff: "off",
    rtGrip: "Move: {name}. Position {n} of {m}. Up and down arrows change the order.", rtGripLocked: "The order cannot change while a search or filter is active",
    rtMoved: "{name}: position {n} of {m}", rtEdit: "Edit", rtDup: "Duplicate", rtUp: "Move up", rtDown: "Move down", rtCopy: "{name} (copy)",
    rtMore: "{n} more", rtCustom: "Custom rules ({n})", rtEmpty: "No conditions", rtNoCond: "no conditions",
    rtNoRules: "No rules yet", rtNoRulesHint: "Add a service: for example YouTube through the VPN and Russian addresses direct.",
    rtNoMatch: "Nothing found", rtNoMatchHint: "Change the search or the filter.",
    rtOrderHint: "Order matters: for domains the first matching rule wins (top to bottom), for addresses the most specific network. Rules with devices or ports are checked before general ones. Everything else follows the \"Everything else\" setting.",
    rtBadLines: "The engine does not understand {n} lines. The rules will not save while they are there: fix them in the text or delete them.", rtBadShow: "Show the text", rtBadDrop: "Delete these lines",
    rtOffCond: "off", rtMissing: "category is not in the loaded databases",
    rtUnsaved: "You have unsaved changes", rtRevert: "Discard", rtReverted: "Changes discarded.",
    rtAdv: "Rules text", rtAdvShow: "show", rtAdvHide: "hide", rtAdvHint: "The same rules as text. \"#@ Name | icon\" is a block header and \"#~ RULE\" a turned-off line; the engine sees both as comments.",
    rtSettings: "Lists and DNS", rtPresetLabel: "List source", rtPresetLoyal: "Loyalsoldier (default)", rtPresetIplist: "IPList (opencck.org)", rtPresetRunet: "runetfreedom (blocked in Russia)",
    rtPresetHint: "Fills in the geosite.dat and geoip.dat links. After saving, press \"Update databases\". IPList names its tags differently, for example geoip:telegram.org instead of geoip:telegram; the service catalog fills them in for you. IPList is released under the MIT license. runetfreedom uses the Loyalsoldier tags plus geosite:ru-blocked and geoip:ru-blocked (blocked in Russia, updated every 6 hours) and geosite:ru-available-only-inside; the databases are large (~110 MB on disk), so use a disk rather than internal flash. GPL-3.0 license.",
    svcRu: "Russia (IP)", svcPrivate: "Local networks", rtDlgAdd: "Add service", rtDlgEdit: "Edit: {name}", rtCatalog: "Service catalog", rtCatalogHint: "Picking one fills in the name, icon and conditions.",
    rtCatalogSearch: "Search the catalog", rtRetagToIpl: "{n} conditions use Loyalsoldier tags (e.g. geoip:telegram); IPList names them differently.", rtRetagToLoyal: "{n} conditions use IPList tags (e.g. geoip:telegram.org); Loyalsoldier names them differently.", rtRetagBtn: "Replace tags", rtRetagDone: "Conditions replaced: {n}. Save the rules.",
    rtSec_groups: "Groups", rtSec_sites: "Sites", rtSec_svc: "Services", rtSec_cats: "Categories", rtSec_countries: "Countries and networks",
    rtCatMore: "Showing the first {m}, {n} more: refine the search.", rtCatNone: "Nothing found.",
    rtCatalogFull: "All categories of the downloaded database ({n}). Picking one fills in geosite and, if the category has addresses, geoip. Start typing a name to find one.",
    rtCatalogIplist: "List from the downloaded IPList database: a group is a set of sites (ai, messengers), a site is one portal. Picking one fills in geosite and geoip.",
    rtCatalogNoDb: "The full list appears once the database of the selected source is downloaded: save the settings and press \"Update databases\".", rtName: "Name", rtConds: "Conditions", rtKind: "Condition type", rtKindAuto: "Auto-detect",
    rtKindDomain: "Domain and subdomains", rtKindFull: "This exact name", rtKindKeyword: "Part of a name", rtKindCidr: "Network or IP",
    rtCondValue: "Domains, networks or categories", rtCondAdd: "Add", rtCondRemove: "Remove condition {v}", rtCondNone: "No conditions yet",
    rtCondNeed: "Add at least one condition.", rtCondBad: "Not understood: {v}", rtCondDup: "already there: {v}",
    rtCondHint: "You can paste several entries separated by spaces, commas or new lines. Suggestions for geosite and geoip appear once the databases are loaded.",
    rtDest: "Where to send the traffic", rtDone: "Done", rtCancel: "Cancel", rtAdded: "Added: {name}",
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

// Причины от сервера приходят по-русски; в английском интерфейсе известные переводим, остальное (тексты ошибок) оставляем как есть.
const FO_REASONS_EN = [
  [/не в списке резервирования/g, "not in the failover list"],
  [/нет рукопожатия/g, "no handshake"],
  [/рукопожатие (\d+) с назад/g, "handshake $1 s ago"],
  [/ждём первое рукопожатие/g, "waiting for the first handshake"],
  [/поднят резервом/g, "brought up as standby"],
  [/не поднялся: /g, "failed to come up: "],
  [/^проба /g, "probe "],
  [/ неисправен: /g, " is unhealthy: "],
];
const foReason = (r) => (lang === "en" ? FO_REASONS_EN.reduce((acc, [re, to]) => acc.replace(re, to), String(r || "")) : r);
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
  return (i ? n.toFixed(1) : String(Math.round(n))) + " " + u[i];
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
  missed: 0,         // подряд неудачных опросов /api/status
  okAt: 0,           // когда данные обновлялись в последний раз
  hist: [],          // выборки трафика для мини-графиков на плитках (только в этой вкладке)
  pfFilter: "all",   // фильтр списка профилей
};
const root = document.getElementById("app");
let sideEl = null, mainEl = null;
// Один постоянный регион для озвучивания: его текст меняется только при смене смысла,
// а не при каждой перерисовке страницы по таймеру.
const live = h("div", { class: "sr-only", role: "status", "aria-live": "polite" });
document.body.append(live);
function announce(text) { if (live.textContent !== text) live.textContent = text; }

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
  if (parts[0] === "devices") return { page: "devices" };
  if (parts[0] === "monitor") return { page: "monitor" };
  if (parts[0] === "tools") return { page: "tools" };
  if (parts[0] === "log") return { page: "log" };
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
  if (S.route.page === "devices") { S.devEdit = null; S.leases = null; loadDevices(true); }
  if (S.route.page === "monitor") { loadMonitor(true); loadFlows(true); }
  if (S.route.page === "tools") { S.toolTest = null; S.extraEdit = null; loadPolicy(true); loadDevices(true); }
  if (S.route.page === "log") loadLog(true);
  if (S.auth && S.auth.authenticated) { renderSide(); renderMain(); }
});
document.addEventListener("input", (e) => { if (e.target.closest && e.target.closest("#main form") && !e.target.closest("[data-nodirty]")) S.dirty = true; });
document.addEventListener("click", (e) => {
  for (const d of document.querySelectorAll(".row-menu[open]")) if (!d.contains(e.target)) d.open = false;
});
document.addEventListener("keydown", (e) => {
  if (e.key !== "Escape") return;
  const d = document.querySelector(".row-menu[open]");
  if (d) { d.open = false; const sm = d.querySelector("summary"); if (sm) sm.focus(); }
});

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
    S.missed = 0; S.okAt = Date.now(); sample();
  } catch (e) {
    if (e.status === 401) { S.auth = { configured: true, authenticated: false }; render(); return; }
    S.missed++;
    if (!quiet) say("error", e.message);
  }
  // Открытое меню строки не закрываем перерисовкой по таймеру.
  if (quiet) { renderSide(); if (!S.dirty && !S.busy && !document.querySelector(".row-menu[open]") && !rtTyping() && S.route.page !== "log") renderMain(); } else { renderSide(); renderMain(); }
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
    link("monitor", "activity", t("navMonitor"), "/monitor"),
    link("devices", "devices", t("navDevices"), "/devices"),
    h("div", { class: "side-title side-group" }, h("span", {}, t("navProfiles")), h("span", { class: "badge" }, String(list.length))),
    list.map((p) => h("a", { class: "side-inst", href: "#/p/" + encodeURIComponent(p.name), "aria-current": r.page === "profile" && r.name === p.name ? "page" : null, title: p.name },
      sideDot(p),
      h("span", { class: "names" }, h("span", { class: "n" }, p.name), h("span", { class: "a" }, addrOf(p))))),
    h("div", { class: "side-title" }, h("span", {}, t("navSystem"))),
    link("router", "link", t("navRouter"), "/router"),
    link("tools", "wrench", t("navTools"), "/tools"),
    link("log", "logs", t("navLog"), "/log"),
    link("maintenance", "settings", t("navMaint"), "/maintenance"),
    h("div", { class: "side-status", id: "side-status" }),
    h("div", { class: "side-foot" }, icon("tag", 16), h("span", { class: "ver" }, "v" + ((S.data && S.data.version) || "…"))),
  ].flat(Infinity));
  renderSideStatus();
}

// Блок состояния внизу меню: основной туннель, скорость, процессор, память, соединения, время работы.
function renderSideStatus() {
  const box = document.getElementById("side-status");
  if (!box) return;
  const m = S.mon, d = S.data || {};
  if (!m) { box.replaceChildren(); return; }
  const sys = m.system || {}, pts = (m.traffic || []).slice(-60), last = pts[pts.length - 1] || { down: 0, up: 0 };
  const prim = d.primary || ((d.profiles || []).find((p) => p.primary) || {}).name || "";
  const up = (d.profiles || []).some((p) => p.up);
  const dur = (s2) => { s2 = Math.max(0, s2 | 0); const dd = Math.floor(s2 / 86400), hh = Math.floor(s2 % 86400 / 3600), mm = Math.floor(s2 % 3600 / 60); return dd ? dd + (lang === "en" ? "d " : "д ") + hh + (lang === "en" ? "h" : "ч") : hh ? hh + (lang === "en" ? "h " : "ч ") + mm + (lang === "en" ? "m" : "м") : mm + (lang === "en" ? " min" : " мин"); };
  box.replaceChildren(
    h("div", { class: "ss-head" }, icon("activity", 14), h("b", {}, "AWG"), h("span", { class: "ss-state" + (up ? " on" : "") }, up ? t("ssWorking") : t("ssIdle"))),
    prim ? h("div", { class: "ss-prim" }, icon("shield", 14), h("span", {}, prim)) : null,
    sparkline(pts.map((p) => p.down)),
    h("div", { class: "ss-rates" }, h("span", { class: "d" }, "↓ " + fmtRate(last.down)), h("span", { class: "u" }, "↑ " + fmtRate(last.up))),
    h("div", { class: "ss-grid" },
      h("span", { title: t("ssCpu") }, icon("server", 13), (sys.cpu || 0).toFixed(1) + "%"),
      h("span", { title: t("ssMem") }, icon("layers", 13), fmtBytes(sys.mem_group || sys.mem_rss || 0)),
      h("span", { title: t("ssConns") }, icon("link", 13), String(sys.conns || 0)),
      h("span", { title: t("ssUptime") }, icon("clock", 13), dur(sys.uptime || 0))));
}

function renderMain() {
  if (!mainEl) return;
  const r = S.route;
  const page = r.page === "profiles" ? pageProfiles() : r.page === "profile" ? pageProfile(r)
    : r.page === "new" ? pageNew() : r.page === "maintenance" ? pageMaintenance()
    : r.page === "failover" ? pageFailover() : r.page === "groups" ? pageGroups() : r.page === "policy" ? pagePolicy() : r.page === "router" ? pageRouter() : r.page === "devices" ? pageDevices() : r.page === "monitor" ? pageMonitor() : r.page === "tools" ? pageTools() : r.page === "log" ? pageLog() : pageOverview();
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
/* ---------- состояние туннелей ---------- */
// Рукопожатие моложе этого срока (с) считаем свежим. WireGuard обновляет сессию примерно раз в 2 минуты
// при трафике; запас до 3 минут — рабочая граница интерфейса, а не требование протокола.
const HS_FRESH = 180;
const hsAge = (p) => { const a = agg(p); return a.hs ? Math.max(0, Math.floor(Date.now() / 1000 - a.hs)) : -1; };
function liveness(p) {
  if (!p.up) return "stopped";
  const age = hsAge(p);
  return age >= 0 && age <= HS_FRESH ? "live" : "wait";
}
const hasProblem = (p) => !!(p.error || (p.subscription && p.subscription.last_error));
function tunnelState(p) {
  if (p.error) return { key: "error", cls: "pill-off", label: t("stError") };
  const k = liveness(p);
  return k === "live" ? { key: k, cls: "pill-on", label: t("running") }
    : k === "wait" ? { key: k, cls: "pill-warn", label: t("stWaiting") } : { key: k, cls: "pill-idle", label: t("stopped") };
}
function pill(p) { const s = tunnelState(p); return h("span", { class: "pill " + s.cls }, s.label); }
function sideDot(p) {
  const s = tunnelState(p);
  return [h("span", { class: "dot" + ({ live: " on", wait: " warn", error: " crit" }[s.key] || "") }), h("span", { class: "sr-only" }, s.label)];
}
function fleet(list) {
  const c = { all: list.length, live: 0, wait: 0, stopped: 0, bad: 0 };
  for (const p of list) { c[liveness(p)]++; if (hasProblem(p)) c.bad++; }
  return c;
}
const nameList = (ps) => ps.slice(0, 3).map((p) => p.name).join(", ") + (ps.length > 3 ? " +" + (ps.length - 3) : "");

// Одна фраза о состоянии: уровень, заголовок, пояснение. Порядок — от тяжёлого к лёгкому.
function summary(list, c) {
  const d = S.data || {}, net = d.network || {}, up = c.live + c.wait;
  if (S.missed >= 3) return { level: "crit", icon: "alert", head: t("sumLost"), detail: t("sumLostDetail", { s: Math.max(1, Math.round((Date.now() - S.okAt) / 1000)) }) };
  if (d.tunnel_error) return { level: "crit", icon: "alert", head: t("sumTunnelErr"), detail: d.tunnel_error };
  const broken = list.filter((p) => p.error), subErr = list.filter((p) => !p.error && hasProblem(p));
  if (broken.length) return { level: "crit", icon: "alert", head: t("sumErr"), detail: t("sumErrDetail", { names: nameList(broken) }) };
  if (net.error) return { level: "crit", icon: "alert", head: t("sumNetErr"), detail: net.error };
  if (!list.length) return { level: "info", icon: "info", head: t("sumEmpty"), detail: t("sumEmptyDetail") };
  if (!up) return { level: "warn", icon: "alert", head: t("sumNoneUp"), detail: t("sumNoneUpDetail", { total: list.length }) };
  if (subErr.length) return { level: "warn", icon: "alert", head: t("sumSubErr"), detail: t("sumErrDetail", { names: nameList(subErr) }) };
  if (c.wait) return { level: "warn", icon: "alert", head: t("sumWait"), detail: t("sumWaitDetail", { names: nameList(list.filter((p) => liveness(p) === "wait")), min: HS_FRESH / 60 }) };
  return { level: "ok", icon: "shield", head: t("sumOk"), detail: t("sumOkDetail", { up, total: list.length }) };
}

// Выборки суммарного трафика: по ним считается скорость для мини-графиков плиток.
function sample() {
  let rx = 0, tx = 0;
  for (const p of profiles()) { const a = agg(p); rx += a.rx; tx += a.tx; }
  const now = Date.now(), last = S.hist[S.hist.length - 1];
  if (last && now - last.t < 1500) return; // refresh() зовут и таймер, и действия
  let drx = 0, dtx = 0;
  if (last) { // счётчики сбрасываются, когда туннель опускают: отрицательный прирост считаем нулём
    const dt = (now - last.t) / 1000;
    drx = rx >= last.rx ? (rx - last.rx) / dt : 0; dtx = tx >= last.tx ? (tx - last.tx) / dt : 0;
  }
  S.hist.push({ t: now, rx, tx, drx, dtx });
  if (S.hist.length > 60) S.hist.shift();
}
const SVGNS = "http://www.w3.org/2000/svg";
function sparkline(vals) {
  if (!vals || vals.length < 2) return null;
  const W = 100, H = 40, max = Math.max(...vals, 1), n = vals.length;
  const line = vals.map((v, i) => (i ? "L" : "M") + ((i / (n - 1)) * W).toFixed(1) + " " + (H - 2 - (v / max) * (H * 0.3)).toFixed(1)).join(" ");
  const svg = document.createElementNS(SVGNS, "svg");
  svg.setAttribute("class", "stat-chart"); svg.setAttribute("viewBox", "0 0 " + W + " " + H);
  svg.setAttribute("preserveAspectRatio", "none"); svg.setAttribute("aria-hidden", "true");
  for (const [cls, d] of [["area", line + " L" + W + " " + H + " L0 " + H + " Z"], ["line", line]]) {
    const path = document.createElementNS(SVGNS, "path"); path.setAttribute("class", cls); path.setAttribute("d", d); svg.append(path);
  }
  return svg;
}
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
function statCard(label, value, note, o) {
  o = o || {};
  return h("div", { class: "stat" + (o.tone ? " " + o.tone : ""), "data-level": o.level || null },
    o.series && sparkline(o.series),
    h("div", { class: "stat-label" }, label), h("div", { class: "stat-value" }, value), h("div", { class: "stat-note" }, note));
}

function statusStrip(sum) {
  const d = S.data || {}, n = d.network || {};
  return h("section", { class: "status-strip", "data-level": sum.level, "aria-label": t("statusTitle") },
    h("span", { class: "status-mark" }, icon(sum.icon, 22)),
    h("div", { class: "status-body" },
      h("div", { class: "status-head" }, h("span", {}, sum.head), h("span", { class: "status-level" }, t("level_" + sum.level))),
      h("div", { class: "status-detail" }, sum.detail)),
    h("div", { class: "status-facts" },
      d.primary && h("span", { class: "fact" }, t("primary") + ":", h("b", {}, d.primary)),
      n.gateway && h("span", { class: "fact" }, t("netKill") + ":", h("b", {}, n.kill_switch ? t("netOn") : t("netOff")))));
}

function pageOverview() {
  const list = profiles(), c = fleet(list), up = c.live + c.wait;
  let rx = 0, tx = 0, hs = 0;
  for (const p of list) { const a = agg(p); rx += a.rx; tx += a.tx; hs = Math.max(hs, a.hs); }
  const sum = summary(list, c);
  announce(t("level_" + sum.level) + ": " + sum.head);
  const hist = S.hist, rate = (k) => (hist.length > 1 ? t("rateNow", { v: fmtRate(hist[hist.length - 1][k]) }) : t("statSumNote"));
  const hsFresh = hs && Math.floor(Date.now() / 1000 - hs) <= HS_FRESH;
  const stale = S.missed >= 3; // данные устарели: плитки без цветового уровня, чтобы не показывать «норму» по старым цифрам
  return h("div", {},
    heading(t("console"), t("overviewTitle")),
    statusStrip(sum),
    h("div", { class: "grid-stats" },
      statCard(t("statTunnels"), up + " / " + list.length, t("statTunnelsNote"), { level: stale ? null : sum.level }),
      statCard(t("statRx"), fmtBytes(rx), rate("drx"), { tone: "rx", series: hist.slice(1).map((x) => x.drx) }),
      statCard(t("statTx"), fmtBytes(tx), rate("dtx"), { tone: "tx", series: hist.slice(1).map((x) => x.dtx) }),
      statCard(t("statHandshake"), hs ? fmtAgo(hs) : "—", t("statHandshakeNote"), { level: up && !stale ? (hsFresh ? "ok" : "warn") : null })),
    list.length ? overviewCards(list) : null,
    h("div", { class: "section-title" }, h("h2", {}, t("profilesTitle")),
      h("a", { href: "#/profiles" }, t("manageAll") + " →")),
    list.length ? h("div", { class: "inst-grid" }, list.map(profileCard)) : emptyState());
}

// «Обзор»: трафик, туннели и группы, правила и устройства по соединениям.
function overviewCards(list) {
  if (!S.mon) loadMonitor(true);
  if (!S.flows) loadFlows(true);
  if (!S.grp) loadGroups(true);
  const f = S.flows, m = S.mon;
  const outConns = {};
  for (const o of (f && f.outs) || []) outConns[o.id] = o.conns;
  const more = (href, label) => h("a", { class: "card-link", href }, label + " →");
  const tunRow = (p) => {
    const a = agg(p), s2 = tunnelState(p);
    return h("a", { class: "ov-row" + (p.primary ? " is-primary" : ""), href: "#/p/" + encodeURIComponent(p.name) },
      h("span", { class: "dot" + ({ live: " on", wait: " warn", error: " crit" }[s2.key] || "") }),
      h("span", { class: "ov-name" }, p.name, p.primary ? h("span", { class: "pill pill-warn" }, t("primary")) : null),
      h("span", { class: "ov-chip", title: t("ovConnsHint") }, icon("link", 13), outConns["out:" + p.name] != null ? String(outConns["out:" + p.name]) : "—"),
      h("span", { class: "ov-chip", title: t("handshake") }, icon("clock", 13), a.hs ? fmtAgo(a.hs) : "—"));
  };
  const groups = (S.grp && S.grp.groups) || [];
  const grpRow = (g) => h("a", { class: "ov-row", href: "#/groups" },
    h("span", { class: "ov-ic" }, icon("merge", 15)),
    h("span", { class: "ov-name" }, g.name, h("span", { class: "muted" }, " · " + g.members.length)),
    h("span", { class: "ov-chip" }, icon("link", 13), outConns["out:" + g.name] != null ? String(outConns["out:" + g.name]) : "—"),
    h("span", { class: "ov-chip" }, t(g.mode === "balance" ? "ovBalance" : "ovOrdered")));
  // правила: самые нагруженные и куда идут
  const ruleOut = {};
  for (const p of (f && f.paths) || []) { const r = ruleOut[p.rule] || (ruleOut[p.rule] = {}); r[p.out] = (r[p.out] || 0) + p.conns; }
  const outLabel = (id) => { const o = ((f && f.outs) || []).find((x) => x.id === id) || {}; return o.kind === "direct" ? [t("monDirect"), "direct"] : o.kind === "drop" ? [t("monDrop"), "drop"] : [o.name === "vpn" ? t("monVpnOut") : (o.name || "—"), "proxy"]; };
  const rules = ((f && f.rules) || []).filter((r) => r.id !== "rule:default").slice(0, 6);
  const devs = ((f && f.devices) || []).slice(0, 6);
  return [
    h("div", { class: "ov-grid" },
      h("div", { class: "card" }, h("div", { class: "heading-row" }, h("h2", {}, icon("activity", 18), " ", t("monTraffic")), more("#/monitor", t("navMonitor"))),
        m ? [trafficHead(m.traffic.slice(-180)), trafficChart(m.traffic.slice(-180), { compact: true })] : h("div", { class: "loading" }, t("loading"))),
      h("div", { class: "card" }, h("div", { class: "heading-row" }, h("h2", {}, t("ovTunnels")), more("#/profiles", t("navAll"))),
        h("div", { class: "ov-list" }, groups.map(grpRow), list.slice().sort((a, b) => (b.primary ? 1 : 0) - (a.primary ? 1 : 0) || (b.up ? 1 : 0) - (a.up ? 1 : 0)).slice(0, 8).map(tunRow)))),
    f && f.source !== "none" && h("div", { class: "ov-grid" },
      h("div", { class: "card" }, h("div", { class: "heading-row" }, h("h2", {}, t("ovRules")), more("#/policy", t("ovAllRules"))),
        rules.length ? h("div", { class: "ov-list" }, rules.map((r) => {
          const info = monNodeInfo(r), outs = Object.entries(ruleOut[r.id] || {}).sort((a, b) => b[1] - a[1]), [ol, oc] = outs.length ? outLabel(outs[0][0]) : ["—", ""];
          return h("a", { class: "ov-row ov-rule", href: "#/policy" }, info.tile || h("span", { class: "ov-ic" }, info.ico),
            h("span", { class: "ov-name" }, info.label, h("span", { class: "sub mono" }, r.sub || "")),
            h("span", { class: "rt-hits" }, String(r.conns)),
            h("span", { class: "ov-dest d-" + oc }, ol));
        })) : h("p", { class: "hint" }, t("ovNoRules"))),
      h("div", { class: "card" }, h("div", { class: "heading-row" }, h("h2", {}, t("navDevices")), more("#/devices", t("ovAllDevices"))),
        devs.length ? h("div", { class: "ov-list" }, devs.map((d) => h("a", { class: "ov-row", href: "#/monitor", onclick: () => { S.monSel = d.id; } },
          h("span", { class: "ov-ic" }, devIcon(d.kind, 15)), h("span", { class: "ov-name" }, d.name, h("span", { class: "sub mono" }, d.sub || "")),
          h("span", { class: "rt-hits" }, String(d.conns)), f.bytes ? h("span", { class: "ov-chip" }, fmtBytes(d.bytes)) : null))) : h("p", { class: "hint" }, t("monNoFlows")))),
  ];
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

// Меню строки: «⋯» открывает второстепенные действия. Удаление требует второго нажатия.
function rowMenu(p) {
  const net = (S.data && S.data.network) || {};
  let armed = false, timer = 0;
  const delLabel = h("span", {}, t("delete"));
  const del = h("button", { type: "button", class: "is-danger", disabled: S.busy, onclick: () => {
    if (!armed) { armed = true; delLabel.textContent = t("confirmDelete"); timer = setTimeout(() => { armed = false; delLabel.textContent = t("delete"); }, 5000); return; }
    clearTimeout(timer);
    act(async () => { await api("DELETE", "/api/profiles/" + encodeURIComponent(p.name)); go("/profiles"); }, t("deleted", { name: p.name }));
  } }, icon("trash", 16), delLabel);
  const canPrimary = p.up && !p.primary && net.gateway && net.lan_iface;
  return h("details", { class: "row-menu" },
    h("summary", { class: "icon-btn", title: t("moreActions"), "aria-label": t("moreActions") + ": " + p.name }, icon("more", 18)),
    h("div", { class: "row-menu-list" },
      h("a", { href: "#/p/" + encodeURIComponent(p.name) }, icon("settings", 16), t("manage")),
      canPrimary && h("button", { type: "button", disabled: S.busy, onclick: () => doPrimary(p.name) }, icon("star", 16), t("makePrimary")),
      del));
}

function pageProfiles() {
  const list = profiles(), c = fleet(list), flt = S.pfFilter;
  const tests = { all: () => true, live: (p) => liveness(p) === "live", wait: (p) => liveness(p) === "wait", stopped: (p) => !p.up, bad: hasProblem };
  const shown = list.filter(tests[flt] || tests.all);
  const chip = (key, n) => h("button", { class: "filter-chip", type: "button", "aria-pressed": String(flt === key),
    onclick: () => { S.pfFilter = key; renderMain(); announce(t("shownOf", { n: list.filter(tests[key]).length, m: list.length }) + (key === "all" ? "" : " · " + t("flt" + key[0].toUpperCase() + key.slice(1)))); } },
    t("flt" + key[0].toUpperCase() + key.slice(1)), h("b", {}, String(n)));
  const cols = ["colName", "colAddress", "colState", "colTraffic", "colSub", "colActions"];
  return h("div", {},
    heading(t("navProfiles"), t("profilesHeading"), t("profilesSub"),
      h("a", { class: "btn btn-primary", href: "#/new" }, icon("plus", 16), t("addProfile"))),
    !list.length ? emptyState() : [
      h("div", { class: "filter-chips", role: "group", "aria-label": t("fltLabel") },
        chip("all", c.all), chip("live", c.live), chip("wait", c.wait), chip("stopped", c.stopped), chip("bad", c.bad)),
      !shown.length ? h("div", { class: "card empty" }, icon("server", 32), h("strong", {}, t("noMatch")),
        h("button", { class: "btn btn-secondary", type: "button", onclick: () => { S.pfFilter = "all"; renderMain(); } }, t("showAll")))
        : h("div", { class: "table-scroll has-menu" }, h("table", {},
          h("thead", {}, h("tr", {}, cols.map((k) => h("th", { scope: "col" }, t(k))))),
          h("tbody", {}, shown.map((p) => {
            const a = agg(p), sub = p.subscription;
            return h("tr", {},
              h("td", {}, h("a", { class: "name", href: "#/p/" + encodeURIComponent(p.name) }, icon("server", 18), p.name)),
              h("td", { "data-label": t("colAddress") }, h("span", { class: "ip-pill" }, addrOf(p))),
              h("td", { "data-label": t("colState") }, h("div", {}, h("div", { class: "actions" }, pill(p), primaryBadge(p)), h("div", { class: "sub" }, t("handshake") + ": " + (a.hs ? fmtAgo(a.hs) : t("never"))))),
              h("td", { class: "mono", "data-label": t("colTraffic") }, h("span", {}, "↓ " + fmtBytes(a.rx), h("br"), "↑ " + fmtBytes(a.tx))),
              h("td", { "data-label": t("colSub") }, h("span", {}, sub ? fmtInterval(sub.interval_min) : t("subNone"),
                sub && sub.last_error && h("div", { class: "sub" }, h("span", { class: "pill pill-off" }, t("subLastError"))))),
              h("td", {}, h("div", { class: "row-actions" }, toggleBtn(p, true), rowMenu(p))));
          }))))]);
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
    if (!S.foEdit || !S.dirty) S.foEdit = { settings: { ...S.fo.settings }, order: [...(S.fo.order || [])] };
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
    S.foEdit = { settings: { ...S.fo.settings }, order: [...(S.fo.order || [])] };
  }, t("foSaved"));
  const check = () => act(async () => { S.fo = await api("POST", "/api/failover/check"); S.foEdit = { settings: { ...S.fo.settings }, order: [...(S.fo.order || [])] }; }, t("foChecked"));

  const row = (name, i) => {
    const hh = health[name] || { name, state: "down", handshake_age: -1 };
    const p = profiles().find((x) => x.name === name) || { name };
    return h("tr", {},
      h("td", { class: "mono" }, i >= 0 ? String(i + 1) : "—"),
      h("td", {}, h("a", { class: "name", href: "#/p/" + encodeURIComponent(name) }, icon("server", 18), name), p.primary && h("div", { class: "sub" }, primaryBadge(p))),
      h("td", {}, foStatePill(hh), hh.reason && h("div", { class: "sub" }, foReason(hh.reason)), hh.cooldown_left > 0 && h("div", { class: "sub" }, t("foCooldownLeft", { n: hh.cooldown_left }))),
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
      statCard(t("foSwitches"), String(fo.switches || 0), foReason(fo.last_switch_reason) || t("foNoSwitches")),
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
    return hh ? h("span", {}, foStatePill(hh), hh.reason && h("div", { class: "sub" }, foReason(hh.reason))) : h("span", { class: "pill pill-idle" }, t("foState_starting"));
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
// Редактор правил: блоки-сервисы поверх того же текста правил. Модель текст <-> блоки лежит в
// static/rules.js, каталог и значки — в static/services.js (Simple Icons, CC0). Хранилище одно: текст.
const RULES = window.AWG_RULES, SVC = window.AWG_SERVICES;
const GEO_PRESETS = {
  loyal: ["https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat", "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat"],
  iplist: ["https://iplist.opencck.org/?format=geosite&data=domains", "https://iplist.opencck.org/?format=geoip&data=cidr4"],
  // runetfreedom: теги как у Loyalsoldier (v2fly) плюс ru-blocked, ru-available-only-inside и др.
  runet: ["https://raw.githubusercontent.com/runetfreedom/russia-v2ray-rules-dat/release/geosite.dat", "https://raw.githubusercontent.com/runetfreedom/russia-v2ray-rules-dat/release/geoip.dat"],
};
const GEO_PRESET_KEYS = ["loyal", "iplist", "runet"];
// У IPList теги называются иначе: это имя портала (telegram.org) или группы (messengers), а не
// «telegram» как у Loyalsoldier. Пока выбран IPList, каталог подставляет его имена. Сервисов,
// которых в IPList нет (Google, Apple, Steam и др.), здесь нет, для них остаются обычные теги.
const SVC_IPLIST = {
  youtube: ["geosite:youtube.com", "geoip:youtube.com"],
  telegram: ["geosite:telegram.org", "geoip:telegram.org"],
  openai: ["geosite:chatgpt.com", "geoip:chatgpt.com"],
  anthropic: ["geosite:claude.ai", "geoip:claude.ai"],
  netflix: ["geosite:netflix.com", "geoip:netflix.com"],
  instagram: ["geosite:instagram.com", "geoip:instagram.com"],
  facebook: ["geosite:facebook.com", "geoip:facebook.com"],
  twitter: ["geosite:x.com", "geoip:x.com"],
  tiktok: ["geosite:tiktok.com", "geoip:tiktok.com"],
  spotify: ["geosite:spotify.com", "geoip:spotify.com"],
  discord: ["geosite:discord", "geoip:discord"],
  whatsapp: ["geosite:whatsapp.com", "geoip:whatsapp.com"],
  signal: ["geosite:signal.org", "geoip:signal.org"],
  linkedin: ["geosite:linkedin.com", "geoip:linkedin.com"],
};
// Замена тегов при смене источника: для сервисов каталога, у которых в IPList свои имена, пары
// «тег Loyalsoldier ↔ тег IPList» одного вида (geosite с geosite, geoip с geoip). Тег, у которого
// пары нет (у Loyalsoldier нет geoip:youtube), при переходе в эту сторону убирается.
function tagPairs(toIplist) {
  const map = {}, drop = {};
  SVC.catalog.forEach((s) => {
    const alt = SVC_IPLIST[s.id];
    if (!alt) return;
    const from = toIplist ? s.conds : alt, to = toIplist ? alt : s.conds;
    from.forEach((c) => {
      const k = c.split(":")[0], m = to.find((x) => x.split(":")[0] === k);
      if (m && m !== c) map[c] = m;
    });
    if (!toIplist) alt.forEach((c) => { if (!s.conds.some((x) => x.split(":")[0] === c.split(":")[0])) drop[c] = true; });
  });
  return { map, drop };
}
// Сколько условий в правилах записано тегами другого источника; с fix — заменяет их.
function retag(toIplist, fix) {
  const { map, drop } = tagPairs(toIplist);
  let n = 0;
  if (!S.rt || !S.rt.model) return 0;
  RULES.blocks(S.rt.model).forEach((b) => {
    const keep = [];
    b.lines.forEach((l) => {
      const key = l.kind + ":" + l.value;
      if (drop[key]) { n++; if (fix) return; }
      else if (map[key]) {
        n++;
        if (fix) {
          const nv = map[key];
          if (b.lines.some((o) => o !== l && o.kind + ":" + o.value === nv) || keep.some((o) => o.kind + ":" + o.value === nv)) return;
          l.value = nv.slice(nv.indexOf(":") + 1); l.raw = null;
        }
      }
      keep.push(l);
    });
    if (fix) b.lines = keep;
  });
  return n;
}
// IPList выбран на странице (ссылки обеих баз совпадают с его набором).
const iplistChosen = () => { const ed = S.polEdit || {}; return ed.geosite_url === GEO_PRESETS.iplist[0] && ed.geoip_url === GEO_PRESETS.iplist[1]; };
// Условия записи каталога под источник баз, который сейчас выбран на странице.
function svcConds(s) {
  const alt = SVC_IPLIST[s.id];
  return alt && iplistChosen() ? alt : s.conds;
}
// База в виде списка сервисов: источник вида IPList (ссылка с format=geosite) и совпадает ли загруженная
// база с источником, выбранным на странице.
const iplistLike = () => /[?&]format=geosite\b/.test((S.polEdit || {}).geosite_url || "");
const dbMatches = () => {
  const g = (S.pol && S.pol.geo) || {}, ed = S.polEdit || {};
  return !!(g.geosite && g.geosite.present && (!g.geosite.source || g.geosite.source === ed.geosite_url));
};
const iconUrl = (tag) => "/api/geo/icon?site=" + encodeURIComponent(tag);
// Значок сайта IPList отдаёт контейнер; его просят только для тегов, похожих на имя сайта.
const isSiteTag = (v) => /^[a-z0-9][a-z0-9-]*(\.[a-z0-9-]+)+$/.test(v);
const regionNames = () => { try { return new Intl.DisplayNames([lang === "en" ? "en" : "ru"], { type: "region" }); } catch (_) { return null; } };
// Группы IPList (каталоги config/ в их репозитории); остальные теги — отдельные сайты, в том числе
// без домена в имени (copilot, arma2). Новые группы IPList попадут в «Сайты», выбор при этом работает.
const IPL_GROUPS = new Set(["ai", "anime", "art", "casino", "discord", "education", "finance", "games", "hosting", "jetbrains",
  "messengers", "music", "news", "porn", "shop", "socials", "tools", "torrent", "video", "youtube"]);
// Весь список из скачанной базы: теги geosite и geoip одним списком. Известным сервисам достаются название
// и значок каталога, остальным буквенная плитка (у сайтов IPList подгружается значок сайта). Условия:
// geosite и, если у тега есть адреса, geoip.
function fullEntries(sites, ips) {
  const known = {}, sset = new Set(sites), iset = new Set(ips), ipl = iplistLike(), regions = regionNames();
  SVC.catalog.forEach((s) => svcConds(s).forEach((c) => { known[c.replace(/^geo(?:site|ip):/, "")] = s; }));
  return Array.from(new Set(sites.concat(ips))).sort().map((tag) => {
    const s = known[tag], two = /^[a-z]{2}$/.test(tag);
    let name = s ? svcName(s) : tag;
    if (!s && !sset.has(tag) && two && regions) { try { const r = regions.of(tag.toUpperCase()); if (r && r.toLowerCase() !== tag) name = r + " (" + tag + ")"; } catch (_) { /* не код страны */ } }
    let sec = "svc";
    if (ipl) sec = IPL_GROUPS.has(tag) ? "groups" : "sites";
    else if (!sset.has(tag) && two) sec = "countries";
    else if (/^(category|geolocation|tld)-/.test(tag) || tag === "cn" || tag === "private") sec = "cats";
    return { key: tag, id: s ? s.id : "", name, sec,
      tile: { slug: s ? s.icon : null, id: s ? s.id : "", name, ico: ipl && !s && isSiteTag(tag) ? iconUrl(tag) : "" },
      conds: (sset.has(tag) ? ["geosite:" + tag] : []).concat(iset.has(tag) ? ["geoip:" + tag] : []),
      n: (tag + " " + name).toLowerCase() };
  });
}
const SVG_NS = "http://www.w3.org/2000/svg";
const fmtNum = (n) => Number(n).toLocaleString(lang === "en" ? "en-US" : "ru-RU");
const rtEls = {};      // живые узлы текущей страницы: chips, list, notes, bar
let rtSayTimer = 0;

function rtReset() {
  const old = S.rt || {};
  S.rt = { model: RULES.parse((S.polEdit && S.polEdit.rules) || ""), filter: old.filter || "all", q: old.q || "", adv: !!old.adv, uid: old.uid || 0, drag: null };
}
const rtUid = (b) => b.uid || (b.uid = ++S.rt.uid);
// Пока человек работает с полем на странице маршрутов, фоновое обновление её не перерисовывает.
function rtTyping() {
  const a = document.activeElement;
  return !!(S.route.page === "policy" && a && mainEl && mainEl.contains(a) && /^(SELECT|INPUT|TEXTAREA)$/.test(a.tagName));
}

// Названия каталога на языке интерфейса (у большинства сервисов это бренд, он не переводится).
const svcName = (s) => (s.id === "ru" ? t("svcRu") : s.id === "private" ? t("svcPrivate") : s.name);

// Название, значок и условия блока для показа.
function rtInfo(b) {
  const svc = RULES.service(b, SVC.catalog), conds = RULES.conds(b);
  let name = b.name;
  if (!name && svc) name = svcName(svc);
  if (!name) name = conds.length === 1 ? conds[0].value : conds.length ? t("rtCustom", { n: conds.length }) : t("rtEmpty");
  const slug = svc ? svc.icon : (b.icon && SVC.icons[b.icon] ? b.icon : null);
  let ico = "";
  if (!slug && iplistLike()) { const c = conds.find((x) => (x.kind === "geosite" || x.kind === "geoip") && isSiteTag(x.value)); if (c) ico = iconUrl(c.value); }
  return { svc, name, slug, id: svc ? svc.id : "", conds, ico };
}
function rtTile(info, size) {
  const tile = h("span", { class: "svc-tile" + (size === "lg" ? " lg" : "") + (info.slug && info.id ? " svc-" + info.id : "") });
  if (info.slug && SVC.icons[info.slug]) {
    const s = document.createElementNS(SVG_NS, "svg");
    s.setAttribute("viewBox", "0 0 24 24"); s.setAttribute("aria-hidden", "true"); s.setAttribute("class", "svc-mark");
    const p = document.createElementNS(SVG_NS, "path");
    p.setAttribute("d", SVC.icons[info.slug]); p.setAttribute("fill", "currentColor");
    s.append(p); tile.append(s);
    return tile;
  }
  if (info.id === "private") { tile.classList.add("hue-0"); tile.append(icon("lock", size === "lg" ? 22 : 18)); return tile; }
  let sum = 0;
  for (const ch of info.name) sum = (sum * 31 + ch.codePointAt(0)) % 997;
  tile.classList.add("hue-" + (sum % 8));
  tile.append(h("span", { "aria-hidden": "true" }, info.id === "ru" ? "RU" : (Array.from(info.name)[0] || "?").toUpperCase()));
  if (info.ico) {
    // Значок сайта ложится поверх буквы, а если его нет или это пустой пиксель, остаётся буква.
    const img = h("img", { class: "svc-ico", alt: "", loading: "lazy", decoding: "async", src: info.ico,
      onload: () => { if (img.naturalWidth > 1) tile.classList.add("has-ico"); else img.remove(); }, onerror: () => img.remove() });
    tile.append(img);
  }
  return tile;
}

function rtDestOptions(cur) {
  const tunnels = profiles().map((p) => p.name), groups = ((S.grp && S.grp.groups) || []).map((x) => x.name);
  const known = new Set(tunnels.concat(groups));
  const opt = (value, label, extra) => h("option", { value, selected: cur === value, ...(extra || {}) }, label);
  const out = [];
  if (cur === "mixed") out.push(opt("mixed", t("rtMixed"), { disabled: true }));
  out.push(opt("PROXY", t("rtDestProxy")));
  if (tunnels.length) out.push(h("optgroup", { label: t("rtTunnels") }, tunnels.map((n) => opt("PROXY@" + n, n))));
  if (groups.length) out.push(h("optgroup", { label: t("rtGroups") }, groups.map((n) => opt("PROXY@" + n, n))));
  if (cur.startsWith("PROXY@") && !known.has(cur.slice(6))) out.push(opt(cur, cur.slice(6) + " — " + t("rtGone")));
  out.push(opt("DIRECT", t("rtDestDirect")), opt("DROP", t("rtDestDrop")));
  return out;
}
const rtDestClass = (d) => (d === "DIRECT" ? "direct" : d === "DROP" ? "drop" : d === "mixed" ? "mixed" : "proxy");

// Устройства из справочника по ссылкам правила (@id); сырые адреса — как есть.
function devByRef(ref) {
  const id = String(ref).replace(/^@/, "");
  return ((S.dev && S.dev.devices) || []).find((d) => d.id === id) || null;
}
function scopeDevs(sc) {
  return (sc ? sc.src : []).map((r) => { const d = r.charAt(0) === "@" ? devByRef(r) : null; return d ? { name: d.name, addr: d.addr, kind: d.kind } : { name: r.charAt(0) === "@" ? r : r, addr: r.charAt(0) === "@" ? "?" : r, kind: "other", missing: r.charAt(0) === "@" }; });
}
function rtDevicesCell(b, info) {
  const sc = RULES.scope(b);
  if (!sc || !sc.src.length) return h("span", { class: "rt-alldev" }, icon("devices", 15), t("rtAllDevices"));
  const list = scopeDevs(sc);
  return h("details", { class: "rt-devpop" },
    h("summary", { "aria-label": t("rtDevicesOf", { name: info.name }) }, icon("devices", 15), list.length === 1 ? list[0].name : t("rtNDevices", { n: list.length }), icon("chevron", 14)),
    h("div", { class: "rt-devpop-list" }, h("b", {}, t("rtColDevices") + " · " + list.length),
      list.map((d) => h("div", { class: "rt-devpop-item" + (d.missing ? " is-missing" : "") }, devIcon(d.kind, 15), h("span", {}, d.name), h("span", { class: "mono" }, d.addr)))));
}
function rtScopeChips(b) {
  const sc = RULES.scope(b);
  if (!sc) return [];
  const out = [];
  if (sc.ports.length) out.push(h("span", { class: "rt-chip k-port" }, h("i", {}, "port"), sc.ports.join(", ")));
  if (sc.net.length) out.push(h("span", { class: "rt-chip k-net" }, h("i", {}, "net"), sc.net.join(",")));
  return out;
}

// Счётчик соединений правила из мониторинга (только когда текст сохранён: номера строк те же).
function rtHits(b) {
  const by = S.flows && S.flows.by_line;
  if (!by || S.dirty || S.flows.source === "none") return null;
  let n = 0;
  for (const l of b.lines) if (l.ln && by[l.ln]) n += by[l.ln];
  return h("span", { class: "rt-hits" + (n ? "" : " zero"), title: t("rtHits", { n }) }, String(n));
}

function rtCondChip(c, g) {
  const loaded = c.kind === "geosite" ? (g.site_loaded || {})[c.value] : c.kind === "geoip" ? (g.ip_loaded || {})[c.value] : undefined;
  const miss = (g.missing || []).includes(c.key);
  return h("span", { class: "rt-chip k-" + c.kind + (c.off ? " is-off" : "") + (miss ? " is-missing" : "") },
    h("i", {}, c.kind === "cidr" ? "net" : c.kind), c.value,
    loaded != null && h("b", {}, fmtNum(loaded)),
    miss && icon("alert", 13),
    (c.off || miss) && h("span", { class: "sr-only" }, " (" + [c.off && t("rtOffCond"), miss && t("rtMissing")].filter(Boolean).join(", ") + ")"));
}

// Одно изменение в редакторе: текст правил пересобирается, страница помечается несохранённой.
function rtChanged(fk) {
  S.polEdit.rules = RULES.serialize(S.rt.model);
  const ta = document.getElementById("pol-rules");
  if (ta && ta.value !== S.polEdit.rules) ta.value = S.polEdit.rules;
  rtDirty();
  rtDraw(fk);
}
function rtDirty() {
  if (!S.dirty) { S.dirty = true; announce(t("rtUnsaved")); }
  rtBar();
}
const rtRevert = () => {
  S.dirty = false; S.polEdit = { ...S.pol.settings }; rtReset(); renderMain(); announce(t("rtReverted"));
};
const rtSave = () => act(async () => {
  const ed = S.polEdit;
  polGen++;
  try {
    S.pol = await api("PUT", "/api/policy", { ...ed, geo_update_hours: parseInt(ed.geo_update_hours, 10) || 0 });
  } catch (e) {
    throw new Error(e.data && e.data.errors ? e.data.errors.join(" · ") : e.message);
  }
  S.polEdit = { ...S.pol.settings }; rtReset();
  if (S.pol.errors && S.pol.errors.length) { S.dirty = false; rtBar(); throw new Error(t("polSavedWarn") + " " + S.pol.errors.join("; ")); }
}, t("polSaved"));

function rtBar() {
  const bar = rtEls.bar;
  if (!bar) return;
  bar.hidden = !S.dirty;
  bar.replaceChildren(
    h("span", { class: "save-msg" }, icon("info", 16), t("rtUnsaved")),
    h("div", { class: "actions" },
      h("button", { class: "btn btn-secondary", type: "button", disabled: S.busy, onclick: rtRevert }, t("rtRevert")),
      h("button", { class: "btn btn-primary", type: "button", disabled: S.busy, onclick: rtSave }, icon("check", 16), t("foSave"))));
}

function rtMenu(b, pos, total, locked) {
  const m = S.rt.model, uid = rtUid(b);
  const item = (ic, label, fn, o) => h("button", { type: "button", class: o && o.danger ? "is-danger" : "", disabled: !!(o && o.disabled), onclick: (e) => { const d = e.currentTarget.closest("details"); if (d) d.open = false; fn(); } }, icon(ic, 16), label);
  return h("details", { class: "row-menu" },
    h("summary", { class: "icon-btn", "data-fk": uid + ":menu", title: t("moreActions"), "aria-label": t("moreActions") + ": " + rtInfo(b).name }, icon("more", 18)),
    h("div", { class: "row-menu-list" },
      item("settings", t("rtEdit"), () => rtDialog(b)),
      item("layers", t("rtDup"), () => {
        const c = RULES.clone(b), i = rtInfo(b);
        c.name = t("rtCopy", { name: i.name }); c.icon = i.svc ? i.svc.id : b.icon;
        RULES.add(m, c, m.items.indexOf(b) + 1);
        rtChanged(rtUid(c) + ":menu");
      }),
      item("arrowUp", t("rtUp"), () => { RULES.step(m, b, -1); rtChanged(uid + ":menu"); announce(t("rtMoved", { name: rtInfo(b).name, n: RULES.blocks(m).indexOf(b) + 1, m: total })); }, { disabled: locked || pos === 0 }),
      item("arrowDown", t("rtDown"), () => { RULES.step(m, b, 1); rtChanged(uid + ":menu"); announce(t("rtMoved", { name: rtInfo(b).name, n: RULES.blocks(m).indexOf(b) + 1, m: total })); }, { disabled: locked || pos === total - 1 }),
      item("trash", t("delete"), () => { RULES.remove(m, b); rtChanged(); }, { danger: true })));
}

function rtRow(b, pos, total, locked, g) {
  const m = S.rt.model, info = rtInfo(b), uid = rtUid(b), d = RULES.dest(b), off = RULES.isOff(b);
  const li = h("li", { class: "rt-row" + (off ? " is-off" : ""), "data-uid": String(uid) });
  const move = (dir) => {
    RULES.step(m, b, dir);
    rtChanged(uid + ":grip");
    announce(t("rtMoved", { name: info.name, n: RULES.blocks(m).indexOf(b) + 1, m: total }));
  };
  const grip = h("button", { type: "button", class: "rt-grip", "data-fk": uid + ":grip", disabled: locked,
    title: locked ? t("rtGripLocked") : "", "aria-label": locked ? t("rtGripLocked") : t("rtGrip", { name: info.name, n: pos + 1, m: total }),
    onkeydown: (e) => {
      if (e.key === "ArrowUp" && pos > 0) { e.preventDefault(); move(-1); }
      else if (e.key === "ArrowDown" && pos < total - 1) { e.preventDefault(); move(1); }
    } }, icon("grip", 18));
  // Перетаскивание мышью: строка становится draggable только пока нажата ручка.
  grip.addEventListener("pointerdown", (e) => { if (!locked && e.pointerType !== "touch") li.setAttribute("draggable", "true"); });
  const unarm = () => li.removeAttribute("draggable");
  grip.addEventListener("pointerup", unarm); grip.addEventListener("pointercancel", unarm);
  li.addEventListener("dragstart", (e) => {
    if (e.target !== li) return;
    S.rt.drag = b; li.classList.add("is-dragging");
    e.dataTransfer.effectAllowed = "move"; e.dataTransfer.setData("text/plain", info.name);
  });
  li.addEventListener("dragend", () => { S.rt.drag = null; unarm(); rtEls.list && rtEls.list.querySelectorAll(".is-dragging,.drop-before,.drop-after").forEach((x) => x.classList.remove("is-dragging", "drop-before", "drop-after")); });
  const after = (e) => { const r = li.getBoundingClientRect(); return e.clientY > r.top + r.height / 2; };
  li.addEventListener("dragover", (e) => {
    if (!S.rt.drag || S.rt.drag === b) return;
    e.preventDefault(); e.dataTransfer.dropEffect = "move";
    const a = after(e); li.classList.toggle("drop-after", a); li.classList.toggle("drop-before", !a);
  });
  li.addEventListener("dragleave", (e) => { if (!li.contains(e.relatedTarget)) li.classList.remove("drop-before", "drop-after"); });
  li.addEventListener("drop", (e) => {
    const src = S.rt.drag;
    if (!src || src === b) return;
    e.preventDefault();
    const list = RULES.blocks(m);
    RULES.move(m, src, after(e) ? (list[list.indexOf(b) + 1] || null) : b);
    S.rt.drag = null;
    rtChanged(rtUid(src) + ":grip");
    announce(t("rtMoved", { name: rtInfo(src).name, n: RULES.blocks(m).indexOf(src) + 1, m: total }));
  });

  const sel = h("select", { class: "rt-dest dest-" + rtDestClass(d), "data-fk": uid + ":dest", "data-keep": "",
    "aria-label": t("rtDestOf", { name: info.name }),
    onchange: (e) => { RULES.setDest(b, e.target.value); rtChanged(uid + ":dest"); } }, rtDestOptions(d));
  const sw = h("button", { type: "button", class: "rt-switch", role: "switch", "aria-checked": String(!off), "data-fk": uid + ":tog",
    "aria-label": off ? t("rtToggleOn", { name: info.name }) : t("rtToggleOff", { name: info.name }),
    onclick: () => { RULES.setOff(b, !off); rtChanged(uid + ":tog"); announce(info.name + ": " + (off ? t("rtStateOn") : t("rtStateOff"))); } },
  h("span", { class: "rt-knob" }), h("span", { class: "sr-only" }, off ? t("rtStateOff") : t("rtStateOn")));

  const MAX = 4, conds = info.conds;
  li.append(
    grip,
    h("div", { class: "rt-svc" }, rtTile(info), h("div", { class: "rt-name" }, h("b", {}, info.name), h("span", {}, conds.length ? t("rtColConds") + ": " + conds.length : t("rtNoCond"))),
      rtHits(b)),
    h("div", { class: "rt-conds" }, conds.slice(0, MAX).map((c) => rtCondChip(c, g)),
      conds.length > MAX && h("button", { type: "button", class: "rt-chip rt-more", onclick: () => rtDialog(b) }, t("rtMore", { n: conds.length - MAX })),
      rtScopeChips(b)),
    h("div", { class: "rt-dev" }, rtDevicesCell(b, info)),
    h("div", { class: "rt-to" }, sel),
    h("div", { class: "rt-on" }, sw),
    h("div", { class: "rt-menu" }, rtMenu(b, pos, total, locked)));
  return li;
}

function rtDraw(fk) {
  const R = S.rt, m = R.model, all = RULES.blocks(m), g = (S.pol && S.pol.geo) || {};
  RULES.lineNos(m);
  if (!rtEls.list) return;
  const q = R.q.trim().toLowerCase();
  const tests = { all: () => true, proxy: (b) => rtDestClass(RULES.dest(b)) === "proxy", direct: (b) => RULES.dest(b) === "DIRECT", drop: (b) => RULES.dest(b) === "DROP",
    devs: (b) => { const sc = RULES.scope(b); return !sc || sc.src.length > 0; }, extra: (b) => { const sc = RULES.scope(b); return !sc || sc.ports.length > 0 || sc.net.length > 0; }, off: (b) => RULES.isOff(b) };
  const TILE_ICON = { all: "layers", proxy: "shield", direct: "arrow", drop: "x", devs: "devices", extra: "filter", off: "pause" };
  const match = (b) => { if (!q) return true; const i = rtInfo(b); return i.name.toLowerCase().includes(q) || i.conds.some((c) => c.key.includes(q)); };
  const shown = all.filter((b) => tests[R.filter](b) && match(b));
  const locked = R.filter !== "all" || q !== "";

  rtEls.chips.replaceChildren(...["all", "proxy", "direct", "drop", "devs", "extra", "off"].map((k) => h("button", { class: "rt-tile t-" + k, type: "button", "aria-pressed": String(R.filter === k),
    onclick: () => { R.filter = k; rtDraw(); announce(t("shownOf", { n: all.filter((b) => tests[k](b) && match(b)).length, m: all.length }) + (k === "all" ? "" : " · " + t("rtFlt" + k[0].toUpperCase() + k.slice(1)))); } },
  h("span", { class: "rt-tile-ic" }, icon(TILE_ICON[k], 18)), h("span", { class: "rt-tile-l" }, t("rtFlt" + k[0].toUpperCase() + k.slice(1))), h("b", {}, String(all.filter(tests[k]).length)))));

  const bad = RULES.bad(m);
  rtEls.notes.replaceChildren(...(bad.length ? [h("div", { class: "flash warn rt-bad", role: "status" }, icon("alert", 18),
    h("div", { class: "msg" }, t("rtBadLines", { n: bad.length }),
      h("ul", {}, bad.slice(0, 3).map((x) => h("li", {}, h("code", {}, x.text.trim()))), bad.length > 3 && h("li", {}, t("rtMore", { n: bad.length - 3 }))),
      h("div", { class: "actions" },
        h("button", { class: "btn btn-secondary btn-small", type: "button", onclick: () => {
          R.adv = true;
          const d = document.querySelector(".rt-adv"), ta = document.getElementById("pol-rules");
          if (d) d.open = true;
          if (ta) { ta.scrollIntoView({ block: "center" }); ta.focus(); }
        } }, icon("file", 16), t("rtBadShow")),
        h("button", { class: "btn btn-secondary btn-small", type: "button", onclick: () => { m.items = m.items.filter((it) => !it.bad); rtChanged(); } }, icon("trash", 16), t("rtBadDrop")))))] : []));

  if (!all.length) {
    rtEls.list.replaceChildren(h("li", { class: "rt-empty" }, icon("route", 26), h("b", {}, t("rtNoRules")), h("span", {}, t("rtNoRulesHint")),
      h("button", { class: "btn btn-primary", type: "button", onclick: () => rtDialog(null) }, icon("plus", 16), t("rtAdd"))));
  } else if (!shown.length) {
    rtEls.list.replaceChildren(h("li", { class: "rt-empty" }, icon("info", 26), h("b", {}, t("rtNoMatch")), h("span", {}, t("rtNoMatchHint"))));
  } else {
    rtEls.list.replaceChildren(...shown.map((b) => rtRow(b, all.indexOf(b), all.length, locked, g)));
  }
  if (fk) { const el = rtEls.list.querySelector('[data-fk="' + fk + '"]'); if (el && !el.disabled) el.focus(); }
}

/* Диалог «Добавить сервис / Изменить»: правки идут в черновик и попадают в правила только по «Готово». */
function rtDialog(b) {
  const isNew = !b, info = b ? rtInfo(b) : null;
  const draft = { t: "block", head: true, name: b ? info.name : "", icon: b ? (info.svc ? info.svc.id : b.icon) : "", hraw: b ? b.hraw : null,
    pre: b ? b.pre : [], trail: b ? b.trail : 1, lines: b ? b.lines.map((l) => ({ ...l, pre: l.pre.slice() })) : [] };
  const ui = { dest: b ? RULES.dest(b) : "PROXY", touched: !!b, lastPick: b && info.svc ? svcName(info.svc) : "" };
  const g = (S.pol && S.pol.geo) || {};

  const dlg = h("dialog", { class: "rt-dialog", "aria-labelledby": "rt-dlg-title" });
  const err = h("p", { class: "rt-err", role: "alert", hidden: true });
  const showErr = (s) => { err.textContent = s || ""; err.hidden = !s; };
  const close = () => { if (dlg.open) dlg.close(); };
  dlg.addEventListener("close", () => {
    dlg.remove();
    setTimeout(() => {
      if (document.activeElement && document.activeElement !== document.body) return;
      const el = b ? rtEls.list && rtEls.list.querySelector('[data-fk="' + rtUid(b) + ':menu"]') : document.getElementById("rt-add-btn");
      if (el) el.focus();
    });
  });
  dlg.addEventListener("click", (e) => { if (e.target === dlg) close(); });

  const nameInp = h("input", { id: "rt-name", type: "text", value: draft.name, maxlength: "60", autocomplete: "off",
    oninput: (e) => { draft.name = e.target.value; ui.touched = true; } });
  const tileSlot = h("span", { class: "rt-dlg-tile" });
  const drawTile = () => tileSlot.replaceChildren(rtTile(draft.name || draft.icon || draft.lines.length ? rtInfo(draft) : { slug: null, id: "", name: "+" }, "lg"));

  // каталог: по умолчанию наш (с названиями и значками); когда загружена база выбранного источника,
  // добавляется её полный список, из которого можно выбрать любую категорию
  const grid = h("div", { class: "svc-grid", role: "group", "aria-label": t("rtCatalog") });
  const catHint = h("p", { class: "hint" }, t("rtCatalogHint"));
  const cat = { all: [], featured: [], big: false, sections: false, picked: "" };
  const SHOWN = 300;
  const find = h("input", { type: "search", id: "rt-find", "aria-label": t("rtCatalogSearch"), placeholder: t("rtCatalogSearch"), autocomplete: "off", oninput: () => { clearTimeout(cat.timer); cat.timer = setTimeout(drawCatalog, 120); } });
  function drawCatalog() {
    const q = find.value.trim().toLowerCase(), idle = cat.big && !q;
    let list = idle ? cat.featured : cat.all.filter((e) => !q || e.n.includes(q)), more = 0;
    if (list.length > SHOWN) { more = list.length - SHOWN; list = list.slice(0, SHOWN); }
    const nodes = [], pickEl = (e) => {
      const el = h("button", { type: "button", class: "svc-pick", "aria-pressed": String(cat.picked === e.key),
        onclick: () => {
          if (!draft.name || draft.name === ui.lastPick) { draft.name = e.name; nameInp.value = draft.name; }
          draft.icon = e.id; ui.lastPick = e.name; cat.picked = e.key;
          if (!ui.touched) draft.lines = [];
          e.conds.forEach((c) => RULES.addCond(draft, c));
          grid.querySelectorAll(".svc-pick").forEach((p) => p.setAttribute("aria-pressed", String(p === el)));
          drawConds(); drawTile();
        } },
      rtTile(e.tile), h("span", {}, e.name));
      return el;
    };
    if (cat.sections && !idle) {
      for (const sec of ["groups", "sites", "svc", "cats", "countries"]) {
        const part = list.filter((e) => e.sec === sec);
        if (part.length) nodes.push(h("h4", { class: "svc-sub" }, t("rtSec_" + sec)), ...part.map(pickEl));
      }
    } else nodes.push(...list.map(pickEl));
    if (more) nodes.push(h("p", { class: "hint svc-more" }, t("rtCatMore", { m: SHOWN, n: more })));
    if (!list.length) nodes.push(h("p", { class: "hint svc-more" }, t("rtCatNone")));
    grid.replaceChildren(...nodes);
  }
  cat.all = cat.featured = SVC.catalog.map((s) => ({ key: s.id, id: s.id, name: svcName(s), tile: { slug: s.icon, id: s.id, name: svcName(s) }, conds: svcConds(s),
    n: (svcName(s) + " " + s.name + " " + s.id).toLowerCase() }));
  cat.picked = b && info.svc ? info.svc.id : "";
  drawCatalog();
  if (dbMatches()) {
    Promise.all(["geosite", "geoip"].map((k) => api("GET", "/api/geo/categories?kind=" + k + "&all=1").then((r) => r.items || []).catch(() => []))).then(([sites, ips]) => {
      if (!dlg.isConnected || !sites.length) return;
      cat.all = fullEntries(sites, ips); cat.sections = true; cat.big = cat.all.length > 300;
      catHint.textContent = iplistLike() ? t("rtCatalogIplist") : t("rtCatalogFull", { n: fmtNum(cat.all.length) });
      drawCatalog();
    });
  } else if (S.polEdit && S.polEdit.geosite_url) catHint.textContent = t("rtCatalogNoDb");

  // условия
  const list = h("ul", { class: "rt-cond-list" });
  function drawConds() {
    const cs = RULES.conds(draft);
    list.replaceChildren(...(cs.length ? cs.map((c, i) => h("li", { class: c.off ? "is-off" : "" },
      rtCondChip(c, g),
      h("button", { type: "button", class: "icon-btn", "aria-label": t("rtCondRemove", { v: c.key }), onclick: () => { RULES.removeCond(draft, i); ui.touched = true; drawConds(); drawTile(); showErr(""); } }, icon("x", 16))))
      : [h("li", { class: "rt-cond-none" }, t("rtCondNone"))]));
  }
  const kinds = [["auto", t("rtKindAuto")], ["domain", t("rtKindDomain")], ["full", t("rtKindFull")], ["keyword", t("rtKindKeyword")], ["cidr", t("rtKindCidr")], ["geosite", "geosite"], ["geoip", "geoip"]];
  const kindSel = h("select", { id: "rt-kind", "aria-label": t("rtKind"), onchange: () => suggest() }, kinds.map(([v, l]) => h("option", { value: v }, l)));
  const dl = h("datalist", { id: "rt-cats" });
  const valInp = h("input", { id: "rt-val", type: "text", class: "mono", list: "rt-cats", autocomplete: "off", spellcheck: "false", placeholder: "example.com, 10.0.0.0/8, geosite:youtube",
    oninput: () => { suggest(); showErr(""); },
    onkeydown: (e) => { if (e.key === "Enter") { e.preventDefault(); addConds(); } } });
  const reCat = /^[a-z0-9!@_.:-]{1,64}$/, reHost = /^[a-z0-9_]([a-z0-9_-]{0,62}\.)*[a-z0-9_-]{0,63}$/, reNet = /^[0-9a-f:.]+(\/\d{1,3})?$/i;
  function addConds() {
    const toks = valInp.value.split(/[\s,;]+/).filter(Boolean), bad = [], dup = [];
    let added = 0;
    for (const tok of toks) {
      const k = kindSel.value;
      const hasPrefix = /^(domain|suffix|domain-suffix|full|keyword|domain-keyword|geosite|geoip|cidr|ip-cidr|ip):/i.test(tok);
      const cond = k !== "auto" && !hasPrefix ? k + ":" + tok : tok;
      const c = RULES.parseCond(cond);
      const ok = c.kind === "geosite" || c.kind === "geoip" ? reCat.test(c.value) : c.kind === "cidr" ? reNet.test(c.value) : c.kind === "keyword" ? c.value.length > 0 && c.value.length <= 64 : reHost.test(c.value);
      if (!ok) { bad.push(tok); continue; }
      if (RULES.hasCond(draft, c.kind + ":" + c.value)) { dup.push(tok); continue; }
      RULES.addCond(draft, c.kind + ":" + c.value); added++;
    }
    if (added) { ui.touched = true; drawConds(); drawTile(); }
    valInp.value = bad.join(" ");
    showErr(bad.length ? t("rtCondBad", { v: bad.join(", ") }) : dup.length ? t("rtCondDup", { v: dup.join(", ") }) : "");
    dl.replaceChildren();
    if (!added || bad.length) valInp.focus();
  }
  // подсказки категорий: только для geosite/geoip и только когда базы загружены
  let sugTimer = 0, sugSeq = 0;
  function suggest() {
    clearTimeout(sugTimer);
    sugTimer = setTimeout(async () => {
      const raw = valInp.value, at = raw.search(/[^\s,;]*$/), head = raw.slice(0, at);
      let tail = raw.slice(at), kind = kindSel.value, pre = "";
      const mm = /^(geosite|geoip):(.*)$/i.exec(tail);
      if (mm) { kind = mm[1].toLowerCase(); tail = mm[2]; pre = kind + ":"; } else if (kind === "geosite" || kind === "geoip") pre = "";
      if ((kind !== "geosite" && kind !== "geoip") || (!mm && kindSel.value === "auto") || tail.length < 2) { dl.replaceChildren(); return; }
      const seq = ++sugSeq;
      try {
        const r = await api("GET", "/api/geo/categories?kind=" + kind + "&q=" + encodeURIComponent(tail));
        if (seq !== sugSeq) return;
        dl.replaceChildren(...(r.items || []).map((x) => h("option", { value: head + pre + x })));
      } catch (_) { dl.replaceChildren(); }
    }, 200);
  }

  const destSel = h("select", { id: "rt-dest", onchange: (e) => { ui.dest = e.target.value; } }, rtDestOptions(ui.dest));

  // сужение: устройства, порты, протокол (одно на всё правило)
  const sc0 = b ? RULES.scope(b) : { src: [], ports: [], net: [] };
  const scMixed = !sc0;
  const sc = sc0 || (b && b.lines[0] ? { src: b.lines[0].src.slice(), ports: b.lines[0].ports.slice(), net: b.lines[0].net.slice() } : { src: [], ports: [], net: [] });
  ui.scope = { src: sc.src.slice(), ports: sc.ports.slice(), net: sc.net.slice() };
  ui.allDevs = !ui.scope.src.length;
  const devs = (S.dev && S.dev.devices) || [];
  const devBox = h("div", { class: "rt-scope-devs" });
  const devFind = h("input", { type: "search", "aria-label": t("rtScopeSearch"), placeholder: t("rtScopeSearch"), autocomplete: "off", oninput: () => drawDevs() });
  const allSw = h("button", { type: "button", class: "rt-switch", role: "switch", "aria-checked": String(ui.allDevs), "aria-label": t("rtScopeAll"),
    onclick: () => { ui.allDevs = !ui.allDevs; allSw.setAttribute("aria-checked", String(ui.allDevs)); if (ui.allDevs) ui.scope.src = []; drawDevs(); showErr(""); } },
  h("span", { class: "rt-knob" }));
  function drawDevs() {
    if (ui.allDevs) { devBox.replaceChildren(); devBox.hidden = true; return; }
    devBox.hidden = false;
    if (!devs.length && !ui.scope.src.length) {
      devBox.replaceChildren(h("p", { class: "hint" }, t("rtScopeNoDevices"), " ", h("a", { href: "#/devices", onclick: () => close() }, t("rtScopeGoDevices"))));
      return;
    }
    const q = devFind.value.trim().toLowerCase();
    const raw = ui.scope.src.filter((r) => r.charAt(0) !== "@" || !devByRef(r));
    const items = devs.map((d) => ({ ref: "@" + d.id, name: d.name, addr: d.addr, kind: d.kind })).concat(raw.map((r) => ({ ref: r, name: r, addr: r.charAt(0) === "@" ? "?" : "", kind: "other" })));
    const toggle = (ref, on) => { ui.scope.src = ui.scope.src.filter((x) => x !== ref); if (on) ui.scope.src.push(ref); drawDevs(); showErr(""); };
    const chosen = ui.scope.src.map((r) => items.find((i) => i.ref === r)).filter(Boolean);
    devBox.replaceChildren(
      chosen.length ? h("div", { class: "rt-scope-chosen" }, chosen.map((i) => h("span", { class: "rt-chip rt-devchip" }, devIcon(i.kind, 14), i.name,
        h("button", { type: "button", class: "chip-x", "aria-label": t("devRemove", { name: i.name }), onclick: () => toggle(i.ref, false) }, icon("x", 12))))) : null,
      h("div", { class: "rt-scope-grid" },
        h("div", {}, h("div", { class: "field" }, devFind),
          h("ul", { class: "rt-scope-list" }, items.filter((i) => !q || (i.name + " " + i.addr).toLowerCase().includes(q)).map((i) => {
            const on = ui.scope.src.includes(i.ref);
            return h("li", {}, h("label", { class: on ? "is-on" : "" },
              h("input", { type: "checkbox", checked: on, onchange: (e) => toggle(i.ref, e.target.checked) }),
              devIcon(i.kind, 16), h("span", { class: "nm" }, i.name), h("span", { class: "mono ad" }, i.addr)));
          }))),
        h("p", { class: "hint rt-scope-count" }, t("rtScopeChosen", { n: ui.scope.src.length }), " · ",
          h("button", { type: "button", class: "linklike", onclick: () => { ui.scope.src = []; drawDevs(); } }, t("rtScopeReset")))));
  }
  const portsInp = h("input", { id: "rt-ports", type: "text", class: "mono", value: ui.scope.ports.join(", "), placeholder: "443, 27000-27100", autocomplete: "off", spellcheck: "false",
    oninput: () => showErr("") });
  const netKey = (n) => (n.length === 2 ? "both" : n[0] || "any");
  const netSel = h("select", { id: "rt-net" }, ["any", "tcp", "udp", "both"].map((k) => h("option", { value: k, selected: netKey(ui.scope.net) === k }, t("rtNet" + k[0].toUpperCase() + k.slice(1)))));
  const readScope = () => {
    const ports = portsInp.value.split(/[\s,;]+/).filter(Boolean);
    for (const p of ports) {
      const m = /^(\d{1,5})(?:-(\d{1,5}))?$/.exec(p);
      if (!m || +m[1] < 1 || +(m[2] || m[1]) > 65535 || +m[1] > +(m[2] || m[1])) { showErr(t("rtPortsBad")); portsInp.focus(); return null; }
    }
    if (!ui.allDevs && !ui.scope.src.length) { showErr(t("rtScopeEmpty")); return null; }
    const nv = netSel.value;
    return { src: ui.allDevs ? [] : ui.scope.src.slice(), ports, net: nv === "any" ? [] : nv === "both" ? ["tcp", "udp"] : [nv] };
  };

  const commit = () => {
    if (!RULES.conds(draft).length) { showErr(t("rtCondNeed")); valInp.focus(); return; }
    if (valInp.value.trim()) { addConds(); if (!err.hidden) return; }
    const nsc = readScope();
    if (!nsc) return;
    if (ui.dest !== "mixed" && (isNew || ui.dest !== RULES.dest(b))) RULES.setDest(draft, ui.dest);
    const tmp = { lines: [{ src: nsc.src, ports: nsc.ports, net: nsc.net }] };
    if (scMixed || RULES.modsText(tmp.lines[0]) !== RULES.modsText(sc)) RULES.setScope(draft, nsc);
    const name = draft.name.trim() || rtInfo({ ...draft, name: "" }).name;
    let target;
    if (isNew) {
      target = Object.assign(draft, { name, hraw: null });
      RULES.add(S.rt.model, target);
    } else {
      const changed = name !== info.name || draft.icon !== (info.svc ? info.svc.id : b.icon);
      target = Object.assign(b, { name: changed ? name : b.name, icon: draft.icon, lines: draft.lines, head: b.head || changed, hraw: changed ? null : b.hraw });
    }
    close();
    rtChanged(rtUid(target) + ":grip");
    if (isNew) { announce(t("rtAdded", { name })); const row = rtEls.list.querySelector('[data-uid="' + rtUid(target) + '"]'); if (row) row.scrollIntoView({ block: "nearest" }); }
  };

  dlg.append(h("form", { class: "rt-dlg", novalidate: true, onsubmit: (e) => { e.preventDefault(); commit(); } },
    h("header", {}, h("h2", { id: "rt-dlg-title" }, isNew ? t("rtDlgAdd") : t("rtDlgEdit", { name: info.name })),
      h("button", { type: "button", class: "icon-btn", "aria-label": t("close"), onclick: close }, icon("x", 18))),
    h("div", { class: "rt-dlg-body" },
      h("section", {}, h("h3", {}, t("rtCatalog")), catHint, h("div", { class: "field" }, find), grid),
      h("section", {},
        h("div", { class: "rt-name-row" }, tileSlot, h("div", { class: "field" }, h("label", { for: "rt-name" }, t("rtName")), nameInp)),
        h("h3", {}, t("rtConds")), list,
        h("div", { class: "rt-add" },
          h("div", { class: "field" }, h("label", { for: "rt-kind" }, t("rtKind")), kindSel),
          h("div", { class: "field grow" }, h("label", { for: "rt-val" }, t("rtCondValue")), valInp, dl),
          h("button", { type: "button", class: "btn btn-secondary", onclick: addConds }, icon("plus", 16), t("rtCondAdd"))),
        h("p", { class: "hint" }, t("rtCondHint")),
        err,
        h("div", { class: "field" }, h("label", { for: "rt-dest" }, t("rtDest")), destSel),
        h("div", { class: "rt-scope" },
          h("div", { class: "rt-scope-head" }, h("h3", {}, t("rtScopeTitle")), h("span", { class: "rt-scope-all" }, t("rtScopeAll"), allSw)),
          scMixed && h("p", { class: "hint" }, t("rtScopeMixed")),
          devBox,
          h("div", { class: "rt-scope-extra" },
            h("div", { class: "field" }, h("label", { for: "rt-ports" }, t("rtPorts")), portsInp, h("div", { class: "hint" }, t("rtPortsHint"))),
            h("div", { class: "field" }, h("label", { for: "rt-net" }, t("rtNet")), netSel)),
          h("p", { class: "hint" }, t("rtScopeNote"))))),
    h("footer", { class: "actions" },
      h("button", { type: "button", class: "btn btn-secondary", onclick: close }, t("rtCancel")),
      h("button", { type: "submit", class: "btn btn-primary" }, icon("check", 16), t("rtDone")))));
  document.body.append(dlg);
  drawConds(); drawTile(); drawDevs();
  dlg.showModal();
  if (window.matchMedia("(pointer: fine)").matches) (isNew ? find : nameInp).focus();
}

// Номер сохранения: ответ фонового GET, начатого до сохранения, устарел и страницу не трогает.
let polGen = 0;
const rtDialogOpen = () => !!document.querySelector("dialog.rt-dialog[open]");

async function loadPolicy(quiet) {
  const first = !S.polEdit, gen = polGen;
  try {
    const v = await api("GET", "/api/policy");
    if (gen !== polGen) return;
    S.pol = v;
    // Пока открыт диалог сервиса или есть несохранённые правки, модель правил не пересоздаётся:
    // диалог держит ссылку на неё, и добавленное в него пропало бы.
    if (!S.polEdit || !(S.dirty || rtDialogOpen())) { S.polEdit = { ...S.pol.settings }; rtReset(); }
    if (S.importRules != null && S.route.page === "policy") {
      // правила из файла («Инструменты» → «Импорт»): в редактор, сохраняет человек
      S.polEdit.rules = S.importRules; S.importRules = null; rtReset(); S.dirty = true;
      say("ok", t("toolImported")); renderMain(); rtBar();
    }
  } catch (e) {
    if (!quiet) { say("error", e.message); renderFlash(); }
  }
  if (first || !S.grp) try { S.grp = await api("GET", "/api/groups"); } catch (_) { /* группы нужны только для списка «Куда» */ }
  if (first || !S.dev) try { S.dev = await api("GET", "/api/devices"); } catch (_) { /* устройства нужны только для правил по устройствам */ }
  if (S.route.page === "policy" && !S.dirty && !S.busy && !rtTyping() && !rtDialogOpen()) renderMain();
  if (S.route.page === "tools" && !S.dirty && !S.busy && !toolsTyping()) renderMain();
}
const toolsTyping = () => { const a = document.activeElement; return !!(a && mainEl && mainEl.contains(a) && /^(INPUT|SELECT|TEXTAREA)$/.test(a.tagName)); };

/* ---------- журнал ---------- */
// Записи приходят порциями после последнего номера; список перерисовывается отдельно от
// страницы, чтобы поиск и прокрутка не сбивались при каждом опросе.
const LOG_KEEP = 3000, LOG_SHOW = 800;
function logState() {
  if (!S.log) S.log = { entries: [], last: 0, level: "all", q: "", follow: true, loaded: false };
  return S.log;
}
async function loadLog(quiet) {
  const L = logState();
  if (L.busy) return; // опрос и переход на страницу не должны забрать одну порцию дважды
  L.busy = true;
  let r;
  try { r = await api("GET", "/api/log?after=" + L.last); } catch (e) { if (!quiet) { say("error", e.message); renderFlash(); } return; } finally { L.busy = false; }
  if (r.last < L.last) { L.entries = []; L.last = 0; return loadLog(quiet); } // контейнер перезапустился
  const first = !L.loaded;
  L.loaded = true;
  const fresh = r.entries.filter((e) => e.seq > L.last);
  if (fresh.length) L.entries = L.entries.concat(fresh).slice(-LOG_KEEP);
  L.last = Math.max(L.last, r.last);
  r.entries = fresh;
  if (S.route.page !== "log") return;
  if (first || !document.getElementById("log-list")) renderMain();
  else if (r.entries.length) logDraw();
}
const logMatch = (e, L) => (L.level === "all" || (L.level === "warn" ? e.level !== "info" : e.level === "error")) &&
  (!L.q || e.text.toLowerCase().includes(L.q.toLowerCase()));
function logTimeText(iso) {
  const d = new Date(iso), now = new Date();
  const p = (n) => String(n).padStart(2, "0");
  const time = p(d.getHours()) + ":" + p(d.getMinutes()) + ":" + p(d.getSeconds());
  return d.toDateString() === now.toDateString() ? time : p(d.getDate()) + "." + p(d.getMonth() + 1) + " " + time;
}
function logDraw() {
  const L = logState(), box = document.getElementById("log-list"), cnt = document.getElementById("log-count");
  if (!box) return;
  const all = L.entries.filter((e) => logMatch(e, L));
  const shown = all.slice(-LOG_SHOW);
  const atBottom = box.scrollHeight - box.scrollTop - box.clientHeight < 40;
  box.replaceChildren(...(shown.length ? shown.map((e) => h("div", { class: "log-row lvl-" + e.level },
    h("span", { class: "log-time mono", title: new Date(e.t).toLocaleString() }, logTimeText(e.t)),
    h("span", { class: "log-lvl" }, t("logLevel_" + e.level)),
    h("span", { class: "log-text mono" }, e.text)))
    : [h("p", { class: "hint log-empty" }, t(L.entries.length ? "logNoMatch" : "logEmpty"))]));
  if (cnt) cnt.textContent = all.length > shown.length ? t("logShown", { n: shown.length, all: all.length }) : "";
  if (L.follow && (atBottom || !box.dataset.drawn)) box.scrollTop = box.scrollHeight;
  box.dataset.drawn = "1";
}
function pageLog() {
  const L = logState();
  if (!L.loaded) { loadLog(true); return h("div", {}, heading(t("navSystem"), t("logTitle")), h("div", { class: "loading" }, t("loading"))); }
  const chip = (key, label) => h("button", { class: "filter-chip", type: "button", "aria-pressed": String(L.level === key),
    onclick: () => { L.level = key; for (const b of document.querySelectorAll("#log-levels .filter-chip")) b.setAttribute("aria-pressed", String(b.dataset.k === key)); logDraw(); }, "data-k": key }, label);
  const page = h("div", {},
    heading(t("navSystem"), t("logTitle"), t("logSub"),
      h("a", { class: "btn btn-secondary", href: "/api/log/download", download: "" }, icon("download", 16), t("logDownload"))),
    h("div", { class: "card log-card" },
      h("div", { class: "log-toolbar" },
        h("div", { class: "filter-chips", id: "log-levels", role: "group", "aria-label": t("logLevel") },
          chip("all", t("logAll")), chip("warn", t("logWarn")), chip("error", t("logErr"))),
        h("label", { class: "log-search" }, icon("search", 16),
          h("input", { type: "search", value: L.q, placeholder: t("logSearch"), "aria-label": t("logSearch"), "data-nodirty": "", spellcheck: "false",
            oninput: (e) => { L.q = e.target.value; logDraw(); } })),
        h("label", { class: "check log-follow" }, h("input", { type: "checkbox", checked: L.follow, "data-nodirty": "",
          onchange: (e) => { L.follow = e.target.checked; if (L.follow) logDraw(); } }), h("span", {}, t("logFollow")))),
      h("div", { class: "log-list", id: "log-list", role: "log", "aria-live": "polite", tabindex: "0" }),
      h("div", { class: "hint log-count", id: "log-count" })));
  setTimeout(logDraw, 0);
  return page;
}

/* ---------- инструменты ---------- */
function pageTools() {
  if (!S.pol) { loadPolicy(); return h("div", {}, heading(t("navSystem"), t("toolTitle")), h("div", { class: "loading" }, t("loading"))); }
  const v = S.pol, g = v.geo || {}, devs = (S.dev && S.dev.devices) || [];
  if (!S.extraEdit) S.extraEdit = (v.settings.geo_extra || []).map((x) => ({ ...x }));
  const ex = S.extraEdit, exStat = {};
  for (const x of v.geo_extra || []) exStat[x.kind + ":" + x.tag] = x;
  const dirty = () => { S.dirty = true; };
  const host = (u) => { try { return new URL(u).host; } catch (_) { return u || "—"; } };
  const geoLine = (label, f, n) => h("div", { class: "tool-geo" },
    h("div", {}, h("b", { class: "mono" }, label), " ", f && f.present ? h("span", { class: "pill pill-on" }, t("polGeoHave")) : h("span", { class: "pill pill-idle" }, t("polGeoNone")),
      f && f.present ? h("span", { class: "muted" }, " " + fmtBytes(f.size) + " · " + fmtDate(f.updated)) : null),
    h("div", { class: "mono muted tool-src" }, f && f.source ? host(f.source) : "—", n ? " · " + t("toolLoadedCats", { n }) : ""));
  const updateGeo = () => act(async () => { polGen++; S.pol = await api("POST", "/api/geo/update"); S.polEdit = { ...S.pol.settings }; rtReset(); S.extraEdit = null; }, t("polGeoUpdated"));
  const saveExtra = () => act(async () => {
    const st = { ...S.pol.settings, geo_extra: ex.map((x) => ({ tag: String(x.tag || "").trim().toLowerCase(), url: String(x.url || "").trim(), kind: x.kind })) };
    try { S.pol = await api("PUT", "/api/policy", { ...st, geo_update_hours: parseInt(st.geo_update_hours, 10) || 0 }); } catch (e) { throw new Error(e.data && e.data.errors ? e.data.errors.join(" · ") : e.message); }
    polGen++; S.pol = await api("POST", "/api/geo/update"); S.polEdit = { ...S.pol.settings }; rtReset(); S.extraEdit = null;
  }, t("toolExtraSaved"));
  const exRow = (x, i) => {
    const st = exStat[x.kind + ":" + x.tag];
    return h("div", { class: "tool-extra" },
      h("input", { type: "text", class: "mono", value: x.url || "", placeholder: "https://…/geosite.dat", "aria-label": "URL", spellcheck: "false", oninput: (e) => { x.url = e.target.value; dirty(); } }),
      h("select", { "aria-label": t("toolKind"), onchange: (e) => { x.kind = e.target.value; dirty(); } }, ["geosite", "geoip"].map((k) => h("option", { value: k, selected: x.kind === k }, k === "geosite" ? "GeoSite" : "GeoIP"))),
      h("input", { type: "text", class: "mono", value: x.tag || "", placeholder: t("toolTag"), "aria-label": t("toolTag"), maxlength: "24", spellcheck: "false", oninput: (e) => { x.tag = e.target.value; dirty(); renderSyntax(); } }),
      h("span", { class: "tool-extra-st" }, st && st.file && st.file.present ? [h("span", { class: "pill pill-on" }, t("polGeoHave")), h("span", { class: "muted" }, " " + fmtBytes(st.file.size) + " · " + t("toolCats", { n: st.categories }))] : h("span", { class: "pill pill-idle" }, t("toolNotLoaded"))),
      h("button", { type: "button", class: "icon-btn", "aria-label": t("delete"), title: t("delete"), onclick: () => { ex.splice(i, 1); dirty(); renderMain(); } }, icon("trash", 16)));
  };
  const syntax = h("p", { class: "hint" });
  const renderSyntax = () => { const x = ex[0]; syntax.textContent = t("toolExtraHint", { ex: (x && x.kind) || "geosite", tag: (x && x.tag) || "slim" }); };
  renderSyntax();
  // тест маршрута
  const tt = S.toolTest;
  const tq = h("input", { id: "tt-q", type: "text", class: "mono", placeholder: "www.youtube.com / 77.88.8.8", spellcheck: "false", "data-nodirty": "" });
  const tdev = h("select", { id: "tt-dev", "data-nodirty": "" }, h("option", { value: "" }, t("toolAnyDevice")), devs.map((d) => h("option", { value: "@" + d.id }, d.name + " · " + d.addr)));
  const tport = h("input", { id: "tt-port", type: "number", min: "0", max: "65535", placeholder: "443", "data-nodirty": "" });
  const tproto = h("select", { id: "tt-proto", "data-nodirty": "" }, h("option", { value: "tcp" }, "TCP"), h("option", { value: "udp" }, "UDP"));
  const runTest = async () => {
    const body = { query: tq.value };
    if (tdev.value || tport.value) { body.src = tdev.value; body.port = parseInt(tport.value, 10) || 0; body.proto = tproto.value; }
    try { S.toolTest = await api("POST", "/api/policy/test", body); } catch (e) { S.toolTest = { error: e.message }; }
    S.toolTestIn = { q: tq.value, dev: tdev.value, port: tport.value, proto: tproto.value };
    renderMain();
  };
  if (S.toolTestIn) { tq.value = S.toolTestIn.q; tdev.value = S.toolTestIn.dev; tport.value = S.toolTestIn.port; tproto.value = S.toolTestIn.proto; }
  const actLabel = (a) => (a ? t("polAct_" + String(a).toLowerCase()) : "—");
  // импорт правил и восстановление копии
  const fileImport = h("input", { type: "file", accept: ".txt,.list,.conf,text/plain", class: "sr-only", id: "tool-import", "data-nodirty": "",
    onchange: async (e) => {
      const f = e.target.files[0]; if (!f) return;
      if (f.size > 64 * 1024) { say("error", t("toolTooBig")); renderFlash(); return; }
      S.importRules = await f.text(); location.hash = "#/policy";
    } });
  const restoreFile = h("input", { type: "file", accept: ".json,application/json", id: "tool-restore", "data-nodirty": "", onchange: (e) => { S.restoreFile = e.target.files[0] || null; renderMain(); } });
  const restorePw = h("input", { type: "password", id: "tool-restore-pw", autocomplete: "current-password", "data-nodirty": "" });
  const restore = () => act(async () => {
    if (!S.restoreFile) throw new Error(t("toolPickFile"));
    let backup;
    try { backup = JSON.parse(await S.restoreFile.text()); } catch (_) { throw new Error(t("toolNotBackup")); }
    const r = await api("POST", "/api/backup/restore", { password: restorePw.value, backup });
    S.restoreFile = null; S.pol = null; S.dev = null; S.grp = null; S.extraEdit = null;
    await refresh(true);
    return r;
  }, t("toolRestored"));
  return h("div", {},
    heading(t("navSystem"), t("toolTitle"), t("toolSub")),
    h("div", { class: "tools-grid" },
      h("div", {},
        h("div", { class: "card" }, h("div", { class: "heading-row" }, h("h2", {}, t("toolGeoTitle")), h("a", { class: "card-link", href: "#/policy" }, t("toolGeoSources") + " →")),
          geoLine("geosite.dat", g.geosite, Object.keys(g.site_loaded || {}).length), geoLine("geoip.dat", g.geoip, Object.keys(g.ip_loaded || {}).length),
          g.last_error && h("p", { class: "info-line" }, icon("alert", 16), h("span", {}, g.last_error)),
          h("div", { class: "actions" }, h("button", { class: "btn btn-primary", type: "button", disabled: S.busy, onclick: updateGeo }, icon("restart", 16), t("polGeoUpdate")))),
        h("div", { class: "card" }, h("h2", {}, t("toolExtraTitle")),
          h("p", { class: "hint" }, t("toolExtraSub")),
          ex.length ? h("div", { class: "tool-extra-list" }, ex.map(exRow)) : h("p", { class: "hint" }, t("toolExtraEmpty")),
          syntax,
          h("div", { class: "actions" },
            h("button", { class: "btn btn-secondary", type: "button", disabled: ex.length >= 8, onclick: () => { ex.push({ url: "", kind: "geosite", tag: "" }); dirty(); renderMain(); } }, icon("plus", 16), t("toolExtraAdd")),
            h("button", { class: "btn btn-primary", type: "button", disabled: S.busy, onclick: saveExtra }, icon("download", 16), t("toolExtraSave"))))),
      h("div", {},
        h("div", { class: "card" }, h("h2", {}, t("toolTestTitle")),
          h("form", { "data-nodirty": "", onsubmit: (e) => { e.preventDefault(); runTest(); } },
            h("div", { class: "field" }, h("label", { for: "tt-q" }, t("polTestLabel")), tq),
            h("div", { class: "tool-test-row" },
              h("div", { class: "field" }, h("label", { for: "tt-dev" }, t("toolDevice")), tdev),
              h("div", { class: "field" }, h("label", { for: "tt-port" }, t("toolPort")), tport),
              h("div", { class: "field" }, h("label", { for: "tt-proto" }, t("rtNet")), tproto)),
            h("div", { class: "actions" }, h("button", { class: "btn btn-secondary", type: "submit" }, icon("arrow", 16), t("polTestBtn")))),
          tt && (tt.error ? h("p", { class: "info-line" }, icon("alert", 16), h("span", {}, tt.error))
            : h("div", { class: "tool-result d-" + String(tt.action || "").toLowerCase() },
              h("b", {}, tt.query), " → ", h("b", {}, actLabel(tt.action) + (tt.target ? " (" + tt.target + ")" : "")),
              h("div", { class: "muted" }, tt.rule ? t("toolByRule", { name: tt.rule, by: tt.matched_by }) : t("polBy") + " " + (tt.matched_by === "default" ? t("monDefault") : tt.matched_by))))),
        h("div", { class: "card" }, h("h2", {}, t("toolRulesTitle")),
          h("p", { class: "hint" }, t("toolRulesSub")),
          h("div", { class: "actions" },
            h("a", { class: "btn btn-secondary", href: "/api/policy/export", download: "" }, icon("download", 16), t("toolExport")),
            h("label", { class: "btn btn-secondary", for: "tool-import" }, icon("upload", 16), t("toolImport")), fileImport)),
        h("div", { class: "card" }, h("h2", {}, t("toolBackupTitle")),
          h("p", { class: "info-line" }, icon("alert", 16), h("span", {}, t("toolBackupWarn"))),
          h("div", { class: "actions" }, h("a", { class: "btn btn-primary", href: "/api/backup", download: "" }, icon("download", 16), t("toolBackupGet"))),
          h("h3", { class: "tool-sub" }, t("toolRestoreTitle")),
          h("p", { class: "hint" }, t("toolRestoreSub")),
          h("div", { class: "field" }, h("label", { for: "tool-restore" }, t("toolRestoreFile")), restoreFile),
          h("div", { class: "field" }, h("label", { for: "tool-restore-pw" }, t("pwCurrent")), restorePw),
          h("div", { class: "actions" }, h("button", { class: "btn btn-danger", type: "button", disabled: S.busy || !S.restoreFile, onclick: restore }, icon("restart", 16), t("toolRestore")))))));
}

function pagePolicy() {
  if (!S.pol || !S.polEdit) { loadPolicy(); return h("div", {}, heading(t("navInfra"), t("polTitle")), h("div", { class: "loading" }, t("loading"))); }
  if (!S.rt) rtReset();
  const v = S.pol, st = v.stats || {}, ed = S.polEdit, g = v.geo || {}, R = S.rt;
  const dirty = rtDirty;
  const text = (id, key) => h("input", { id, type: "text", class: "mono", value: ed[key] || "", spellcheck: "false",
    oninput: (e) => { ed[key] = e.target.value; dirty(); } });
  const updateGeo = () => act(async () => { polGen++; S.pol = await api("POST", "/api/geo/update"); S.polEdit = { ...S.pol.settings }; rtReset(); }, t("polGeoUpdated"));
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
  const presetNote = h("p", { class: "info-line rt-retag", hidden: true });

  const master = h("button", { type: "button", class: "rt-switch", role: "switch", "aria-checked": String(!!ed.enabled), "aria-labelledby": "rt-master-l",
    onclick: (e) => { ed.enabled = !ed.enabled; e.currentTarget.setAttribute("aria-checked", String(!!ed.enabled)); dirty(); } }, h("span", { class: "rt-knob" }), h("span", { class: "sr-only" }, t("polEnable")));
  const search = h("input", { id: "rt-q", type: "search", "data-keep": "", placeholder: t("rtSearch"), "aria-label": t("rtSearch"), autocomplete: "off", value: R.q,
    oninput: (e) => { R.q = e.target.value; rtDraw(); clearTimeout(rtSayTimer); rtSayTimer = setTimeout(() => announce(t("shownOf", { n: rtEls.list.querySelectorAll(".rt-row").length, m: RULES.blocks(R.model).length })), 500); } });
  rtEls.chips = h("div", { class: "rt-tiles", role: "group", "aria-label": t("rtSearch") });
  rtEls.notes = h("div", { class: "rt-notes" });
  rtEls.list = h("ol", { class: "rt-list", role: "list" });
  rtEls.bar = h("div", { class: "save-bar", hidden: !S.dirty });

  const area = h("textarea", { id: "pol-rules", spellcheck: "false", placeholder: "#@ YouTube | youtube\nPROXY geosite:youtube\n\nDIRECT geoip:ru\nPROXY@awg1 domain:example.com\nDROP keyword:ads",
    oninput: (e) => { ed.rules = e.target.value; dirty(); }, onchange: () => { R.model = RULES.parse(ed.rules || ""); rtDraw(); } }, ed.rules || "");

  const page = h("div", {},
    heading(t("navInfra"), t("polTitle"), t("polSub")),
    !v.gateway && h("div", { class: "flash warn", role: "status" }, icon("info", 18), h("div", { class: "msg" }, t("polNoGateway"))),
    h("div", { class: "grid-stats" },
      statCard(t("polState"), st.enabled ? t("netOn") : t("netOff"), t("polDefault") + ": " + actName(st.default_action)),
      statCard(t("polRules"), String(st.rules || 0), t("polRoutes", { n: st.routes || 0 })),
      statCard(t("polLearned"), String(st.learned || 0), t("polLearnedNote")),
      statCard(t("polQueries"), String(st.queries || 0), v.dns ? t("polDnsNote", { p: st.proxy || 0, d: st.direct || 0, x: st.dropped || 0 }) : t("polDnsOff"))),
    st.last_error && h("div", { class: "flash error", role: "alert" }, icon("alert", 18), h("div", { class: "msg" }, st.last_error)),
    h("div", { class: "card" },
      h("div", { class: "heading-row" }, h("h2", {}, t("polRulesTitle")),
        h("button", { id: "rt-add-btn", class: "btn btn-primary", type: "button", onclick: () => rtDialog(null) }, icon("plus", 16), t("rtAdd"))),
      h("div", { class: "rt-top" },
        h("div", { class: "rt-master" }, master, h("div", {}, h("b", { id: "rt-master-l" }, t("polEnable")), h("div", { class: "hint" }, t("polEnableHint")))),
        h("div", { class: "field" }, h("label", { for: "pol-def" }, t("polDefault")),
          h("select", { id: "pol-def", "data-keep": "", onchange: (e) => { ed.default_action = e.target.value; dirty(); } },
            ["proxy", "direct"].map((o) => h("option", { value: o, selected: ed.default_action === o }, actName(o)))))),
      rtEls.chips,
      h("div", { class: "rt-tools" }, h("div", { class: "field" }, search)),
      rtEls.notes,
      h("div", { class: "rt-head", "aria-hidden": "true" }, h("span", {}), h("span", {}, t("rtColService")), h("span", {}, t("rtColConds")), h("span", {}, t("rtColDevices")), h("span", {}, t("rtColDest")), h("span", {}, t("rtColOn")), h("span", {})),
      rtEls.list,
      h("p", { class: "hint rt-order" }, t("rtOrderHint")),
      h("details", { class: "rt-adv", open: R.adv, ontoggle: (e) => { R.adv = e.currentTarget.open; const sub = e.currentTarget.querySelector(".rt-adv-sub"); if (sub) sub.textContent = t(R.adv ? "rtAdvHide" : "rtAdvShow"); } },
        h("summary", {}, icon("chevron", 16), icon("file", 16), t("rtAdv"), h("span", { class: "rt-adv-sub" }, t(R.adv ? "rtAdvHide" : "rtAdvShow"))),
        h("p", { class: "hint" }, t("rtAdvHint")),
        h("div", { class: "field" }, area, h("div", { class: "hint" }, t("polRulesHint"))))),
    h("div", { class: "card" }, h("h2", {}, t("rtSettings")),
      h("form", { onsubmit: (e) => { e.preventDefault(); rtSave(); } },
        h("div", { class: "fields-grid" },
          h("div", { class: "field" }, h("label", { for: "pol-pdns" }, t("polProxyDNS")), text("pol-pdns", "proxy_dns"), h("div", { class: "hint" }, t("polDnsHint"))),
          h("div", { class: "field" }, h("label", { for: "pol-ddns" }, t("polDirectDNS")), text("pol-ddns", "direct_dns")),
          h("div", { class: "field" }, h("label", { for: "pol-gs" }, "geosite.dat"), text("pol-gs", "geosite_url")),
          h("div", { class: "field" }, h("label", { for: "pol-gi" }, "geoip.dat"), text("pol-gi", "geoip_url")),
          h("div", { class: "field" }, h("label", { for: "pol-upd" }, t("polGeoEvery")),
            h("input", { id: "pol-upd", type: "number", min: "0", max: "720", value: String(ed.geo_update_hours ?? 24), oninput: (e) => { ed.geo_update_hours = e.target.value; dirty(); } }),
            h("div", { class: "hint" }, t("polGeoEveryHint")))),
        h("div", { class: "rt-presets" }, h("span", { class: "rt-presets-l" }, t("rtPresetLabel")),
          GEO_PRESET_KEYS.map((k) => h("button", { type: "button", class: "filter-chip",
            "aria-pressed": String(ed.geosite_url === GEO_PRESETS[k][0] && ed.geoip_url === GEO_PRESETS[k][1]),
            onclick: () => { ed.geosite_url = GEO_PRESETS[k][0]; ed.geoip_url = GEO_PRESETS[k][1]; document.getElementById("pol-gs").value = ed.geosite_url; document.getElementById("pol-gi").value = ed.geoip_url; rtDirty(); renderPresets(); } },
          t({ loyal: "rtPresetLoyal", iplist: "rtPresetIplist", runet: "rtPresetRunet" }[k])))),
        presetNote,
        h("p", { class: "hint" }, t("rtPresetHint")))),
    h("div", { class: "card" }, h("h2", {}, t("polTestTitle")),
      h("form", { "data-nodirty": "", onsubmit: (e) => { e.preventDefault(); runTest(); } },
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
      h("div", { class: "actions" }, h("button", { class: "btn btn-secondary", type: "button", disabled: S.busy, onclick: updateGeo }, icon("restart", 16), t("polGeoUpdate")))),
    rtEls.bar);
  // Переключатели источников подсвечиваются по текущим ссылкам.
  function renderPresets() {
    page.querySelectorAll(".rt-presets .filter-chip").forEach((el, i) => { const k = GEO_PRESET_KEYS[i]; el.setAttribute("aria-pressed", String(ed.geosite_url === GEO_PRESETS[k][0] && ed.geoip_url === GEO_PRESETS[k][1])); });
    drawPresetNote();
  }
  // Правила записаны тегами другого источника: предложить заменить их одной кнопкой.
  function drawPresetNote() {
    // у runetfreedom теги те же, что у Loyalsoldier
    const loyal = ["loyal", "runet"].some((k) => ed.geosite_url === GEO_PRESETS[k][0] && ed.geoip_url === GEO_PRESETS[k][1]);
    const ipl = ed.geosite_url === GEO_PRESETS.iplist[0] && ed.geoip_url === GEO_PRESETS.iplist[1];
    const n = loyal || ipl ? retag(ipl, false) : 0;
    if (!n) { presetNote.replaceChildren(); presetNote.hidden = true; return; }
    presetNote.hidden = false;
    presetNote.replaceChildren(icon("info", 16), h("span", {}, t(ipl ? "rtRetagToIpl" : "rtRetagToLoyal", { n })),
      h("button", { type: "button", class: "btn btn-secondary btn-small", onclick: () => {
        const done = retag(ipl, true);
        rtChanged(); drawPresetNote();
        say("ok", t("rtRetagDone", { n: done })); renderFlash();
      } }, t("rtRetagBtn")));
  }
  drawPresetNote();
  rtDraw(); rtBar();
  return page;
}

/* ---------- мониторинг ---------- */
async function loadMonitor(quiet) {
  const win = S.monWin || 900;
  try { S.mon = await api("GET", "/api/monitor?window=" + win); } catch (e) { if (!quiet) { say("error", e.message); renderFlash(); } return; }
  if (S.route.page === "monitor" || S.route.page === "overview") { if (!S.busy && !document.querySelector(".row-menu[open]") && !S.monHover) renderMain(); }
  renderSideStatus();
}
async function loadFlows(quiet) {
  try { S.flows = await api("GET", "/api/monitor/flows"); } catch (e) { if (!quiet) { say("error", e.message); renderFlash(); } return; }
  if ((S.route.page === "monitor" || S.route.page === "overview") && !S.busy && !S.monHover && !document.querySelector(".row-menu[open]")) renderMain();
  if (S.route.page === "policy" && !S.dirty && !S.busy && !rtTyping() && !rtDialogOpen()) rtDraw();
}
const fmtRate = (bps) => {
  const bits = (Number(bps) || 0) * 8;
  const u = lang === "ru" ? ["бит/с", "Кбит/с", "Мбит/с", "Гбит/с"] : ["bit/s", "kbit/s", "Mbit/s", "Gbit/s"];
  let v = bits, i = 0;
  while (v >= 1000 && i < u.length - 1) { v /= 1000; i++; }
  return (i ? v.toFixed(v < 10 ? 1 : 0) : Math.round(v)) + " " + u[i];
};
const fmtClock = (unix) => new Date(unix * 1000).toLocaleTimeString(lang === "en" ? "en-GB" : "ru-RU", { hour: "2-digit", minute: "2-digit" });
function svgEl(tag, attrs) {
  const el = document.createElementNS(SVGNS, tag);
  for (const [k, v] of Object.entries(attrs || {})) if (v != null) el.setAttribute(k, v);
  return el;
}

// График трафика: загрузка (янтарный) и отдача (голубой), подсказка по наведению.
function trafficChart(pts, o = {}) {
  const box = h("div", { class: "tchart" + (o.compact ? " compact" : "") });
  if (!pts || pts.length < 2) { box.append(h("p", { class: "hint" }, t("monNoData"))); return box; }
  const W = 1000, H = o.compact ? 150 : 200, n = pts.length;
  const max = Math.max(1, ...pts.map((p) => Math.max(p.down, p.up)));
  const x = (i) => (i / (n - 1)) * W, y = (v) => H - 4 - (v / max) * (H - 14);
  const line = (k) => pts.map((p, i) => (i ? "L" : "M") + x(i).toFixed(1) + " " + y(p[k]).toFixed(1)).join(" ");
  const svg = svgEl("svg", { viewBox: "0 0 " + W + " " + H, preserveAspectRatio: "none", class: "tchart-svg", "aria-hidden": "true" });
  for (const fr of [0.25, 0.5, 0.75]) svg.append(svgEl("line", { x1: 0, x2: W, y1: (H * fr).toFixed(1), y2: (H * fr).toFixed(1), class: "grid" }));
  svg.append(svgEl("path", { d: line("down") + " L" + W + " " + H + " L0 " + H + " Z", class: "a-down" }), svgEl("path", { d: line("down"), class: "l-down" }),
    svgEl("path", { d: line("up") + " L" + W + " " + H + " L0 " + H + " Z", class: "a-up" }), svgEl("path", { d: line("up"), class: "l-up" }));
  const cursor = svgEl("line", { x1: 0, x2: 0, y1: 0, y2: H, class: "cursor", visibility: "hidden" });
  svg.append(cursor);
  const tip = h("div", { class: "tchart-tip", hidden: true });
  const wrap = h("div", { class: "tchart-plot" }, svg, tip, h("span", { class: "tchart-max" }, fmtRate(max)));
  wrap.addEventListener("mousemove", (e) => {
    const r = wrap.getBoundingClientRect(), fx = Math.min(1, Math.max(0, (e.clientX - r.left) / r.width)), i = Math.round(fx * (n - 1)), p = pts[i];
    S.monHover = true;
    cursor.setAttribute("x1", x(i)); cursor.setAttribute("x2", x(i)); cursor.setAttribute("visibility", "visible");
    tip.hidden = false;
    tip.replaceChildren(h("b", {}, fmtClock(p.t)), h("span", { class: "d" }, "↓ " + fmtRate(p.down)), h("span", { class: "u" }, "↑ " + fmtRate(p.up)), h("span", { class: "v" }, t("monVpn") + " ↓ " + fmtRate(p.vdown)));
    // позиция подсказки — классом (CSP запрещает менять стиль из кода): 21 шаг по ширине
    const step = Math.round(fx * 20);
    tip.className = "tchart-tip " + (step > 13 ? "r-" + (20 - step) : "l-" + step);
  });
  wrap.addEventListener("mouseleave", () => { S.monHover = false; tip.hidden = true; cursor.setAttribute("visibility", "hidden"); });
  const ticks = [0, 0.25, 0.5, 0.75, 1].map((f) => h("span", {}, fmtClock(pts[Math.round(f * (n - 1))].t)));
  box.append(wrap, h("div", { class: "tchart-x" }, ticks));
  return box;
}
function trafficHead(pts) {
  const last = pts && pts.length ? pts[pts.length - 1] : { down: 0, up: 0 };
  const peak = pts && pts.length ? Math.max(...pts.map((p) => Math.max(p.down, p.up))) : 0;
  return h("div", { class: "tchart-head" },
    h("span", { class: "rate-chip d" }, "↓ " + fmtRate(last.down)), h("span", { class: "rate-chip u" }, "↑ " + fmtRate(last.up)),
    h("span", { class: "tchart-peak" }, t("monPeak") + " ", h("b", {}, fmtRate(peak))));
}

// Узел схемы: подпись и значок для устройства, правила или выхода.
function monNodeInfo(n) {
  if (n.id.startsWith("dev:") || n.id.startsWith("ip:")) return { label: n.name, sub: n.sub || (n.id.startsWith("ip:") ? "" : ""), ico: devIcon(n.kind, 15) };
  if (n.id.startsWith("rule:")) {
    if (n.id === "rule:default") return { label: t("monDefault"), sub: "", ico: icon("route", 15) };
    const svc = (SVC.catalog || []).find((s) => s.id === n.icon);
    return { label: n.name, sub: n.sub, tile: rtTile({ slug: svc ? svc.icon : (SVC.icons[n.icon] ? n.icon : null), id: svc ? svc.id : "", name: n.name || "?" }) };
  }
  if (n.kind === "direct") return { label: t("monDirect"), ico: icon("arrow", 15), cls: "o-direct" };
  if (n.kind === "drop") return { label: t("monDrop"), ico: icon("x", 15), cls: "o-drop" };
  return { label: n.name === "vpn" ? t("monVpnOut") : n.name, sub: n.kind === "group" ? t("navGroups") : "", ico: icon(n.kind === "group" ? "merge" : "shield", 15), cls: "o-vpn" };
}

// Схема «устройства → правила → выходы» на SVG. Толщина потока — соединения или объём.
function sankey(snap, mode, sel, q) {
  const val = (x) => (mode === "bytes" ? x.bytes : x.conns);
  let paths = snap.paths.filter((p) => val(p) > 0);
  const nodesById = {};
  for (const n of [...snap.devices, ...snap.rules, ...snap.outs]) nodesById[n.id] = n;
  if (q) {
    const ql = q.toLowerCase();
    const hit = (id) => { const n = nodesById[id] || {}; const i = monNodeInfo(n); return (String(i.label) + " " + (n.sub || "") + " " + id).toLowerCase().includes(ql); };
    const hosts = new Set(snap.conns.filter((c) => (c.host + " " + c.dst).toLowerCase().includes(ql)).map((c) => c.src));
    paths = paths.filter((p) => hit(p.dev) || hit(p.rule) || hit(p.out) || [...hosts].some((h) => p.dev === "ip:" + h || (nodesById[p.dev] && nodesById[p.dev].sub === h)));
  }
  if (!paths.length) return h("p", { class: "hint" }, t("monNoFlows"));
  // ограничение числа узлов в колонке: остальное — «ещё N»
  const LIMIT = 14;
  const cols = ["dev", "rule", "out"].map((k) => {
    const sum = {};
    for (const p of paths) sum[p[k]] = (sum[p[k]] || 0) + val(p);
    const ids = Object.keys(sum).sort((a, b) => sum[b] - sum[a] || a.localeCompare(b));
    const keep = new Set(ids.slice(0, LIMIT));
    const rest = ids.length - keep.size;
    return { k, ids: ids.slice(0, LIMIT).concat(rest > 0 ? ["more:" + k] : []), sum, keep, rest };
  });
  paths = paths.map((p) => {
    const q2 = { ...p };
    for (const c of cols) if (!c.keep.has(p[c.k])) q2[c.k] = "more:" + c.k;
    return q2;
  });
  for (const c of cols) if (c.rest > 0) { c.sum["more:" + c.k] = 0; for (const p of paths) if (p[c.k] === "more:" + c.k) c.sum["more:" + c.k] += val(p); }
  const total = paths.reduce((a, p) => a + val(p), 0) || 1;
  const W = 1000, NW = 230, GAP = 10, MINH = 34;
  const colX = [0, (W - NW) / 2, W - NW];
  const maxN = Math.max(...cols.map((c) => c.ids.length));
  const scaleH = Math.max(260, maxN * (MINH + GAP) + 40);
  const pos = {};
  cols.forEach((c, ci) => {
    let y = 4;
    for (const id of c.ids) {
      const hgt = Math.max(MINH, (c.sum[id] / total) * (scaleH - c.ids.length * GAP));
      pos[id] = { x: colX[ci], y, h: hgt, outY: y, inY: y };
      y += hgt + GAP;
    }
    c.height = y;
  });
  const H = Math.max(...cols.map((c) => c.height)) + 4;
  const svg = svgEl("svg", { viewBox: "0 0 " + W + " " + H, class: "sankey-svg", role: "img", "aria-label": t("monTopo") });
  // выделение: путь выбранного узла
  const onSel = (p) => !sel || p.dev === sel || p.rule === sel || p.out === sel;
  const bandsG = svgEl("g", { class: "bands" }), nodesG = svgEl("g", { class: "nodes" });
  const hue = (id) => { let s2 = 0; for (const ch of id) s2 = (s2 * 31 + ch.codePointAt(0)) % 997; return s2 % 8; };
  // ширина потока внутри узла пропорциональна его значению
  const band = (a, b, v, cls) => {
    const pa = pos[a], pb = pos[b], ha = (v / cols[0].sum[a] || 0);
    return { pa, pb, v, cls };
  };
  const seg = [];
  const agg = (k1, k2) => {
    const m = {};
    for (const p of paths) { const key = p[k1] + "|" + p[k2]; (m[key] = m[key] || { a: p[k1], b: p[k2], v: 0, on: false }).v += val(p); if (onSel(p)) m[key].on = true; }
    return Object.values(m).sort((x, y) => (pos[x.a].y - pos[y.a].y) || (pos[x.b].y - pos[y.b].y));
  };
  const nodeSum = (id, ci) => cols[ci].sum[id] || 1;
  for (const [k1, k2, ci1, ci2] of [["dev", "rule", 0, 1], ["rule", "out", 1, 2]]) {
    for (const l of agg(k1, k2)) {
      const pa = pos[l.a], pb = pos[l.b];
      const ta = (l.v / nodeSum(l.a, ci1)) * pa.h, tb = (l.v / nodeSum(l.b, ci2)) * pb.h;
      const x0 = pa.x + NW, x1 = pb.x, y0 = pa.outY, y1 = pb.inY, xm = (x0 + x1) / 2;
      pa.outY += ta; pb.inY += tb;
      const d = "M" + x0 + " " + y0 + " C" + xm + " " + y0 + " " + xm + " " + y1 + " " + x1 + " " + y1 +
        " L" + x1 + " " + (y1 + tb) + " C" + xm + " " + (y1 + tb) + " " + xm + " " + (y0 + ta) + " " + x0 + " " + (y0 + ta) + " Z";
      const colorId = k1 === "dev" ? l.a : l.b;
      bandsG.append(svgEl("path", { d, class: "band h" + hue(colorId) + (sel ? (l.on ? " on" : " dim") : "") + (k2 === "out" ? " to-" + ((nodesById[l.b] || {}).kind || "more") : "") }));
    }
  }
  void band; void seg;
  // узлы
  const pick = (id) => { S.monSel = S.monSel === id ? "" : id; renderMain(); };
  cols.forEach((c, ci) => {
    for (const id of c.ids) {
      const p = pos[id], n = nodesById[id];
      const g = svgEl("g", { class: "node" + (sel && sel === id ? " sel" : "") + (sel && sel !== id && !paths.some((x) => onSel(x) && (x.dev === id || x.rule === id || x.out === id)) ? " dim" : ""), tabindex: id.startsWith("more:") ? null : "0", role: id.startsWith("more:") ? null : "button" });
      const info = n ? monNodeInfo(n) : { label: t("monMore", { n: c.rest }) };
      g.append(svgEl("rect", { x: p.x, y: p.y, width: NW, height: p.h, rx: 8, class: "nb " + (info.cls || "") + (ci === 0 ? " c-dev" : ci === 1 ? " c-rule" : " c-out") }));
      const tx = svgEl("text", { x: p.x + 12, y: p.y + Math.min(p.h / 2, 22) + 4, class: "nl" });
      tx.textContent = String(info.label || "").slice(0, 26);
      const tv = svgEl("text", { x: p.x + NW - 10, y: p.y + Math.min(p.h / 2, 22) + 4, class: "nv", "text-anchor": "end" });
      tv.textContent = mode === "bytes" ? fmtBytes(c.sum[id]) : String(c.sum[id]) + (ci === 2 || ci === 0 ? "" : "");
      g.append(tx, tv);
      if (info.sub && p.h >= 46) { const ts = svgEl("text", { x: p.x + 12, y: p.y + 38, class: "ns" }); ts.textContent = String(info.sub).slice(0, 30); g.append(ts); }
      if (!id.startsWith("more:")) {
        g.addEventListener("click", () => pick(id));
        g.addEventListener("keydown", (e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); pick(id); } });
        const tt = svgEl("title", {}); tt.textContent = info.label + (info.sub ? " · " + info.sub : "") + " — " + (mode === "bytes" ? fmtBytes(c.sum[id]) : c.sum[id] + " " + t("monConns")); g.append(tt);
      }
      nodesG.append(g);
    }
  });
  svg.append(bandsG, nodesG);
  return h("div", { class: "sankey" },
    h("div", { class: "sankey-cols" }, h("span", {}, t("monDevices")), h("span", {}, t("monRules")), h("span", {}, t("monOuts"))),
    svg);
}

function pageMonitor() {
  const m = S.mon, f = S.flows;
  if (!m) { loadMonitor(); return h("div", {}, heading(t("navInfra"), t("monTitle")), h("div", { class: "loading" }, t("loading"))); }
  if (!f) loadFlows(true);
  const win = S.monWin || 900;
  const setWin = (v) => { S.monWin = v; loadMonitor(); };
  const mode = S.monMode === "bytes" && f && f.bytes ? "bytes" : "conns";
  const q = S.monQ || "";
  const search = h("input", { type: "search", id: "mon-q", "data-keep": "", "data-nodirty": "", placeholder: t("monSearch"), "aria-label": t("monSearch"), value: q, autocomplete: "off",
    oninput: (e) => { S.monQ = e.target.value; clearTimeout(S.monQT); S.monQT = setTimeout(() => { renderMain(); const el = document.getElementById("mon-q"); if (el) { el.focus(); el.setSelectionRange(el.value.length, el.value.length); } }, 250); } });
  const groupsN = f ? f.outs.filter((o) => o.kind === "group").length : 0;
  // соединения одного устройства к одному адресу и порту — одной строкой
  const grouped = {};
  for (const c of f ? f.conns : []) {
    if (q && ![c.device, c.src, c.dst, c.host, c.rule, c.out].join(" ").toLowerCase().includes(q.toLowerCase())) continue;
    const k = [c.src, c.host || c.dst, c.port, c.proto, c.rule, c.out].join("|");
    const g = grouped[k] || (grouped[k] = { ...c, n: 0, bytes: 0, dsts: new Set() });
    g.n++; g.bytes += c.bytes || 0; g.dsts.add(c.dst);
  }
  const conns = Object.values(grouped).sort((a, b) => (f.bytes ? b.bytes - a.bytes : 0) || b.n - a.n);
  return h("div", {},
    heading(t("navInfra"), t("monTitle"), t("monSub")),
    h("div", { class: "card" },
      h("div", { class: "heading-row" }, h("h2", {}, icon("activity", 18), " ", t("monTraffic")),
        h("select", { class: "cell-input", "data-nodirty": "", "aria-label": t("monTraffic"), onchange: (e) => setWin(+e.target.value) },
          h("option", { value: "900", selected: win === 900 }, t("monWin15")), h("option", { value: "3600", selected: win === 3600 }, t("monWin60")))),
      trafficHead(m.traffic), trafficChart(m.traffic)),
    h("div", { class: "card" },
      h("div", { class: "heading-row" }, h("h2", {}, t("monTopo")),
        f && f.bytes ? h("div", { class: "seg" },
          h("button", { type: "button", "aria-pressed": String(mode === "conns"), onclick: () => { S.monMode = "conns"; renderMain(); } }, t("monByConns")),
          h("button", { type: "button", "aria-pressed": String(mode === "bytes"), onclick: () => { S.monMode = "bytes"; renderMain(); } }, t("monByBytes"))) : null),
      !f ? h("div", { class: "loading" }, t("loading"))
        : f.source === "none" ? h("p", { class: "info-line" }, icon("alert", 16), h("span", {}, t("monNoConntrack")))
          : [h("div", { class: "field mon-search" }, search),
            h("div", { class: "mon-stats" },
              h("span", {}, icon("devices", 16), h("b", {}, String(f.devices.length)), t("monStatDev")),
              h("span", {}, icon("route", 16), h("b", {}, String(f.rules.length)), t("monStatRules")),
              h("span", {}, icon("layers", 16), h("b", {}, String(f.outs.length)), t("monStatOuts") + (groupsN ? " · " + groupsN + " " + t("navGroups").toLowerCase() : "")),
              h("span", {}, icon("link", 16), h("b", {}, String(f.total)), t("monStatConns"))),
            sankey(f, mode, S.monSel || "", q),
            h("p", { class: "hint" }, t("monSelHint"), f.bytes ? "" : " " + t("monNoBytes"))]),
    f && f.source !== "none" && h("div", { class: "card" }, h("h2", {}, t("monConnTitle")),
      conns.length ? h("div", { class: "table-scroll" }, h("table", { class: "mon-conns" },
        h("thead", {}, h("tr", {}, [t("monColDev"), t("monColDst"), t("monColN"), t("monColRule"), t("monColOut"), t("monColBytes")].map((x) => h("th", { scope: "col" }, x)))),
        h("tbody", {}, conns.slice(0, 60).map((c) => h("tr", {},
          h("td", {}, c.device, c.device !== c.src ? h("div", { class: "sub mono" }, c.src) : null),
          h("td", {}, c.host || h("span", { class: "mono" }, c.dst), h("div", { class: "sub mono" }, (c.host ? [...c.dsts].slice(0, 2).join(", ") + (c.dsts.size > 2 ? "…" : "") + " · " : "") + c.proto + "/" + c.port)),
          h("td", { class: "mono" }, String(c.n)),
          h("td", {}, c.rule || h("span", { class: "muted" }, t("monDefault"))),
          h("td", {}, c.out === "direct" ? t("monDirect") : c.out === "drop" ? t("monDrop") : c.out === "vpn" ? t("monVpnOut") : c.out),
          h("td", { class: "mono" }, f.bytes ? fmtBytes(c.bytes) : "—"))))),
        conns.length > 60 && h("p", { class: "hint" }, t("monShown", { n: 60, m: conns.length }))) : h("p", { class: "hint" }, t("monNoFlows"))));
}

/* ---------- устройства ---------- */
const DEV_KINDS = ["pc", "laptop", "phone", "tablet", "tv", "console", "speaker", "nas", "printer", "camera", "router", "wifi", "iot", "other"];
const devIcon = (kind, size = 18) => icon("dev_" + (DEV_KINDS.includes(kind) ? kind : "other"), size);
// Тип по имени из DHCP: только догадка, его можно поменять.
function guessKind(name) {
  const n = String(name || "").toLowerCase();
  const rules = [[/iphone|galaxy|pixel|redmi|xiaomi|phone|android|honor|huawei|poco/, "phone"], [/ipad|tab\b|tablet|pad/, "tablet"],
    [/macbook|book|laptop|notebook|thinkpad/, "laptop"], [/tv|bravia|webos|tizen|chromecast|android-tv/, "tv"], [/station|alice|homepod|echo|speaker|sonos/, "speaker"],
    [/nas|synology|qnap|truenas/, "nas"], [/print|epson|canon|brother|hp-/, "printer"], [/cam|ipc|dahua|hikvision/, "camera"],
    [/playstation|ps[345]|xbox|switch|deck/, "console"], [/mikrotik|keenetic|router|gateway/, "router"], [/ap\b|access|unifi|wifi|wi-fi/, "wifi"],
    [/esp|tuya|sonoff|yeelight|vacuum|ecovacs|roborock|plug|sensor/, "iot"], [/pc|desktop|win|imac|mac/, "pc"]];
  for (const [re, k] of rules) if (re.test(n)) return k;
  return "other";
}
async function loadDevices(quiet) {
  try {
    S.dev = await api("GET", "/api/devices");
    if (!S.devEdit || !S.dirty) S.devEdit = S.dev.devices.map((d) => ({ ...d }));
  } catch (e) {
    if (!quiet) { say("error", e.message); renderFlash(); }
  }
  if ((S.route.page === "devices" || S.route.page === "tools") && !S.dirty && !S.busy) renderMain();
}
// Сколько правил ссылается на устройство (по тексту правил, если он уже загружен).
function devRefCount(id) {
  const txt = (S.pol && S.pol.settings && S.pol.settings.rules) || "";
  const re = new RegExp("src=[^\\s]*@" + id.replace(/[-]/g, "\\-") + "(?![a-z0-9_-])", "g");
  return (txt.match(re) || []).length;
}
function pageDevices() {
  if (!S.dev || !S.devEdit) { loadDevices(); return h("div", {}, heading(t("navInfra"), t("devTitle")), h("div", { class: "loading" }, t("loading"))); }
  if (!S.pol) loadPolicy(true);
  const ed = S.devEdit;
  const dirty = () => { S.dirty = true; };
  const save = () => act(async () => {
    S.dev = await api("PUT", "/api/devices", { devices: ed });
    S.devEdit = S.dev.devices.map((d) => ({ ...d }));
  }, t("devSaved"));
  const leases = () => act(async () => { S.leases = (await api("GET", "/api/devices/leases")).leases || []; S.leasePick = {}; });
  const row = (d, i) => h("tr", {},
    h("td", { class: "dev-kind" }, h("label", { class: "dev-kind-sel" }, devIcon(d.kind, 18),
      h("select", { "aria-label": t("devKind") + ": " + (d.name || d.addr), onchange: (e) => { d.kind = e.target.value; dirty(); renderMain(); } },
        DEV_KINDS.map((k) => h("option", { value: k, selected: d.kind === k }, t("devKind_" + k)))))),
    h("td", {}, h("input", { type: "text", value: d.name || "", maxlength: "40", "aria-label": t("devName"), placeholder: t("devName"), oninput: (e) => { d.name = e.target.value; dirty(); } })),
    h("td", {}, h("input", { type: "text", class: "mono", value: d.addr || "", "aria-label": t("devAddr"), placeholder: "192.168.1.10", spellcheck: "false", oninput: (e) => { d.addr = e.target.value; dirty(); } })),
    h("td", { class: "dev-used" }, d.id && devRefCount(d.id) ? h("span", { class: "pill pill-idle" }, t("devUsed", { n: devRefCount(d.id) })) : ""),
    h("td", { class: "dev-act" }, h("button", { type: "button", class: "icon-btn", "aria-label": t("devRemove", { name: d.name || d.addr || "" }), title: t("delete"),
      onclick: () => { ed.splice(i, 1); dirty(); renderMain(); } }, icon("trash", 16))));
  const have = new Set(ed.map((d) => String(d.addr).trim()));
  const fresh = (S.leases || []).filter((l) => !have.has(l.addr));
  const leaseCard = S.leases && h("div", { class: "card" }, h("h2", {}, t("devLeasesTitle")),
    fresh.length ? [h("p", { class: "hint" }, t("devLeasesHint")),
      h("div", { class: "table-scroll" }, h("table", { class: "dev-leases" }, h("tbody", {}, fresh.map((l) => {
        const name = l.comment || l.host_name || l.addr;
        return h("tr", {},
          h("td", {}, h("input", { type: "checkbox", "data-nodirty": "", "aria-label": name, checked: !!S.leasePick[l.addr], onchange: (e) => { S.leasePick[l.addr] = e.target.checked; } })),
          h("td", {}, devIcon(guessKind(name), 16)), h("td", {}, name), h("td", { class: "mono" }, l.addr), h("td", { class: "mono muted" }, l.mac || ""),
          h("td", {}, l.dynamic ? "" : h("span", { class: "pill pill-on" }, t("devLeasesStatic"))));
      })))),
      h("div", { class: "actions" }, h("button", { class: "btn btn-secondary", type: "button", onclick: () => {
        for (const l of fresh) if (S.leasePick[l.addr]) { const name = (l.comment || l.host_name || l.addr).slice(0, 40); ed.push({ id: "", name, addr: l.addr, kind: guessKind(name) }); }
        S.leases = null; dirty(); renderMain();
      } }, icon("plus", 16), t("devLeasesAdd")))]
      : h("p", { class: "hint" }, t("devLeasesNone")));
  return h("div", {},
    heading(t("navInfra"), t("devTitle"), t("devSub")),
    h("div", { class: "card" },
      h("div", { class: "heading-row" }, h("h2", {}, t("devListTitle")),
        h("div", { class: "actions" },
          h("button", { class: "btn btn-secondary", type: "button", disabled: S.busy, onclick: leases }, icon("download", 16), t("devFromDhcp")),
          h("button", { class: "btn btn-primary", type: "button", onclick: () => { ed.push({ id: "", name: "", addr: "", kind: "pc" }); dirty(); renderMain(); setTimeout(() => { const ins = document.querySelectorAll(".dev-table tbody tr:last-child input"); if (ins[0]) ins[0].focus(); }); } }, icon("plus", 16), t("devAdd")))),
      ed.length ? h("div", { class: "table-scroll" }, h("table", { class: "dev-table" },
        h("thead", {}, h("tr", {}, h("th", { scope: "col" }, t("devKind")), h("th", { scope: "col" }, t("devName")), h("th", { scope: "col" }, t("devAddr")), h("th", {}), h("th", {}))),
        h("tbody", {}, ed.map(row)))) : h("p", { class: "hint" }, t("devEmpty")),
      h("p", { class: "info-line" }, icon("info", 16), h("span", {}, t("devHint"))),
      h("div", { class: "actions" }, h("button", { class: "btn btn-primary", type: "button", disabled: S.busy, onclick: save }, icon("check", 16), t("devSave")))),
    leaseCard);
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
    "/user/group/add name=awg31 policy=read,write,api,rest-api",
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
  // При входе поле пустое: логин задан при первичной настройке и может быть любым. "admin" подставляется
  // только при создании администратора.
  const user = h("input", { id: "au-user", type: "text", autocomplete: "username", value: setup ? "admin" : "" });
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
    if (S.route && S.route.page === "log") loadLog(true);
    loadMonitor(true);
    if (S.route && (S.route.page === "monitor" || S.route.page === "overview")) loadFlows(true);
    if (S.route && S.route.page === "policy" && !S.dirty) loadFlows(true);
  }
}, 5000);

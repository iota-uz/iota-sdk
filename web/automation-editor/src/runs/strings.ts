// Strings of the runs view, the Dry run and Test node panels in en, ru, uz and uz-Cyrl (C4). They live
// apart from the editor's table: they load with the runs chunk.
import { pick, plural } from '../locale'
import type { DecisionMode, EditorLocale, RunState, RunWait } from '../types'

type Plural = { one: string; few?: string; many?: string; other: string }

export type RunStrings = {
  list: {
    label: string
    search: string
    searchLabel: string
    filter: string
    states: Record<RunState, string>
    noSubject: string
    empty: string
    emptyText: string
    start: string
    emptyFilter: string
    reset: string
    more: string
    loading: string
    loadFailed: string
    stale(time: string): string
    count(n: number): string
  }
  run: {
    notFound: string
    notFoundText: string
    back: string
    live: string
    reconnecting: string
    finished: string
    onVersion(n: number): string
    openVersion(n: number): string
    openSubject(label: string): string
    startedBy(trigger: string): string
    started(time: string): string
    loadFailed: string
    canvas: string
    id: string
    cancel: string
    cancelling: string
    retry: string
    retrying: string
    resolve: string
    resolving: string
    cancelTitle: string
    cancelText: string
    keep: string
    resolveTitle(node: string): string
    resolveText: string
    outcome: string
    pickOutcome: string
    stillWaiting(what: string): string
    actionFailed(what: string): string
    finishedAlready: string
    cancelled: string
    retried: string
    resolved: string
    what: { cancel: string; retry: string; resolve: string }
    nodeLabel: string
  }
  node: {
    input: string
    output: string
    logs: string
    outcome: string
    error: string
    noData: string
    hidden: string
    notReached: string
    filterLogs: string
    noLogMatch(q: string): string
    logCount(shown: number, total: number): string
    noLogs: string
    copy: string
    copied: string
    loadFull: string
    folded(size: string): string
    attempt(n: number): string
    item(n: number): string
    waiting: Record<RunWait['kind'], string>
    until(time: string): string
    duration(text: string): string
    close: string
    steps(n: number): string
    expand: string
    collapse: string
    empty: string
  }
  dry: {
    title: string
    note: string
    subject: string
    subjectNone: string
    subjectSearch: string
    events: string
    eventsHint: string
    invalidJson: string
    context: string
    contextHint: string
    decisions: string
    modes: Record<DecisionMode, string>
    modeHints: Record<DecisionMode, string>
    run: string
    running: string
    stillWaiting: string
    failed: string
    path: string
    notTriggered: string
    condition: string
    wouldSend: string
    nothingSent: string
    live(n: number): string
    ended: string
    waits(node: string): string
    missing(path: string): string
    error: string
    close: string
    empty: string
    captured: string
    executed: string
  }
  test: {
    open: string
    title(node: string): string
    note: string
    input: string
    example: string
    exampleNone: string
    run: string
    running: string
    stillWaiting: string
    result: string
    outcome: string
    duration(ms: number): string
    failed: string
    secretMissing(name: string): string
    secretUnset: string
    confirmTitle: string
    confirmText(connection: string): string
    noConnection: string
    irreversible: string
    confirm: string
    cancel: string
    close: string
    invalidJson: string
    secrets: string
    logs: string
    output: string
  }
}

const n = (locale: EditorLocale, count: number, forms: Plural) => plural(locale, count, forms)

const en: RunStrings = {
  list: {
    label: 'Runs of the graph',
    search: 'Search by subject',
    searchLabel: 'Subject',
    filter: 'State',
    states: { running: 'Running', waiting: 'Waiting', blocked: 'Blocked', failed: 'Failed', done: 'Done', cancelled: 'Cancelled' },
    noSubject: 'No subject',
    empty: 'No runs yet',
    emptyText: 'A run appears here each time a trigger of this graph fires.',
    start: 'Run',
    emptyFilter: 'No runs match the filter',
    reset: 'Reset filters',
    more: 'Show more',
    loading: 'Loading runs…',
    loadFailed: 'Could not load the runs. Try again in a minute.',
    stale: (time) => `Could not refresh · data as of ${time}`,
    count: (c) => n('en', c, { one: '{n} run', other: '{n} runs' }),
  },
  run: {
    notFound: 'Run not found',
    notFoundText: 'It was deleted, or it belongs to another workspace.',
    back: 'Runs',
    live: 'Live',
    reconnecting: 'Reconnecting…',
    finished: 'Finished',
    onVersion: (v) => `Run on version ${v}`,
    openVersion: (v) => `Open v${v}`,
    openSubject: (label) => `Open ${label}`,
    startedBy: (trigger) => `Started by ${trigger}`,
    started: (time) => `Started ${time}`,
    loadFailed: 'Could not load the run. Try again in a minute.',
    canvas: 'Graph of the run',
    id: 'Run ID',
    cancel: 'Cancel run',
    cancelling: 'Cancelling…',
    retry: 'Retry the call',
    retrying: 'Retrying…',
    resolve: 'Resolve',
    resolving: 'Resolving…',
    cancelTitle: 'Cancel this run?',
    cancelText: 'The run stops where it is. Steps already made are not undone.',
    keep: 'Keep running',
    resolveTitle: (node) => `Resolve “${node}”`,
    resolveText: 'The blocked step is not called again: the run goes on by the outcome you pick.',
    outcome: 'Outcome',
    pickOutcome: 'Pick an outcome',
    stillWaiting: (what) => `Still waiting for ${what}. Nothing will run twice.`,
    actionFailed: (what) => `Could not ${what}. Try again in a minute.`,
    finishedAlready: 'The run has already finished.',
    cancelled: 'Run cancelled',
    retried: 'The call is repeated',
    resolved: 'The run goes on',
    what: { cancel: 'cancel the run', retry: 'repeat the call', resolve: 'resolve the step' },
    nodeLabel: 'Step',
  },
  node: {
    input: 'Input',
    output: 'Output',
    logs: 'Logs',
    outcome: 'Outcome',
    error: 'Error',
    noData: 'The step has no data',
    hidden: 'The step read data you may not see: its input and output are hidden.',
    notReached: 'The run did not reach this step.',
    filterLogs: 'Filter logs',
    noLogMatch: (q) => `No log line matches “${q}”`,
    logCount: (shown, total) => `${shown} of ${total}`,
    noLogs: 'No logs',
    copy: 'Copy',
    copied: 'Copied',
    loadFull: 'Load in full',
    folded: (size) => `The output is ${size}; it is shown folded.`,
    attempt: (a) => `Attempt ${a}`,
    item: (i) => `Item ${i}`,
    waiting: { event: 'Waiting for an event', timer: 'Waiting for a timer', human: 'Waiting for a person', call: 'Waiting for a call' },
    until: (time) => `until ${time}`,
    duration: (text) => `took ${text}`,
    close: 'Close',
    steps: (c) => n('en', c, { one: '{n} step', other: '{n} steps' }),
    expand: 'Expand',
    collapse: 'Collapse',
    empty: 'empty',
  },
  dry: {
    title: 'Dry run',
    note: 'Nothing is sent outside: effects are captured and shown below.',
    subject: 'Subject',
    subjectNone: 'No subject',
    subjectSearch: 'Find a subject',
    events: 'Events (JSON array)',
    eventsHint: 'The event that starts the run, then the ones its waits receive.',
    invalidJson: 'This is not a JSON array of objects.',
    context: 'Answers (JSON)',
    contextHint: 'nodes.<id>.output and subject fields the run reads instead of calling.',
    decisions: 'Decisions',
    modes: { cached: 'From cache', live: 'Live call', scripted: 'Set the answer' },
    modeHints: {
      cached: 'A decision made before is reused; a new one is made live once and cached.',
      live: 'Decisions are made for real each time; the provider may charge for them.',
      scripted: 'Decisions are taken from the context you give; nothing is called.',
    },
    run: 'Run dry',
    running: 'Running…',
    stillWaiting: 'Still waiting for the dry run. Nothing is sent outside.',
    failed: 'Could not run the dry run. Try again in a minute.',
    path: 'Path',
    notTriggered: 'The trigger did not fire: its filter did not pass.',
    condition: 'Condition',
    wouldSend: 'What would be sent',
    nothingSent: 'Nothing would be sent.',
    live: (c) => n('en', c, { one: '{n} live decision', other: '{n} live decisions' }),
    ended: 'The run ended',
    waits: (node) => `The run waits at “${node}”`,
    missing: (path) => `Missing data: ${path}`,
    error: 'Stopped with an error',
    close: 'Close the dry run',
    empty: 'Pick a subject and an event, then run.',
    captured: 'captured',
    executed: 'live',
  },
  test: {
    open: 'Test step',
    title: (node) => `Test “${node}”`,
    note: 'The step runs for real with this input.',
    input: 'Input (JSON)',
    example: 'From the last run',
    exampleNone: 'No run reached this step yet.',
    run: 'Test',
    running: 'Testing…',
    stillWaiting: 'Still waiting for the step. It will not run twice.',
    result: 'Result',
    outcome: 'Outcome',
    duration: (ms) => `${ms} ms`,
    failed: 'Could not test the step. Try again in a minute.',
    secretMissing: (name) => `Secret ${name} is not set`,
    secretUnset: 'not set',
    confirmTitle: 'Run the step for real?',
    confirmText: (connection) => `The step calls ${connection}.`,
    noConnection: 'the service',
    irreversible: 'The effect is irreversible',
    confirm: 'Run for real',
    cancel: 'Cancel',
    close: 'Close the test',
    invalidJson: 'This is not a JSON object.',
    secrets: 'Secrets',
    logs: 'Logs',
    output: 'Output',
  },
}

const ru: RunStrings = {
  list: {
    label: 'Запуски графа',
    search: 'Поиск по объекту',
    searchLabel: 'Объект',
    filter: 'Состояние',
    states: { running: 'Идёт', waiting: 'Ждёт', blocked: 'Заблокирован', failed: 'Упал', done: 'Готово', cancelled: 'Отменён' },
    noSubject: 'Без объекта',
    empty: 'Запусков ещё нет',
    emptyText: 'Запуск появляется здесь каждый раз, когда срабатывает триггер графа.',
    start: 'Запустить',
    emptyFilter: 'Нет запусков под фильтром',
    reset: 'Сбросить',
    more: 'Показать ещё',
    loading: 'Загружаем запуски…',
    loadFailed: 'Не удалось загрузить запуски. Повторите через минуту.',
    stale: (time) => `Не удалось обновить · данные на ${time}`,
    count: (c) => n('ru', c, { one: '{n} запуск', few: '{n} запуска', many: '{n} запусков', other: '{n} запуска' }),
  },
  run: {
    notFound: 'Запуск не найден',
    notFoundText: 'Его удалили, или он из другого рабочего пространства.',
    back: 'Запуски',
    live: 'В эфире',
    reconnecting: 'Переподключение…',
    finished: 'Завершено',
    onVersion: (v) => `Запуск на версии ${v}`,
    openVersion: (v) => `Открыть v${v}`,
    openSubject: (label) => `Открыть ${label}`,
    startedBy: (trigger) => `Запущен: ${trigger}`,
    started: (time) => `Начат ${time}`,
    loadFailed: 'Не удалось загрузить запуск. Повторите через минуту.',
    canvas: 'Граф запуска',
    id: 'ID запуска',
    cancel: 'Отменить запуск',
    cancelling: 'Отменяем…',
    retry: 'Повторить вызов',
    retrying: 'Повторяем…',
    resolve: 'Разрешить',
    resolving: 'Разрешаем…',
    cancelTitle: 'Отменить этот запуск?',
    cancelText: 'Запуск остановится там, где он сейчас. Уже сделанные шаги не откатываются.',
    keep: 'Не отменять',
    resolveTitle: (node) => `Разрешить «${node}»`,
    resolveText: 'Заблокированный шаг не вызывается снова: запуск пойдёт по выбранному исходу.',
    outcome: 'Исход',
    pickOutcome: 'Выберите исход',
    stillWaiting: (what) => `Всё ещё ждём: ${what}. Ничего не запустится дважды.`,
    actionFailed: (what) => `Не удалось ${what}. Повторите через минуту.`,
    finishedAlready: 'Запуск уже завершён.',
    cancelled: 'Запуск отменён',
    retried: 'Вызов повторяется',
    resolved: 'Запуск продолжается',
    what: { cancel: 'отменить запуск', retry: 'повторить вызов', resolve: 'разрешить шаг' },
    nodeLabel: 'Шаг',
  },
  node: {
    input: 'Вход',
    output: 'Выход',
    logs: 'Логи',
    outcome: 'Исход',
    error: 'Ошибка',
    noData: 'У ноды нет данных',
    hidden: 'Шаг прочёл данные, которые вам не видны: его вход и выход скрыты.',
    notReached: 'Запуск не дошёл до этого шага.',
    filterLogs: 'Фильтр логов',
    noLogMatch: (q) => `Ни одна строка логов не подходит под «${q}»`,
    logCount: (shown, total) => `${shown} из ${total}`,
    noLogs: 'Логов нет',
    copy: 'Скопировать',
    copied: 'Скопировано',
    loadFull: 'Загрузить полностью',
    folded: (size) => `Выход занимает ${size}; он показан свёрнутым.`,
    attempt: (a) => `Попытка ${a}`,
    item: (i) => `Элемент ${i}`,
    waiting: { event: 'Ждёт событие', timer: 'Ждёт таймер', human: 'Ждёт человека', call: 'Ждёт вызов' },
    until: (time) => `до ${time}`,
    duration: (text) => `заняло ${text}`,
    close: 'Закрыть',
    steps: (c) => n('ru', c, { one: '{n} шаг', few: '{n} шага', many: '{n} шагов', other: '{n} шага' }),
    expand: 'Развернуть',
    collapse: 'Свернуть',
    empty: 'пусто',
  },
  dry: {
    title: 'Пробный прогон',
    note: 'Наружу ничего не отправляется: эффекты перехватываются и показаны ниже.',
    subject: 'Объект',
    subjectNone: 'Без объекта',
    subjectSearch: 'Найти объект',
    events: 'События (JSON-массив)',
    eventsHint: 'Событие, которое запускает граф, затем те, что получают его ожидания.',
    invalidJson: 'Это не JSON-массив объектов.',
    context: 'Ответы (JSON)',
    contextHint: 'nodes.<id>.output и поля объекта, которые прогон читает вместо вызова.',
    decisions: 'Решения',
    modes: { cached: 'Из кеша', live: 'Живой вызов', scripted: 'Задать ответ' },
    modeHints: {
      cached: 'Принятое раньше решение берётся из кеша; новое принимается вживую один раз и кешируется.',
      live: 'Решения каждый раз принимаются по-настоящему; провайдер может брать за них плату.',
      scripted: 'Решения берутся из заданного контекста; ничего не вызывается.',
    },
    run: 'Прогнать',
    running: 'Прогоняем…',
    stillWaiting: 'Всё ещё ждём прогон. Наружу ничего не отправляется.',
    failed: 'Не удалось выполнить пробный прогон. Повторите через минуту.',
    path: 'Путь',
    notTriggered: 'Триггер не сработал: его фильтр не прошёл.',
    condition: 'Условие',
    wouldSend: 'Что было бы отправлено',
    nothingSent: 'Ничего не было бы отправлено.',
    live: (c) => n('ru', c, { one: '{n} живое решение', few: '{n} живых решения', many: '{n} живых решений', other: '{n} живого решения' }),
    ended: 'Запуск завершился',
    waits: (node) => `Запуск ждёт на «${node}»`,
    missing: (path) => `Не хватает данных: ${path}`,
    error: 'Остановлен с ошибкой',
    close: 'Закрыть пробный прогон',
    empty: 'Выберите объект и событие, затем прогоните.',
    captured: 'перехвачено',
    executed: 'вживую',
  },
  test: {
    open: 'Проверить шаг',
    title: (node) => `Проверка «${node}»`,
    note: 'Шаг выполняется по-настоящему с этим входом.',
    input: 'Вход (JSON)',
    example: 'Из последнего запуска',
    exampleNone: 'Ни один запуск ещё не дошёл до этого шага.',
    run: 'Проверить',
    running: 'Проверяем…',
    stillWaiting: 'Всё ещё ждём шаг. Он не выполнится дважды.',
    result: 'Результат',
    outcome: 'Исход',
    duration: (ms) => `${ms} мс`,
    failed: 'Не удалось проверить шаг. Повторите через минуту.',
    secretMissing: (name) => `Секрет ${name} не задан`,
    secretUnset: 'не задан',
    confirmTitle: 'Выполнить шаг по-настоящему?',
    confirmText: (connection) => `Шаг обращается к ${connection}.`,
    noConnection: 'сервису',
    irreversible: 'Эффект необратим',
    confirm: 'Выполнить',
    cancel: 'Отмена',
    close: 'Закрыть проверку',
    invalidJson: 'Это не JSON-объект.',
    secrets: 'Секреты',
    logs: 'Логи',
    output: 'Выход',
  },
}

const uz: RunStrings = {
  list: {
    label: 'Grafning ishga tushirishlari',
    search: 'Obyekt bo‘yicha qidirish',
    searchLabel: 'Obyekt',
    filter: 'Holat',
    states: { running: 'Bajarilmoqda', waiting: 'Kutmoqda', blocked: 'Bloklangan', failed: 'Xato', done: 'Tayyor', cancelled: 'Bekor qilingan' },
    noSubject: 'Obyektsiz',
    empty: 'Hali ishga tushirishlar yo‘q',
    emptyText: 'Grafning triggeri har safar ishlaganda bu yerda ishga tushirish paydo bo‘ladi.',
    start: 'Ishga tushirish',
    emptyFilter: 'Filtrga mos ishga tushirishlar yo‘q',
    reset: 'Filtrlarni tozalash',
    more: 'Yana ko‘rsatish',
    loading: 'Ishga tushirishlar yuklanmoqda…',
    loadFailed: 'Ishga tushirishlarni yuklab bo‘lmadi. Bir daqiqadan keyin qayta urinib ko‘ring.',
    stale: (time) => `Yangilab bo‘lmadi · ma’lumotlar ${time} holatiga`,
    count: (c) => n('uz', c, { one: '{n} ta ishga tushirish', other: '{n} ta ishga tushirish' }),
  },
  run: {
    notFound: 'Ishga tushirish topilmadi',
    notFoundText: 'U o‘chirilgan yoki boshqa ish maydoniga tegishli.',
    back: 'Ishga tushirishlar',
    live: 'Jonli',
    reconnecting: 'Qayta ulanmoqda…',
    finished: 'Tugadi',
    onVersion: (v) => `${v}-versiyadagi ishga tushirish`,
    openVersion: (v) => `v${v} ni ochish`,
    openSubject: (label) => `${label} ni ochish`,
    startedBy: (trigger) => `Ishga tushirgan: ${trigger}`,
    started: (time) => `Boshlangan: ${time}`,
    loadFailed: 'Ishga tushirishni yuklab bo‘lmadi. Bir daqiqadan keyin qayta urinib ko‘ring.',
    canvas: 'Ishga tushirish grafi',
    id: 'Ishga tushirish ID',
    cancel: 'Bekor qilish',
    cancelling: 'Bekor qilinmoqda…',
    retry: 'Chaqiruvni takrorlash',
    retrying: 'Takrorlanmoqda…',
    resolve: 'Hal qilish',
    resolving: 'Hal qilinmoqda…',
    cancelTitle: 'Bu ishga tushirish bekor qilinsinmi?',
    cancelText: 'Ishga tushirish turgan joyida to‘xtaydi. Bajarilgan qadamlar qaytarilmaydi.',
    keep: 'Davom etsin',
    resolveTitle: (node) => `«${node}» ni hal qilish`,
    resolveText: 'Bloklangan qadam qayta chaqirilmaydi: ishga tushirish tanlangan natija bo‘yicha davom etadi.',
    outcome: 'Natija',
    pickOutcome: 'Natijani tanlang',
    stillWaiting: (what) => `Hali kutilmoqda: ${what}. Hech narsa ikki marta bajarilmaydi.`,
    actionFailed: (what) => `${what} amalga oshmadi. Bir daqiqadan keyin qayta urinib ko‘ring.`,
    finishedAlready: 'Ishga tushirish allaqachon tugagan.',
    cancelled: 'Ishga tushirish bekor qilindi',
    retried: 'Chaqiruv takrorlanmoqda',
    resolved: 'Ishga tushirish davom etmoqda',
    what: { cancel: 'Bekor qilish', retry: 'Chaqiruvni takrorlash', resolve: 'Qadamni hal qilish' },
    nodeLabel: 'Qadam',
  },
  node: {
    input: 'Kirish',
    output: 'Chiqish',
    logs: 'Loglar',
    outcome: 'Natija',
    error: 'Xato',
    noData: 'Tugunda ma’lumot yo‘q',
    hidden: 'Qadam siz ko‘ra olmaydigan ma’lumotni o‘qigan: uning kirishi va chiqishi yashirilgan.',
    notReached: 'Ishga tushirish bu qadamga yetmadi.',
    filterLogs: 'Loglarni filtrlash',
    noLogMatch: (q) => `«${q}» ga mos log qatori yo‘q`,
    logCount: (shown, total) => `${total} tadan ${shown} ta`,
    noLogs: 'Loglar yo‘q',
    copy: 'Nusxalash',
    copied: 'Nusxalandi',
    loadFull: 'To‘liq yuklash',
    folded: (size) => `Chiqish hajmi ${size}; u yig‘ilgan holda ko‘rsatilgan.`,
    attempt: (a) => `${a}-urinish`,
    item: (i) => `${i}-element`,
    waiting: { event: 'Hodisani kutmoqda', timer: 'Taymerni kutmoqda', human: 'Odamni kutmoqda', call: 'Chaqiruvni kutmoqda' },
    until: (time) => `${time} gacha`,
    duration: (text) => `${text} davom etdi`,
    close: 'Yopish',
    steps: (c) => n('uz', c, { one: '{n} ta qadam', other: '{n} ta qadam' }),
    expand: 'Yoyish',
    collapse: 'Yig‘ish',
    empty: 'bo‘sh',
  },
  dry: {
    title: 'Sinov prog‘oni',
    note: 'Tashqariga hech narsa yuborilmaydi: effektlar ushlanadi va pastda ko‘rsatiladi.',
    subject: 'Obyekt',
    subjectNone: 'Obyektsiz',
    subjectSearch: 'Obyektni topish',
    events: 'Hodisalar (JSON massiv)',
    eventsHint: 'Grafni ishga tushiradigan hodisa, keyin uning kutishlari oladiganlari.',
    invalidJson: 'Bu obyektlarning JSON massivi emas.',
    context: 'Javoblar (JSON)',
    contextHint: 'Prog‘on chaqirish o‘rniga o‘qiydigan nodes.<id>.output va obyekt maydonlari.',
    decisions: 'Qarorlar',
    modes: { cached: 'Keshdan', live: 'Jonli chaqiruv', scripted: 'Javobni berish' },
    modeHints: {
      cached: 'Avval qabul qilingan qaror keshdan olinadi; yangisi bir marta jonli qabul qilinib keshlanadi.',
      live: 'Qarorlar har safar haqiqatan qabul qilinadi; provayder ular uchun haq olishi mumkin.',
      scripted: 'Qarorlar berilgan kontekstdan olinadi; hech narsa chaqirilmaydi.',
    },
    run: 'Prog‘on qilish',
    running: 'Prog‘on qilinmoqda…',
    stillWaiting: 'Prog‘on hali kutilmoqda. Tashqariga hech narsa yuborilmaydi.',
    failed: 'Sinov prog‘onini bajarib bo‘lmadi. Bir daqiqadan keyin qayta urinib ko‘ring.',
    path: 'Yo‘l',
    notTriggered: 'Trigger ishlamadi: uning filtri o‘tmadi.',
    condition: 'Shart',
    wouldSend: 'Nima yuborilgan bo‘lardi',
    nothingSent: 'Hech narsa yuborilmagan bo‘lardi.',
    live: (c) => n('uz', c, { one: '{n} ta jonli qaror', other: '{n} ta jonli qaror' }),
    ended: 'Ishga tushirish tugadi',
    waits: (node) => `Ishga tushirish «${node}» da kutmoqda`,
    missing: (path) => `Ma’lumot yetishmaydi: ${path}`,
    error: 'Xato bilan to‘xtadi',
    close: 'Sinov prog‘onini yopish',
    empty: 'Obyekt va hodisani tanlang, keyin prog‘on qiling.',
    captured: 'ushlangan',
    executed: 'jonli',
  },
  test: {
    open: 'Qadamni sinash',
    title: (node) => `«${node}» ni sinash`,
    note: 'Qadam shu kirish bilan haqiqatan bajariladi.',
    input: 'Kirish (JSON)',
    example: 'Oxirgi ishga tushirishdan',
    exampleNone: 'Hali hech bir ishga tushirish bu qadamga yetmagan.',
    run: 'Sinash',
    running: 'Sinalmoqda…',
    stillWaiting: 'Qadam hali kutilmoqda. U ikki marta bajarilmaydi.',
    result: 'Natija',
    outcome: 'Natija',
    duration: (ms) => `${ms} ms`,
    failed: 'Qadamni sinab bo‘lmadi. Bir daqiqadan keyin qayta urinib ko‘ring.',
    secretMissing: (name) => `${name} siri berilmagan`,
    secretUnset: 'berilmagan',
    confirmTitle: 'Qadam haqiqatan bajarilsinmi?',
    confirmText: (connection) => `Qadam ${connection} ga murojaat qiladi.`,
    noConnection: 'xizmat',
    irreversible: 'Effektni qaytarib bo‘lmaydi',
    confirm: 'Bajarish',
    cancel: 'Bekor qilish',
    close: 'Sinovni yopish',
    invalidJson: 'Bu JSON obyekt emas.',
    secrets: 'Sirlar',
    logs: 'Loglar',
    output: 'Chiqish',
  },
}

const uzCyrl: RunStrings = {
  list: {
    label: 'Графнинг ишга туширишлари',
    search: 'Объект бўйича қидириш',
    searchLabel: 'Объект',
    filter: 'Ҳолат',
    states: { running: 'Бажарилмоқда', waiting: 'Кутмоқда', blocked: 'Блокланган', failed: 'Хато', done: 'Тайёр', cancelled: 'Бекор қилинган' },
    noSubject: 'Объектсиз',
    empty: 'Ҳали ишга туширишлар йўқ',
    emptyText: 'Графнинг триггери ҳар сафар ишлаганда бу ерда ишга тушириш пайдо бўлади.',
    start: 'Ишга тушириш',
    emptyFilter: 'Фильтрга мос ишга туширишлар йўқ',
    reset: 'Фильтрларни тозалаш',
    more: 'Яна кўрсатиш',
    loading: 'Ишга туширишлар юкланмоқда…',
    loadFailed: 'Ишга туширишларни юклаб бўлмади. Бир дақиқадан кейин қайта уриниб кўринг.',
    stale: (time) => `Янгилаб бўлмади · маълумотлар ${time} ҳолатига`,
    count: (c) => n('uz-Cyrl', c, { one: '{n} та ишга тушириш', other: '{n} та ишга тушириш' }),
  },
  run: {
    notFound: 'Ишга тушириш топилмади',
    notFoundText: 'У ўчирилган ёки бошқа иш майдонига тегишли.',
    back: 'Ишга туширишлар',
    live: 'Жонли',
    reconnecting: 'Қайта уланмоқда…',
    finished: 'Тугади',
    onVersion: (v) => `${v}-версиядаги ишга тушириш`,
    openVersion: (v) => `v${v} ни очиш`,
    openSubject: (label) => `${label} ни очиш`,
    startedBy: (trigger) => `Ишга туширган: ${trigger}`,
    started: (time) => `Бошланган: ${time}`,
    loadFailed: 'Ишга туширишни юклаб бўлмади. Бир дақиқадан кейин қайта уриниб кўринг.',
    canvas: 'Ишга тушириш графи',
    id: 'Ишга тушириш ID',
    cancel: 'Бекор қилиш',
    cancelling: 'Бекор қилинмоқда…',
    retry: 'Чақирувни такрорлаш',
    retrying: 'Такрорланмоқда…',
    resolve: 'Ҳал қилиш',
    resolving: 'Ҳал қилинмоқда…',
    cancelTitle: 'Бу ишга тушириш бекор қилинсинми?',
    cancelText: 'Ишга тушириш турган жойида тўхтайди. Бажарилган қадамлар қайтарилмайди.',
    keep: 'Давом этсин',
    resolveTitle: (node) => `«${node}» ни ҳал қилиш`,
    resolveText: 'Блокланган қадам қайта чақирилмайди: ишга тушириш танланган натижа бўйича давом этади.',
    outcome: 'Натижа',
    pickOutcome: 'Натижани танланг',
    stillWaiting: (what) => `Ҳали кутилмоқда: ${what}. Ҳеч нарса икки марта бажарилмайди.`,
    actionFailed: (what) => `${what} амалга ошмади. Бир дақиқадан кейин қайта уриниб кўринг.`,
    finishedAlready: 'Ишга тушириш аллақачон тугаган.',
    cancelled: 'Ишга тушириш бекор қилинди',
    retried: 'Чақирув такрорланмоқда',
    resolved: 'Ишга тушириш давом этмоқда',
    what: { cancel: 'Бекор қилиш', retry: 'Чақирувни такрорлаш', resolve: 'Қадамни ҳал қилиш' },
    nodeLabel: 'Қадам',
  },
  node: {
    input: 'Кириш',
    output: 'Чиқиш',
    logs: 'Логлар',
    outcome: 'Натижа',
    error: 'Хато',
    noData: 'Тугунда маълумот йўқ',
    hidden: 'Қадам сиз кўра олмайдиган маълумотни ўқиган: унинг кириши ва чиқиши яширилган.',
    notReached: 'Ишга тушириш бу қадамга етмади.',
    filterLogs: 'Логларни фильтрлаш',
    noLogMatch: (q) => `«${q}» га мос лог қатори йўқ`,
    logCount: (shown, total) => `${total} тадан ${shown} та`,
    noLogs: 'Логлар йўқ',
    copy: 'Нусхалаш',
    copied: 'Нусхаланди',
    loadFull: 'Тўлиқ юклаш',
    folded: (size) => `Чиқиш ҳажми ${size}; у йиғилган ҳолда кўрсатилган.`,
    attempt: (a) => `${a}-уриниш`,
    item: (i) => `${i}-элемент`,
    waiting: { event: 'Ҳодисани кутмоқда', timer: 'Таймерни кутмоқда', human: 'Одамни кутмоқда', call: 'Чақирувни кутмоқда' },
    until: (time) => `${time} гача`,
    duration: (text) => `${text} давом этди`,
    close: 'Ёпиш',
    steps: (c) => n('uz-Cyrl', c, { one: '{n} та қадам', other: '{n} та қадам' }),
    expand: 'Ёйиш',
    collapse: 'Йиғиш',
    empty: 'бўш',
  },
  dry: {
    title: 'Синов прогони',
    note: 'Ташқарига ҳеч нарса юборилмайди: эффектлар ушланади ва пастда кўрсатилади.',
    subject: 'Объект',
    subjectNone: 'Объектсиз',
    subjectSearch: 'Объектни топиш',
    events: 'Ҳодисалар (JSON массив)',
    eventsHint: 'Графни ишга туширадиган ҳодиса, кейин унинг кутишлари оладиганлари.',
    invalidJson: 'Бу объектларнинг JSON массиви эмас.',
    context: 'Жавоблар (JSON)',
    contextHint: 'Прогон чақириш ўрнига ўқийдиган nodes.<id>.output ва объект майдонлари.',
    decisions: 'Қарорлар',
    modes: { cached: 'Кешдан', live: 'Жонли чақирув', scripted: 'Жавобни бериш' },
    modeHints: {
      cached: 'Аввал қабул қилинган қарор кешдан олинади; янгиси бир марта жонли қабул қилиниб кешланади.',
      live: 'Қарорлар ҳар сафар ҳақиқатан қабул қилинади; провайдер улар учун ҳақ олиши мумкин.',
      scripted: 'Қарорлар берилган контекстдан олинади; ҳеч нарса чақирилмайди.',
    },
    run: 'Прогон қилиш',
    running: 'Прогон қилинмоқда…',
    stillWaiting: 'Прогон ҳали кутилмоқда. Ташқарига ҳеч нарса юборилмайди.',
    failed: 'Синов прогонини бажариб бўлмади. Бир дақиқадан кейин қайта уриниб кўринг.',
    path: 'Йўл',
    notTriggered: 'Триггер ишламади: унинг фильтри ўтмади.',
    condition: 'Шарт',
    wouldSend: 'Нима юборилган бўларди',
    nothingSent: 'Ҳеч нарса юборилмаган бўларди.',
    live: (c) => n('uz-Cyrl', c, { one: '{n} та жонли қарор', other: '{n} та жонли қарор' }),
    ended: 'Ишга тушириш тугади',
    waits: (node) => `Ишга тушириш «${node}» да кутмоқда`,
    missing: (path) => `Маълумот етишмайди: ${path}`,
    error: 'Хато билан тўхтади',
    close: 'Синов прогонини ёпиш',
    empty: 'Объект ва ҳодисани танланг, кейин прогон қилинг.',
    captured: 'ушланган',
    executed: 'жонли',
  },
  test: {
    open: 'Қадамни синаш',
    title: (node) => `«${node}» ни синаш`,
    note: 'Қадам шу кириш билан ҳақиқатан бажарилади.',
    input: 'Кириш (JSON)',
    example: 'Охирги ишга туширишдан',
    exampleNone: 'Ҳали ҳеч бир ишга тушириш бу қадамга етмаган.',
    run: 'Синаш',
    running: 'Синалмоқда…',
    stillWaiting: 'Қадам ҳали кутилмоқда. У икки марта бажарилмайди.',
    result: 'Натижа',
    outcome: 'Натижа',
    duration: (ms) => `${ms} мс`,
    failed: 'Қадамни синаб бўлмади. Бир дақиқадан кейин қайта уриниб кўринг.',
    secretMissing: (name) => `${name} сири берилмаган`,
    secretUnset: 'берилмаган',
    confirmTitle: 'Қадам ҳақиқатан бажарилсинми?',
    confirmText: (connection) => `Қадам ${connection} га мурожаат қилади.`,
    noConnection: 'хизмат',
    irreversible: 'Эффектни қайтариб бўлмайди',
    confirm: 'Бажариш',
    cancel: 'Бекор қилиш',
    close: 'Синовни ёпиш',
    invalidJson: 'Бу JSON объект эмас.',
    secrets: 'Сирлар',
    logs: 'Логлар',
    output: 'Чиқиш',
  },
}

const table: Record<EditorLocale, RunStrings> = { en, ru, uz, 'uz-Cyrl': uzCyrl }

export function runStrings(locale: EditorLocale | undefined): RunStrings {
  return pick(table, locale)
}

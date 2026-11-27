// Interface strings of the editor in en, ru, uz and uz-Cyrl. Names of nodes and events come from the
// catalog, already localized by the host.
import { pick, plural } from './locale'
import type { EditorLocale, ItemState } from './types'

type Plural = { one: string; few?: string; many?: string; other: string }

export type Strings = {
  canvas: {
    label: string
    hintMouse: string
    hintTouch: string
    hintReadOnly: string
    hintConnecting: string
    rewire(from: string, to: string, was: string): string
    connect(from: string, to: string): string
    nowhere: string
    notConnected: string
    ignored: string
    unavailable: string
    noConnection: string
    zoomIn: string
    zoomOut: string
    fit: string
    tidy: string
    zoomReset: string
    minimap: string
    trigger: string
    error: string
    states: Record<ItemState, string>
    selectMode: string
    add: string
    node(title: string): string
    outcome(name: string, target: string): string
  }
  palette: {
    title: string
    titleFor(outcome: string): string
    titleConnect(outcome: string): string
    search: string
    triggers: string
    steps: string
    count(n: number, total: number): string
    nothing(query: string): string
    noEvents: string
    close: string
    notConnected: string
    noConnection: string
    existing: string
  }
  menu: {
    label: string
    open: string
    delete: string
    deleteMany(n: number): string
    duplicate: string
    copy: string
    paste: string
    connectTo(outcome: string): string
    ignore(outcome: string): string
    unignore(outcome: string): string
    snap: string
    addHere: string
    selectAll: string
  }
  toolbar: {
    dryRun: string
    testNode: string
    label: string
    edit: string
    runs: string
    validate: string
    validating: string
    undo: string
    redo: string
    tidy: string
    add: string
    settings: string
    save: string
    saving: string
    enabled: string
    keys: string
    unsaved: string
    versions: string
    version(v: number): string
    newGraph: string
  }
  panel: {
    label: string
    configure: string
    data: string
    close: string
    output: string
    noOutput: string
    copy: string
    copied: string
    event: string
    eventPlaceholder: string
    action: string
    actionPlaceholder: string
    actionMissing(key: string): string
    actionPick: string
    filter: string
    filterHint: string
    params: string
    schedule: string
    id: string
    unknownType(type: string): string
    eventMissing: string
  }
  validation: {
    title: string
    errors(n: number): string
    warnings(n: number): string
    none: string
    notChecked: string
    checking: string
    failed: string
    retry: string
    unavailable(type: string): string
    noConnection(title: string): string
    close: string
    graph: string
  }
  settings: {
    title: string
    name: string
    description: string
    runAs: string
    owner: string
    ownerHint: string
    initiator: string
    initiatorHint: string
    done: string
  }
  save: {
    pendingReview: string
    conflict(by: string, version: number): string
    loadLatest: string
    stay: string
    replaceTitle: string
    replaceText(version: number): string
    replace: string
    cancel: string
    failed: string
    hasErrors: string
    offline: string
    retry: string
    saved(version: number): string
    layoutSaving: string
    layoutSaved: string
    layoutFailed: string
    reloadFailed: string
    leave: string
    stillWaiting: string
    saveOver(version: number): string
    saved_: string
  }
  states: {
    readOnly: string
    mobile: string
    emptyTitle: string
    emptyText: string
    addTrigger: string
    loading: string
  }
  keymap: {
    title: string
    gestures: string
    keys: string
    close: string
    rows: [string, string][]
    gestureRows: [string, string][]
  }
  outline: {
    label: string
    triggers: string
    steps: string
    unreachable: string
    empty: string
    goes(outcome: string, target: string): string
  }
  common: { close: string; retry: string; undo: string }
}

const n = (locale: EditorLocale, count: number, forms: Plural) => plural(locale, count, forms)

const en: Strings = {
  canvas: {
    label: 'Automation graph',
    hintMouse: 'Drag an outcome onto a step to connect · Drop on empty space to add · ⌘K add · ? keys',
    hintTouch: 'Drag an outcome to a step to connect · Long press for the menu · Pinch to zoom',
    hintReadOnly: 'Read only: pan and zoom to look around',
    hintConnecting: 'Release on a step to connect, on empty space to add one, elsewhere to cancel',
    rewire: (from, to, was) => `${from} → ${to} · was → ${was}`,
    connect: (from, to) => `${from} → ${to}`,
    nowhere: 'nothing',
    notConnected: 'not connected',
    ignored: 'ignored',
    unavailable: 'Unavailable',
    noConnection: 'No connection',
    zoomIn: 'Zoom in',
    zoomOut: 'Zoom out',
    fit: 'Fit to screen',
    tidy: 'Tidy up',
    zoomReset: 'Zoom to 100%',
    minimap: 'Minimap',
    trigger: 'Trigger',
    error: 'Has errors',
    states: { passed: 'Passed', running: 'Running', waiting: 'Waiting', failed: 'Failed', skipped: 'Skipped', cancelled: 'Cancelled' },
    selectMode: 'Select',
    add: 'Add',
    node: (title) => `Step ${title}`,
    outcome: (name, target) => `Outcome ${name}: ${target}`,
  },
  palette: {
    title: 'Add a step',
    titleFor: (outcome) => `Add a step after “${outcome}”`,
    titleConnect: (outcome) => `Connect “${outcome}” to…`,
    search: 'Search steps and triggers',
    triggers: 'Triggers',
    steps: 'Steps',
    count: (k, total) => `${k} of ${total}`,
    nothing: (query) => `Nothing matches “${query}”`,
    noEvents: 'No events available: check integrations and permissions',
    close: 'Close',
    notConnected: 'Not connected',
    noConnection: 'no connection',
    existing: 'Steps on the canvas',
  },
  menu: {
    label: 'Actions',
    open: 'Open',
    delete: 'Delete',
    deleteMany: (k) => n('en', k, { one: 'Delete {n} step', other: 'Delete {n} steps' }),
    duplicate: 'Duplicate',
    copy: 'Copy',
    paste: 'Paste',
    connectTo: (outcome) => `Connect ${outcome} to…`,
    ignore: (outcome) => `Ignore ${outcome}`,
    unignore: (outcome) => `Stop ignoring ${outcome}`,
    snap: 'Snap to grid',
    addHere: 'Add a step here',
    selectAll: 'Select all',
  },
  toolbar: {
    dryRun: 'Dry run',
    testNode: 'Test step',
    label: 'Editor toolbar',
    edit: 'Edit',
    runs: 'Runs',
    validate: 'Validate',
    validating: 'Validating…',
    undo: 'Undo',
    redo: 'Redo',
    tidy: 'Tidy',
    add: 'Add',
    settings: 'Settings',
    save: 'Save',
    saving: 'Saving…',
    enabled: 'Enabled',
    keys: 'Keys',
    unsaved: 'Unsaved changes',
    versions: 'Versions',
    version: (v) => `Version ${v}`,
    newGraph: 'New automation',
  },
  panel: {
    label: 'Step settings',
    configure: 'Configure',
    data: 'Data',
    close: 'Close panel',
    output: 'Output of this step',
    noOutput: 'This step has no output.',
    copy: 'Copy',
    copied: 'Copied',
    event: 'Event',
    eventPlaceholder: 'Choose an event',
    action: 'Action',
    actionPlaceholder: 'Choose an action',
    actionMissing: (key) => `The action “${key}” is not available: its module may be off or you lack the permission.`,
    actionPick: 'Choose the action this step runs; its fields appear here.',
    filter: 'Filter',
    filterHint: 'CEL condition on the event; empty means every event',
    params: 'Parameters',
    schedule: 'Schedule',
    id: 'ID',
    unknownType: (type) => `The step type “${type}” is not available: its connection may be missing or you lack the permission.`,
    eventMissing: 'This event is not available any more.',
  },
  validation: {
    title: 'Validation',
    errors: (k) => n('en', k, { one: '{n} error', other: '{n} errors' }),
    warnings: (k) => n('en', k, { one: '{n} warning', other: '{n} warnings' }),
    none: 'No problems found',
    notChecked: 'Not checked yet',
    checking: 'Checking…',
    failed: 'Could not validate. Your draft is kept.',
    retry: 'Retry',
    unavailable: (type) => `Step type “${type}” is not available: check its connection`,
    noConnection: (title) => `“${title}”: its connection is unavailable. Reconnect it in Integrations or pick another connection.`,
    close: 'Close validation',
    graph: 'Automation',
  },
  settings: {
    title: 'Automation settings',
    name: 'Name',
    description: 'Description',
    runAs: 'Run as',
    owner: 'Owner',
    ownerHint: 'Runs with the rights of the person who last enabled the automation.',
    initiator: 'Initiator',
    initiatorHint: 'Runs with the rights of the user who caused the event; falls back to the owner when there is none.',
    done: 'Done',
  },
  save: {
    pendingReview: 'The new version is saved but not enabled.',
    conflict: (by, v) => `Saved by another user (${by}, version ${v}).`,
    loadLatest: 'Load latest',
    stay: 'Stay',
    replaceTitle: 'Load the latest version?',
    replaceText: (v) => `Your draft will be replaced by version ${v}. Unsaved changes will be lost.`,
    replace: 'Replace my draft',
    cancel: 'Cancel',
    failed: 'Could not save. Your draft is kept.',
    hasErrors: 'Fix the errors to save.',
    offline: 'No connection',
    retry: 'Retry',
    saved: (v) => `Saved as version ${v}`,
    layoutSaving: 'Saving layout…',
    layoutSaved: 'Layout saved',
    layoutFailed: 'Layout not saved',
    reloadFailed: 'Could not load the latest version.',
    leave: 'You have unsaved changes. Leave anyway?',
    stillWaiting: 'Still waiting for the server; nothing will be sent twice.',
    saveOver: (v) => `Save over v${v}`,
    saved_: 'Saved',
  },
  states: {
    readOnly: 'Read only',
    mobile: 'Editing is available on a larger screen',
    emptyTitle: 'Add a trigger',
    emptyText: 'An automation starts with a trigger: an event, a schedule or a manual start.',
    addTrigger: 'Add a trigger',
    loading: 'Loading the editor',
  },
  keymap: {
    title: 'Keyboard shortcuts',
    gestures: 'Gestures',
    keys: 'Keys',
    close: 'Close',
    rows: [
      ['⌘K', 'Add a step or trigger'],
      ['← ↑ → ↓', 'Move to the nearest step'],
      ['Enter', 'Open the step panel'],
      ['Esc', 'Close the top layer'],
      ['Delete', 'Delete the selection'],
      ['Shift+F10', 'Step menu: connect, duplicate, delete'],
      ['Alt+← ↑ → ↓', 'Move the selected step'],
      ['⌘C · ⌘V · ⌘D', 'Copy · paste · duplicate'],
      ['⌘A', 'Select all'],
      ['⌘Z · ⇧⌘Z', 'Undo · redo'],
      ['⌘S', 'Save'],
      ['⌘Enter', 'Validate'],
      ['⇧1 · ⇧0', 'Fit · zoom to 100%'],
      ['⇧⌘L', 'Tidy up the layout'],
      ['?', 'This list'],
    ],
    gestureRows: [
      ['Drag a step', 'Move it'],
      ['Drag an outcome', 'Connect it; release on empty space to add a step'],
      ['Long press', 'Menu of a step or of the canvas'],
      ['Pinch', 'Zoom'],
      ['Drag the canvas', 'Pan'],
      ['Select, then drag', 'Select several steps'],
    ],
  },
  outline: {
    label: 'Automation outline',
    triggers: 'Triggers',
    steps: 'Steps',
    unreachable: 'Not reachable from a trigger',
    empty: 'This automation has no triggers or steps yet.',
    goes: (outcome, target) => `${outcome} → ${target}`,
  },
  common: { close: 'Close', retry: 'Retry', undo: 'Undo' },
}

const ru: Strings = {
  canvas: {
    label: 'Граф автоматизации',
    hintMouse: 'Протяните исход на шаг, чтобы соединить · Отпустите на пустом месте, чтобы добавить · ⌘K — добавить · ? — клавиши',
    hintTouch: 'Протяните исход к шагу, чтобы соединить · Долгое нажатие — меню · Щипок — масштаб',
    hintReadOnly: 'Только чтение: двигайте и масштабируйте холст',
    hintConnecting: 'Отпустите на шаге — соединить, на пустом месте — добавить шаг, в другом месте — отмена',
    rewire: (from, to, was) => `${from} → ${to} · было → ${was}`,
    connect: (from, to) => `${from} → ${to}`,
    nowhere: 'ничего',
    notConnected: 'не подключён',
    ignored: 'игнорируется',
    unavailable: 'Недоступно',
    noConnection: 'Нет подключения',
    zoomIn: 'Приблизить',
    zoomOut: 'Отдалить',
    fit: 'Вместить',
    tidy: 'Упорядочить',
    zoomReset: 'Масштаб 100 %',
    minimap: 'Мини-карта',
    trigger: 'Триггер',
    error: 'Есть ошибки',
    states: { passed: 'Пройден', running: 'Выполняется', waiting: 'Ожидает', failed: 'Ошибка', skipped: 'Пропущен', cancelled: 'Отменён' },
    selectMode: 'Выделить',
    add: 'Добавить',
    node: (title) => `Шаг ${title}`,
    outcome: (name, target) => `Исход ${name}: ${target}`,
  },
  palette: {
    title: 'Добавить шаг',
    titleFor: (outcome) => `Шаг после «${outcome}»`,
    titleConnect: (outcome) => `Подключить «${outcome}» к…`,
    search: 'Поиск шагов и триггеров',
    triggers: 'Триггеры',
    steps: 'Шаги',
    count: (k, total) => `${k} из ${total}`,
    nothing: (query) => `Ничего не подходит под «${query}»`,
    noEvents: 'Нет доступных событий: проверьте интеграции и права',
    close: 'Закрыть',
    notConnected: 'Не подключено',
    noConnection: 'нет подключения',
    existing: 'Шаги на холсте',
  },
  menu: {
    label: 'Действия',
    open: 'Открыть',
    delete: 'Удалить',
    deleteMany: (k) => n('ru', k, { one: 'Удалить {n} шаг', few: 'Удалить {n} шага', many: 'Удалить {n} шагов', other: 'Удалить {n} шага' }),
    duplicate: 'Дублировать',
    copy: 'Копировать',
    paste: 'Вставить',
    connectTo: (outcome) => `Подключить ${outcome} к…`,
    ignore: (outcome) => `Игнорировать ${outcome}`,
    unignore: (outcome) => `Не игнорировать ${outcome}`,
    snap: 'Привязка к сетке',
    addHere: 'Добавить шаг сюда',
    selectAll: 'Выделить всё',
  },
  toolbar: {
    dryRun: 'Пробный прогон',
    testNode: 'Проверить шаг',
    label: 'Панель редактора',
    edit: 'Редактор',
    runs: 'Запуски',
    validate: 'Проверить',
    validating: 'Проверяем…',
    undo: 'Отменить',
    redo: 'Повторить',
    tidy: 'Упорядочить',
    add: 'Добавить',
    settings: 'Настройки',
    save: 'Сохранить',
    saving: 'Сохраняем…',
    enabled: 'Включена',
    keys: 'Клавиши',
    unsaved: 'Есть несохранённые правки',
    versions: 'Версии',
    version: (v) => `Версия ${v}`,
    newGraph: 'Новая автоматизация',
  },
  panel: {
    label: 'Настройки шага',
    configure: 'Настройка',
    data: 'Данные',
    close: 'Закрыть панель',
    output: 'Выход шага',
    noOutput: 'У этого шага нет выхода.',
    copy: 'Копировать',
    copied: 'Скопировано',
    event: 'Событие',
    eventPlaceholder: 'Выберите событие',
    action: 'Действие',
    actionPlaceholder: 'Выберите действие',
    actionMissing: (key) => `Действие «${key}» недоступно: возможно, модуль выключен или нет прав.`,
    actionPick: 'Выберите действие, которое выполняет шаг; его поля появятся здесь.',
    filter: 'Фильтр',
    filterHint: 'Условие CEL на событие; пусто — каждое событие',
    params: 'Параметры',
    schedule: 'Расписание',
    id: 'ID',
    unknownType: (type) => `Тип шага «${type}» недоступен: возможно, нет подключения или прав.`,
    eventMissing: 'Это событие больше недоступно.',
  },
  validation: {
    title: 'Проверка',
    errors: (k) => n('ru', k, { one: '{n} ошибка', few: '{n} ошибки', many: '{n} ошибок', other: '{n} ошибки' }),
    warnings: (k) => n('ru', k, { one: '{n} предупреждение', few: '{n} предупреждения', many: '{n} предупреждений', other: '{n} предупреждения' }),
    none: 'Проблем не найдено',
    notChecked: 'Ещё не проверялось',
    checking: 'Проверяем…',
    failed: 'Не удалось проверить. Черновик сохранён.',
    retry: 'Повторить',
    unavailable: (type) => `Тип шага «${type}» недоступен: проверьте подключение`,
    noConnection: (title) => `«${title}»: подключение недоступно. Переподключите его в «Интеграциях» или выберите другое.`,
    close: 'Закрыть проверку',
    graph: 'Автоматизация',
  },
  settings: {
    title: 'Настройки автоматизации',
    name: 'Название',
    description: 'Описание',
    runAs: 'Выполнять от имени',
    owner: 'Владельца',
    ownerHint: 'С правами человека, который последним включил автоматизацию.',
    initiator: 'Инициатора',
    initiatorHint: 'С правами пользователя, вызвавшего событие; если его нет — от владельца.',
    done: 'Готово',
  },
  save: {
    pendingReview: 'Новая версия сохранена, но не включена.',
    conflict: (by, v) => `Сохранено другим пользователем (${by}, версия ${v}).`,
    loadLatest: 'Загрузить последнюю',
    stay: 'Остаться',
    replaceTitle: 'Загрузить последнюю версию?',
    replaceText: (v) => `Черновик заменится версией ${v}. Несохранённые правки будут потеряны.`,
    replace: 'Заменить черновик',
    cancel: 'Отмена',
    failed: 'Не удалось сохранить. Черновик сохранён.',
    hasErrors: 'Исправьте ошибки, чтобы сохранить.',
    offline: 'Нет соединения',
    retry: 'Повторить',
    saved: (v) => `Сохранено как версия ${v}`,
    layoutSaving: 'Сохраняем раскладку…',
    layoutSaved: 'Раскладка сохранена',
    layoutFailed: 'Раскладка не сохранена',
    reloadFailed: 'Не удалось загрузить последнюю версию.',
    leave: 'Есть несохранённые правки. Всё равно уйти?',
    stillWaiting: 'Всё ещё ждём сервер; ничего не отправится дважды.',
    saveOver: (v) => `Сохранить поверх v${v}`,
    saved_: 'Сохранено',
  },
  states: {
    readOnly: 'Только чтение',
    mobile: 'Редактирование доступно на большом экране',
    emptyTitle: 'Добавьте триггер',
    emptyText: 'Автоматизация начинается с триггера: события, расписания или ручного запуска.',
    addTrigger: 'Добавить триггер',
    loading: 'Загружаем редактор',
  },
  keymap: {
    title: 'Клавиши',
    gestures: 'Жесты',
    keys: 'Клавиши',
    close: 'Закрыть',
    rows: [
      ['⌘K', 'Добавить шаг или триггер'],
      ['← ↑ → ↓', 'К ближайшему шагу'],
      ['Enter', 'Открыть панель шага'],
      ['Esc', 'Закрыть верхний слой'],
      ['Delete', 'Удалить выделенное'],
      ['Shift+F10', 'Меню шага: подключить, дублировать, удалить'],
      ['Alt+← ↑ → ↓', 'Сдвинуть выделенный шаг'],
      ['⌘C · ⌘V · ⌘D', 'Копировать · вставить · дублировать'],
      ['⌘A', 'Выделить всё'],
      ['⌘Z · ⇧⌘Z', 'Отменить · повторить'],
      ['⌘S', 'Сохранить'],
      ['⌘Enter', 'Проверить'],
      ['⇧1 · ⇧0', 'Вместить · 100 %'],
      ['⇧⌘L', 'Упорядочить раскладку'],
      ['?', 'Этот список'],
    ],
    gestureRows: [
      ['Тяните шаг', 'Переместить'],
      ['Тяните исход', 'Соединить; отпустите на пустом месте — добавить шаг'],
      ['Долгое нажатие', 'Меню шага или холста'],
      ['Щипок', 'Масштаб'],
      ['Тяните холст', 'Панорама'],
      ['«Выделить», затем тяните', 'Выделить несколько шагов'],
    ],
  },
  outline: {
    label: 'Структура автоматизации',
    triggers: 'Триггеры',
    steps: 'Шаги',
    unreachable: 'Недостижим из триггера',
    empty: 'В автоматизации ещё нет триггеров и шагов.',
    goes: (outcome, target) => `${outcome} → ${target}`,
  },
  common: { close: 'Закрыть', retry: 'Повторить', undo: 'Отменить' },
}

const uz: Strings = {
  canvas: {
    label: 'Avtomatlashtirish grafi',
    hintMouse: 'Ulash uchun natijani qadamga torting · Qoʻshish uchun boʻsh joyga qoʻying · ⌘K — qoʻshish · ? — tugmalar',
    hintTouch: 'Ulash uchun natijani qadamga torting · Uzoq bosish — menyu · Chimdish — masshtab',
    hintReadOnly: 'Faqat koʻrish: tuvalni suring va masshtablang',
    hintConnecting: 'Qadam ustida qoʻying — ulash, boʻsh joyda — qadam qoʻshish, boshqa joyda — bekor',
    rewire: (from, to, was) => `${from} → ${to} · edi → ${was}`,
    connect: (from, to) => `${from} → ${to}`,
    nowhere: 'hech narsa',
    notConnected: 'ulanmagan',
    ignored: 'eʼtiborsiz',
    unavailable: 'Mavjud emas',
    noConnection: 'Ulanish yoʻq',
    zoomIn: 'Yaqinlashtirish',
    zoomOut: 'Uzoqlashtirish',
    fit: 'Sigʻdirish',
    tidy: 'Tartiblash',
    zoomReset: 'Masshtab 100 %',
    minimap: 'Mini xarita',
    trigger: 'Trigger',
    error: 'Xatolar bor',
    states: { passed: 'Oʻtdi', running: 'Bajarilmoqda', waiting: 'Kutmoqda', failed: 'Xato', skipped: 'Oʻtkazib yuborildi', cancelled: 'Bekor qilindi' },
    selectMode: 'Belgilash',
    add: 'Qoʻshish',
    node: (title) => `Qadam ${title}`,
    outcome: (name, target) => `Natija ${name}: ${target}`,
  },
  palette: {
    title: 'Qadam qoʻshish',
    titleFor: (outcome) => `«${outcome}» dan keyingi qadam`,
    titleConnect: (outcome) => `«${outcome}» ni ulash…`,
    search: 'Qadam va triggerlarni qidirish',
    triggers: 'Triggerlar',
    steps: 'Qadamlar',
    count: (k, total) => `${total} dan ${k}`,
    nothing: (query) => `«${query}» boʻyicha hech narsa topilmadi`,
    noEvents: 'Mavjud hodisalar yoʻq: integratsiyalar va huquqlarni tekshiring',
    close: 'Yopish',
    notConnected: 'Ulanmagan',
    noConnection: 'ulanish yoʻq',
    existing: 'Tuvaldagi qadamlar',
  },
  menu: {
    label: 'Amallar',
    open: 'Ochish',
    delete: 'Oʻchirish',
    deleteMany: (k) => `${k} ta qadamni oʻchirish`,
    duplicate: 'Nusxalash',
    copy: 'Nusxa olish',
    paste: 'Qoʻyish',
    connectTo: (outcome) => `${outcome} ni ulash…`,
    ignore: (outcome) => `${outcome} ni eʼtiborsiz qoldirish`,
    unignore: (outcome) => `${outcome} ni qaytarish`,
    snap: 'Toʻrga yopishtirish',
    addHere: 'Shu yerga qadam qoʻshish',
    selectAll: 'Hammasini belgilash',
  },
  toolbar: {
    dryRun: 'Sinov prog‘oni',
    testNode: 'Qadamni sinash',
    label: 'Muharrir paneli',
    edit: 'Muharrir',
    runs: 'Ishga tushirishlar',
    validate: 'Tekshirish',
    validating: 'Tekshirilmoqda…',
    undo: 'Bekor qilish',
    redo: 'Qaytarish',
    tidy: 'Tartiblash',
    add: 'Qoʻshish',
    settings: 'Sozlamalar',
    save: 'Saqlash',
    saving: 'Saqlanmoqda…',
    enabled: 'Yoqilgan',
    keys: 'Tugmalar',
    unsaved: 'Saqlanmagan oʻzgarishlar bor',
    versions: 'Versiyalar',
    version: (v) => `${v}-versiya`,
    newGraph: 'Yangi avtomatlashtirish',
  },
  panel: {
    label: 'Qadam sozlamalari',
    configure: 'Sozlash',
    data: 'Maʼlumotlar',
    close: 'Panelni yopish',
    output: 'Qadam natijasi',
    noOutput: 'Bu qadamda natija yoʻq.',
    copy: 'Nusxa olish',
    copied: 'Nusxa olindi',
    event: 'Hodisa',
    eventPlaceholder: 'Hodisani tanlang',
    action: 'Amal',
    actionPlaceholder: 'Amalni tanlang',
    actionMissing: (key) => `«${key}» amali mavjud emas: modul oʻchirilgan yoki ruxsat yoʻq boʻlishi mumkin.`,
    actionPick: 'Qadam bajaradigan amalni tanlang; uning maydonlari shu yerda chiqadi.',
    filter: 'Filtr',
    filterHint: 'Hodisa uchun CEL sharti; boʻsh — har bir hodisa',
    params: 'Parametrlar',
    schedule: 'Jadval',
    id: 'ID',
    unknownType: (type) => `«${type}» turidagi qadam mavjud emas: ulanish yoki huquq yoʻq boʻlishi mumkin.`,
    eventMissing: 'Bu hodisa endi mavjud emas.',
  },
  validation: {
    title: 'Tekshiruv',
    errors: (k) => `${k} ta xato`,
    warnings: (k) => `${k} ta ogohlantirish`,
    none: 'Muammo topilmadi',
    notChecked: 'Hali tekshirilmagan',
    checking: 'Tekshirilmoqda…',
    failed: 'Tekshirib boʻlmadi. Qoralama saqlandi.',
    retry: 'Qayta urinish',
    unavailable: (type) => `«${type}» turidagi qadam mavjud emas: ulanishni tekshiring`,
    noConnection: (title) => `«${title}»: ulanish mavjud emas. Uni «Integratsiyalar»da qayta ulang yoki boshqasini tanlang.`,
    close: 'Tekshiruvni yopish',
    graph: 'Avtomatlashtirish',
  },
  settings: {
    title: 'Avtomatlashtirish sozlamalari',
    name: 'Nomi',
    description: 'Tavsif',
    runAs: 'Kim nomidan bajariladi',
    owner: 'Egasi',
    ownerHint: 'Avtomatlashtirishni oxirgi yoqgan odam huquqlari bilan.',
    initiator: 'Tashabbuskor',
    initiatorHint: 'Hodisani keltirib chiqargan foydalanuvchi huquqlari bilan; u boʻlmasa — egasi nomidan.',
    done: 'Tayyor',
  },
  save: {
    pendingReview: 'Yangi versiya saqlandi, lekin yoqilmadi.',
    conflict: (by, v) => `Boshqa foydalanuvchi saqladi (${by}, ${v}-versiya).`,
    loadLatest: 'Oxirgisini yuklash',
    stay: 'Qolish',
    replaceTitle: 'Oxirgi versiyani yuklaysizmi?',
    replaceText: (v) => `Qoralama ${v}-versiya bilan almashtiriladi. Saqlanmagan oʻzgarishlar yoʻqoladi.`,
    replace: 'Qoralamani almashtirish',
    cancel: 'Bekor qilish',
    failed: 'Saqlab boʻlmadi. Qoralama saqlandi.',
    hasErrors: 'Saqlash uchun xatolarni tuzating.',
    offline: 'Aloqa yoʻq',
    retry: 'Qayta urinish',
    saved: (v) => `${v}-versiya sifatida saqlandi`,
    layoutSaving: 'Joylashuv saqlanmoqda…',
    layoutSaved: 'Joylashuv saqlandi',
    layoutFailed: 'Joylashuv saqlanmadi',
    reloadFailed: 'Oxirgi versiyani yuklab boʻlmadi.',
    leave: 'Saqlanmagan oʻzgarishlar bor. Baribir chiqasizmi?',
    stillWaiting: 'Server javobini kutyapmiz; hech narsa ikki marta yuborilmaydi.',
    saveOver: (v) => `v${v} ustidan saqlash`,
    saved_: 'Saqlandi',
  },
  states: {
    readOnly: 'Faqat koʻrish',
    mobile: 'Tahrirlash katta ekranda mavjud',
    emptyTitle: 'Trigger qoʻshing',
    emptyText: 'Avtomatlashtirish triggerdan boshlanadi: hodisa, jadval yoki qoʻlda ishga tushirish.',
    addTrigger: 'Trigger qoʻshish',
    loading: 'Muharrir yuklanmoqda',
  },
  keymap: {
    title: 'Tugmalar',
    gestures: 'Imo-ishoralar',
    keys: 'Tugmalar',
    close: 'Yopish',
    rows: [
      ['⌘K', 'Qadam yoki trigger qoʻshish'],
      ['← ↑ → ↓', 'Eng yaqin qadamga'],
      ['Enter', 'Qadam panelini ochish'],
      ['Esc', 'Yuqori qatlamni yopish'],
      ['Delete', 'Belgilanganni oʻchirish'],
      ['Shift+F10', 'Qadam menyusi: ulash, nusxalash, oʻchirish'],
      ['Alt+← ↑ → ↓', 'Belgilangan qadamni surish'],
      ['⌘C · ⌘V · ⌘D', 'Nusxa · qoʻyish · nusxalash'],
      ['⌘A', 'Hammasini belgilash'],
      ['⌘Z · ⇧⌘Z', 'Bekor qilish · qaytarish'],
      ['⌘S', 'Saqlash'],
      ['⌘Enter', 'Tekshirish'],
      ['⇧1 · ⇧0', 'Sigʻdirish · 100 %'],
      ['⇧⌘L', 'Joylashuvni tartiblash'],
      ['?', 'Ushbu roʻyxat'],
    ],
    gestureRows: [
      ['Qadamni torting', 'Surish'],
      ['Natijani torting', 'Ulash; boʻsh joyda qoʻying — qadam qoʻshish'],
      ['Uzoq bosish', 'Qadam yoki tuval menyusi'],
      ['Chimdish', 'Masshtab'],
      ['Tuvalni torting', 'Surish'],
      ['«Belgilash», soʻng torting', 'Bir nechta qadamni belgilash'],
    ],
  },
  outline: {
    label: 'Avtomatlashtirish tuzilmasi',
    triggers: 'Triggerlar',
    steps: 'Qadamlar',
    unreachable: 'Triggerdan yetib boʻlmaydi',
    empty: 'Avtomatlashtirishda hali trigger va qadamlar yoʻq.',
    goes: (outcome, target) => `${outcome} → ${target}`,
  },
  common: { close: 'Yopish', retry: 'Qayta urinish', undo: 'Bekor qilish' },
}

const uzCyrl: Strings = {
  canvas: {
    label: 'Автоматлаштириш графи',
    hintMouse: 'Улаш учун натижани қадамга тортинг · Қўшиш учун бўш жойга қўйинг · ⌘K — қўшиш · ? — тугмалар',
    hintTouch: 'Улаш учун натижани қадамга тортинг · Узоқ босиш — меню · Чимдиш — масштаб',
    hintReadOnly: 'Фақат кўриш: тувални суринг ва масштабланг',
    hintConnecting: 'Қадам устида қўйинг — улаш, бўш жойда — қадам қўшиш, бошқа жойда — бекор',
    rewire: (from, to, was) => `${from} → ${to} · эди → ${was}`,
    connect: (from, to) => `${from} → ${to}`,
    nowhere: 'ҳеч нарса',
    notConnected: 'уланмаган',
    ignored: 'эътиборсиз',
    unavailable: 'Мавжуд эмас',
    noConnection: 'Уланиш йўқ',
    zoomIn: 'Яқинлаштириш',
    zoomOut: 'Узоқлаштириш',
    fit: 'Сиғдириш',
    tidy: 'Тартиблаш',
    zoomReset: 'Масштаб 100 %',
    minimap: 'Мини харита',
    trigger: 'Триггер',
    error: 'Хатолар бор',
    states: { passed: 'Ўтди', running: 'Бажарилмоқда', waiting: 'Кутмоқда', failed: 'Хато', skipped: 'Ўтказиб юборилди', cancelled: 'Бекор қилинди' },
    selectMode: 'Белгилаш',
    add: 'Қўшиш',
    node: (title) => `Қадам ${title}`,
    outcome: (name, target) => `Натижа ${name}: ${target}`,
  },
  palette: {
    title: 'Қадам қўшиш',
    titleFor: (outcome) => `«${outcome}» дан кейинги қадам`,
    titleConnect: (outcome) => `«${outcome}» ни улаш…`,
    search: 'Қадам ва триггерларни қидириш',
    triggers: 'Триггерлар',
    steps: 'Қадамлар',
    count: (k, total) => `${total} дан ${k}`,
    nothing: (query) => `«${query}» бўйича ҳеч нарса топилмади`,
    noEvents: 'Мавжуд ҳодисалар йўқ: интеграциялар ва ҳуқуқларни текширинг',
    close: 'Ёпиш',
    notConnected: 'Уланмаган',
    noConnection: 'уланиш йўқ',
    existing: 'Тувалдаги қадамлар',
  },
  menu: {
    label: 'Амаллар',
    open: 'Очиш',
    delete: 'Ўчириш',
    deleteMany: (k) => `${k} та қадамни ўчириш`,
    duplicate: 'Нусхалаш',
    copy: 'Нусха олиш',
    paste: 'Қўйиш',
    connectTo: (outcome) => `${outcome} ни улаш…`,
    ignore: (outcome) => `${outcome} ни эътиборсиз қолдириш`,
    unignore: (outcome) => `${outcome} ни қайтариш`,
    snap: 'Тўрга ёпиштириш',
    addHere: 'Шу ерга қадам қўшиш',
    selectAll: 'Ҳаммасини белгилаш',
  },
  toolbar: {
    dryRun: 'Синов прогони',
    testNode: 'Қадамни синаш',
    label: 'Муҳаррир панели',
    edit: 'Муҳаррир',
    runs: 'Ишга туширишлар',
    validate: 'Текшириш',
    validating: 'Текширилмоқда…',
    undo: 'Бекор қилиш',
    redo: 'Қайтариш',
    tidy: 'Тартиблаш',
    add: 'Қўшиш',
    settings: 'Созламалар',
    save: 'Сақлаш',
    saving: 'Сақланмоқда…',
    enabled: 'Ёқилган',
    keys: 'Тугмалар',
    unsaved: 'Сақланмаган ўзгаришлар бор',
    versions: 'Версиялар',
    version: (v) => `${v}-версия`,
    newGraph: 'Янги автоматлаштириш',
  },
  panel: {
    label: 'Қадам созламалари',
    configure: 'Созлаш',
    data: 'Маълумотлар',
    close: 'Панелни ёпиш',
    output: 'Қадам натижаси',
    noOutput: 'Бу қадамда натижа йўқ.',
    copy: 'Нусха олиш',
    copied: 'Нусха олинди',
    event: 'Ҳодиса',
    eventPlaceholder: 'Ҳодисани танланг',
    action: 'Амал',
    actionPlaceholder: 'Амални танланг',
    actionMissing: (key) => `«${key}» амали мавжуд эмас: модул ўчирилган ёки рухсат йўқ бўлиши мумкин.`,
    actionPick: 'Қадам бажарадиган амални танланг; унинг майдонлари шу ерда чиқади.',
    filter: 'Филтр',
    filterHint: 'Ҳодиса учун CEL шарти; бўш — ҳар бир ҳодиса',
    params: 'Параметрлар',
    schedule: 'Жадвал',
    id: 'ID',
    unknownType: (type) => `«${type}» туридаги қадам мавжуд эмас: уланиш ёки ҳуқуқ йўқ бўлиши мумкин.`,
    eventMissing: 'Бу ҳодиса энди мавжуд эмас.',
  },
  validation: {
    title: 'Текширув',
    errors: (k) => `${k} та хато`,
    warnings: (k) => `${k} та огоҳлантириш`,
    none: 'Муаммо топилмади',
    notChecked: 'Ҳали текширилмаган',
    checking: 'Текширилмоқда…',
    failed: 'Текшириб бўлмади. Қоралама сақланди.',
    retry: 'Қайта уриниш',
    unavailable: (type) => `«${type}» туридаги қадам мавжуд эмас: уланишни текширинг`,
    noConnection: (title) => `«${title}»: уланиш мавжуд эмас. Уни «Интеграциялар»да қайта уланг ёки бошқасини танланг.`,
    close: 'Текширувни ёпиш',
    graph: 'Автоматлаштириш',
  },
  settings: {
    title: 'Автоматлаштириш созламалари',
    name: 'Номи',
    description: 'Тавсиф',
    runAs: 'Ким номидан бажарилади',
    owner: 'Эгаси',
    ownerHint: 'Автоматлаштиришни охирги ёқган одам ҳуқуқлари билан.',
    initiator: 'Ташаббускор',
    initiatorHint: 'Ҳодисани келтириб чиқарган фойдаланувчи ҳуқуқлари билан; у бўлмаса — эгаси номидан.',
    done: 'Тайёр',
  },
  save: {
    pendingReview: 'Янги версия сақланди, лекин ёқилмади.',
    conflict: (by, v) => `Бошқа фойдаланувчи сақлади (${by}, ${v}-версия).`,
    loadLatest: 'Охиргисини юклаш',
    stay: 'Қолиш',
    replaceTitle: 'Охирги версияни юклайсизми?',
    replaceText: (v) => `Қоралама ${v}-версия билан алмаштирилади. Сақланмаган ўзгаришлар йўқолади.`,
    replace: 'Қораламани алмаштириш',
    cancel: 'Бекор қилиш',
    failed: 'Сақлаб бўлмади. Қоралама сақланди.',
    hasErrors: 'Сақлаш учун хатоларни тузатинг.',
    offline: 'Алоқа йўқ',
    retry: 'Қайта уриниш',
    saved: (v) => `${v}-версия сифатида сақланди`,
    layoutSaving: 'Жойлашув сақланмоқда…',
    layoutSaved: 'Жойлашув сақланди',
    layoutFailed: 'Жойлашув сақланмади',
    reloadFailed: 'Охирги версияни юклаб бўлмади.',
    leave: 'Сақланмаган ўзгаришлар бор. Барибир чиқасизми?',
    stillWaiting: 'Сервер жавобини кутяпмиз; ҳеч нарса икки марта юборилмайди.',
    saveOver: (v) => `v${v} устидан сақлаш`,
    saved_: 'Сақланди',
  },
  states: {
    readOnly: 'Фақат кўриш',
    mobile: 'Таҳрирлаш катта экранда мавжуд',
    emptyTitle: 'Триггер қўшинг',
    emptyText: 'Автоматлаштириш триггердан бошланади: ҳодиса, жадвал ёки қўлда ишга тушириш.',
    addTrigger: 'Триггер қўшиш',
    loading: 'Муҳаррир юкланмоқда',
  },
  keymap: {
    title: 'Тугмалар',
    gestures: 'Имо-ишоралар',
    keys: 'Тугмалар',
    close: 'Ёпиш',
    rows: [
      ['⌘K', 'Қадам ёки триггер қўшиш'],
      ['← ↑ → ↓', 'Энг яқин қадамга'],
      ['Enter', 'Қадам панелини очиш'],
      ['Esc', 'Юқори қатламни ёпиш'],
      ['Delete', 'Белгиланганни ўчириш'],
      ['Shift+F10', 'Қадам менюси: улаш, нусхалаш, ўчириш'],
      ['Alt+← ↑ → ↓', 'Белгиланган қадамни суриш'],
      ['⌘C · ⌘V · ⌘D', 'Нусха · қўйиш · нусхалаш'],
      ['⌘A', 'Ҳаммасини белгилаш'],
      ['⌘Z · ⇧⌘Z', 'Бекор қилиш · қайтариш'],
      ['⌘S', 'Сақлаш'],
      ['⌘Enter', 'Текшириш'],
      ['⇧1 · ⇧0', 'Сиғдириш · 100 %'],
      ['⇧⌘L', 'Жойлашувни тартиблаш'],
      ['?', 'Ушбу рўйхат'],
    ],
    gestureRows: [
      ['Қадамни тортинг', 'Суриш'],
      ['Натижани тортинг', 'Улаш; бўш жойда қўйинг — қадам қўшиш'],
      ['Узоқ босиш', 'Қадам ёки тувал менюси'],
      ['Чимдиш', 'Масштаб'],
      ['Тувални тортинг', 'Суриш'],
      ['«Белгилаш», сўнг тортинг', 'Бир нечта қадамни белгилаш'],
    ],
  },
  outline: {
    label: 'Автоматлаштириш тузилмаси',
    triggers: 'Триггерлар',
    steps: 'Қадамлар',
    unreachable: 'Триггердан етиб бўлмайди',
    empty: 'Автоматлаштиришда ҳали триггер ва қадамлар йўқ.',
    goes: (outcome, target) => `${outcome} → ${target}`,
  },
  common: { close: 'Ёпиш', retry: 'Қайта уриниш', undo: 'Бекор қилиш' },
}

export const STRINGS: Record<EditorLocale, Strings> = { en, ru, uz, 'uz-Cyrl': uzCyrl }

export function strings(locale: EditorLocale | undefined): Strings {
  return pick(STRINGS, locale)
}

/** Key labels for the current OS: ⌘ on macOS, Ctrl elsewhere (K6). */
export function keyLabel(combo: string, mac: boolean): string {
  if (mac) return combo
  return combo.replace(/⇧⌘/g, 'Ctrl+Shift+').replace(/⌘/g, 'Ctrl+').replace(/⇧/g, 'Shift+')
}

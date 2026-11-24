# Automation Graphs — требования и спецификация (draft v0.1, 2026-11-20)

Статус: черновик для согласования. После согласования он превратится в ADR и задачи.
Прототип — движок автоматизаций Plot (`~/Projects/toys/plot`):
`packages/automation-graph`, `docs/AGENTS-AUTOMATIONS.md`, `adr/platform/automation-graph-package.md`.

---

## 0. Цели и не-цели

**Цели**
- Визуальный graph workflow engine внутри iota-sdk: Go-бэкенд, редактор на SolidJS.
- Расширяемость. Ноды, триггеры и коннекторы регистрируются модулями и приложениями (Granite, ERP) через реестры, без правок ядра.
- Коннекторы как настраиваемые интеграции. Если коннектор не настроен для тенанта, его ноды и триггеры не появляются в каталоге.
- Триггеры на доменные события SDK и приложений (пользователь создан, документ подписан, …) из единого типизированного каталога событий.
- CEL-выражения, Code-нода (JS/TS в песочнице), Jev-классификатор, LLM.
- Мультитенантность, RBAC, аудит, история запусков.
- Дальше: внедрение в Granite и iota ERP, замена routing и настроек уведомлений.

**Не-цели v1**
- Temporal или любая другая внешняя инфраструктура: только PostgreSQL.
- Двусторонняя синхронизация DSL и канваса. Источник истины — JSON графа.
- Произвольный недоверенный код на Go или Python.
- Маркетинговые кампании Granite: модуль campaign остаётся отдельным.

---

## 0.1 Результаты спайков (2026-11-20)

Отчёты и прототипы лежат в scratchpad сессии (`spikes/S1-runtime.md`, `S2-editor.md`, `S3-sandbox.md`, `S4-format-cel.md`, `R1-connectors.md`, `R2-events.md`).

| Спайк | Итог |
|---|---|
| **S1 Runtime** | **Гибрид H**: Go-реализованный workflow-контракт `automation.run` — это машина состояний поверх engine. Engine отвечает за leases, fencing, timers, signals, human tasks, terminate, resolve/retry и recovery. Состояние (runs, tokens, steps, waits) хранится в таблицах `automation.*` и пишется внутри fenced `wf.transaction` вместе с receipt. Journal — только журнал receipts. Внешние вызовы — дочерние instances, которые сигналят родителю. Прототип работает на реальном engine (Postgres 18). **A (replay) отвергнут**: при апгрейде интерпретатора эффекты повторяются (7× в симуляции), nondeterminism это не ловит. Replay стоит ~10 мс/МБ journal на каждое пробуждение. **B отвергнут**: claim, fence и heartbeat не экспортированы, а `human_tasks.instance_id` — FK на instances; B означал бы второй engine. |
| **S2 Editor** | **Своя Solid-обёртка над `@xyflow/system` 1.0.0 (pin exact)**. Граф 200 узлов / 500 рёбер: chunk 31.8 KB gz (~45 KB при паритете), mount 82 мс против 478 мс у `@dschz/solid-flow`, 60 fps. solid-flow 0.2.x заморожен на system 0.0.80, нельзя наложить вето на connect, а 1.x требует Solid 2. Риск: у `@xyflow/system` нет документированного API → все импорты в одном adapter-модуле + регрессионный бенчмарк при каждом bump. Оценка M: ~8–9 человеко-дней на канвас. |
| **S3 Sandbox** | **`fastschema/qjs` непригоден**: монтирует host FS read-write, `qjs:std/os` импортируются, нет cap памяти (рост до 4 GB), игнорирует `MaxExecutionTime`, fatal errors, по issue #49 — нераскрытые уязвимости. **Рекомендация — kafig (QuickJS-ng wasm на wazero) как собственный форк**: нет FS/env/std, page cap даёт чистый OOM, context deadline останавливает за 1.002 с, 226 µs на fresh instance, 18k runs/s, +4.6 MB бинаря. Нужно ~30 строк патчей (hang при deadline во время host call — обязательно; Date, Math.random, interrupt в async) и сборка wasm из исходников. goja не подходит: нет лимита памяти, требует Go 1.26. SSRF — `code.dny.dev/ssrf` + свой redirect policy + отказ, если любой DNS-ответ приватный. |
| **S4 Format/CEL** | `cel.dev/cel-go` v0.33.4 (модуль переехал с github.com/google). CEL-слой портирован: `expression-values` 8/8, 45/45 CEL-проверок из остальных фикстур, 125/139 probe-выражений совпадают с cel-js. `missing_context` — через unknowns + путь из AST, без regex. Нужен **allow-list функций** (cel-go шире cel-js) и отказ от `uint`/`bytes` в общем подмножестве. **Риск**: Plot принимает неизвестный behavior и падает → Plot сначала должен научиться отклонять неизвестные behaviors. Оценка: 10–15 дней на паритет + 6–10 дней на parallel/join/foreach. |
| **R1 Connectors** | **Didox**: публичный API (`api-docs.didox.uz`, test `testapi3.didox.uz`). Partner token выдаёт менеджер Didox. User token — через E-IMZO или пароль (сервер может логиниться сам; блокировки за неверный пароль). Чтение и черновики — без подписи. **Отправка, принятие, отклонение — только подпись пользователя E-IMZO в браузере** → human-нода «подписать в браузере». Webhooks нет → поллинг `/v2/documents`. **WhatsApp**: Meta Cloud API напрямую, креды тенанта, provider-интерфейс под будущих BSP. Окно 24 ч, шаблоны, кнопки: 3 reply / список 10 / шаблон quick-reply 10. HMAC `X-Hub-Signature-256`, ретраи до 7 дней → идемпотентный ingest. **Telegram**: `callback_data` ≤ 64 байта → токен на сервере. **Email**: OAuth для Gmail и M365 (M365 отключает basic SMTP в конце 12/2026), IMAP — fallback. |
| **R2 Events** | D7 осуществим, но **outbox и native set сейчас собираются только при `EXTRUNTIME_ENABLED` + Node dist**. Без runtime outbox растёт бесконечно (латентный баг, приложений на v0.47+ пока нет) → каталог событий и dispatcher выделяем в отдельный Go-only компонент. `core.user.created.v1` объявлен 3 раза (native, notification, history). Уведомление идёт через eventbus после commit, поэтому не атомарно. Миграция события ~40–80 LOC, tx доступна, нужен dual-publish в eventbus. Найдены баги: debt settle падает после commit; warehouse order events не публикуются. Подписка: **одна engine-wide на все графы с fan-out** `graph_version:event_id` и watermark при включении. Список из 28 событий для v1 готов. |

## 1. Принятые решения

Статус: ✅ подтверждено владельцем · 🟡 предложение, ждёт подтверждения.

**Отношение к Plot ✅ (пересмотр D2, 2026-11-20): вариант A «независимо, с заделом на C».** Конфликт D2 и D3 снят: формат не синхронизируется с Plot. Общий движок (C — Plot на Go-движке SDK) рассмотрим, когда Granite будет в проде на графах.

| # | Решение | Обоснование |
|---|---|---|
| D1 ✅ | **Runtime: substrate `pkg/extruntime/workflow`, вариант H по S1**: Go-контракт `automation.run` как машина состояний, состояние в `automation.*`, вызовы — дочерние instances. В `extension-runtime.md` §8/§9 оформляется поправкой «workflow-контракты могут иметь Go-реализации (как native commands §9.3)». Это Go-owned replay journal на Postgres: leases, timers, signals, human tasks, outbox. Temporal не используем. | В SDK уже есть durable substrate, а `docs/extension-runtime.md` запрещает второй engine. Temporal заставил бы каждого потребителя SDK (Granite, ERP, on-prem) поднимать свой кластер. |
| D2 ✅ | **Формат SDK собственный, словарь Plot.** SDK владеет форматом и развивает его свободно. Сознательно сохраняем словарь Plot: behaviors, outcomes как handles, язык полей, `nodes.<id>.output`, intent-before-call, versions вместо drafts, layout отдельно. Фикстуры Plot копируем **один раз** как стартовый набор тестов (`pkg/automation/graph/testdata/conformance`, с указанием commit-источника), без обязательства синхронизации. CEL-подмножество под cel-js не урезаем, нужен лишь allow-list функций для стабильности. | Графы Plot и Granite оперируют непересекающимися доменами, так что переносимость почти ничего не даёт, а координация двух репозиториев тормозит обе стороны. Похожесть сидит в коннекторах и нодах, а их общий формат не делает общими. Сохранённый словарь делает будущее сближение (C) дешёвым. |
| D3 ✅ | **Семантика v1 — полный набор:** последовательное выполнение, branch, switch, wait, **parallel fan-out, join (all/any/n-of), foreach**. | Нужно ERP: рассылки по спискам, параллельное согласование, сбор ответов. |
| D4 ✅ | **Выражения — CEL (`google/cel-go`)**, строгая проверка типов при сохранении. | Паритет с Plot (cel-js), безопасность (нет побочных эффектов, ограниченная стоимость), стандарт де-факто. |
| D5 ✅ | **Code-нода — JS/TS в QuickJS-ng на wazero через собственный форк kafig** (iota-uz): сборка wasm из исходников, патчи по S3, собственный набор тестов на изоляцию внутри процесса Go. TS транспилируется esbuild (Go API). | Недоверенный код админов тенанта. Не нужен CGO, не нужны Node-процессы, есть лимиты памяти и CPU, по умолчанию нет FS и сети. |
| D6 ✅ (S2) | **Редактор — Solid.** Базовая гипотеза: тонкий Solid-биндинг над `@xyflow/system` (framework-agnostic ядро React и Svelte Flow). Альтернатива: `@dschz/solid-flow`. Выбор фиксирует спайк. | В SDK Solid 1.9. Ветка `@dschz/solid-flow` 1.x нацелена на Solid 2.0 и находится в prerelease, 0.2.x — alpha. Рискованная зависимость. |
| D7 🟡 | **Каталог событий один — `native.Event`** (типизированный, transactional outbox). Триггеры, каталог уведомлений и подписки читают его. | Сейчас три несвязанных механизма: eventbus, native events и business history. |

---

## 2. Архитектура и границы

```
pkg/automation/
  graph/        формат, язык полей, catalog types, валидатор, чистый интерпретатор (conformance)
  expr/         CEL env из catalog, typecheck, шаблоны {{ }}
  engine/       durable исполнитель (Runner для extruntime), dispatch событий, waits, calls
  catalog/      реестры: NodeType, TriggerSource, Connector; сборка каталога для тенанта
  connectors/   интерфейс Connector, хранилище connections (шифрованные креды), auth flows
  code/         песочница QuickJS/wazero, host fetch (SSRF guard), секреты, маскирование
  builtin/      встроенные ноды (logic, notify, sdk actions, http)
modules/automations/   UI (редактор, runs, connections), контроллеры, RBAC, локали, settings hub
pkg/automation/connectors/{telegram,whatsapp,email,eskiz,openai,jev,didox,...}
web/automation-editor  → @iota-uz/sdk/automation-editor (bounded subpath, lazy chunk)
```

Принципы:
- `graph` и `expr` чистые: без БД, без IO, без зависимостей SDK. Их тестируют conformance-фикстуры.
- Движок зависит только от интерфейсов (`Catalog`, `ConnectionStore`, `CallExecutor`, `SubjectLoader`), а не от модулей.
- Модули и приложения вносят события, ноды и коннекторы через `composition.Contribute*` по образцу `notifications.Catalog` / `native.Set`: дубликаты ключей падают при композиции.

### Публичные интерфейсы (сигнатуры, без реализации)

```go
// pkg/automation/catalog
type NodeType interface {
    Spec() graph.NodeSpec                     // behavior, params (field lang), outcomes rule, expressions, output schema
    Requires() []ConnectorKind                // пусто → всегда доступна
}
type CallNode interface {                     // behavior=call: внешний эффект вне lock
    NodeType
    Execute(ctx context.Context, in CallInput) (CallResult, error) // CallResult{Outcome, Output, RetryAfter}
    Idempotent() bool                         // true → разрешён авто-retry
}
type TriggerSource interface {
    Events() []graph.EventSpec                // ключ, payload schema, subject, trigger params
    Requires() []ConnectorKind
}
type Registry interface {
    RegisterNode(NodeType) error
    RegisterTriggers(TriggerSource) error
    RegisterConnector(Connector) error
    ForTenant(ctx context.Context, tenantID uuid.UUID) (graph.Catalog, error) // фильтр по настроенным connections + RBAC
}

// pkg/automation/connectors
type Connector interface {
    Kind() ConnectorKind                      // "telegram_bot", "whatsapp_cloud", "smtp", "openai", "jev", "didox"…
    Descriptor() Descriptor                   // имя, иконка, credential schema, auth flow (api_key|oauth2|basic|custom)
    Test(ctx context.Context, c Credentials) (Identity, error)
    Nodes() []NodeType
    Triggers() []TriggerSource                // inbound (telegram.message_received…)
}
type ConnectionStore interface {
    List(ctx context.Context, tenantID uuid.UUID, kind ConnectorKind) ([]Connection, error)
    Get(ctx context.Context, tenantID, id uuid.UUID) (Connection, Credentials, error) // креды только в engine
    Save(ctx context.Context, c Connection, creds Credentials) error
    SetStatus(ctx context.Context, id uuid.UUID, s ConnectionStatus, reason string) error
}
```

---

## 3. Модель графа (общая с Plot + расширения)

База — `AutomationGraph` из Plot:
- `triggers[]`, `nodes[]`, `edges{from:{node,outcome}, to}`;
- outcomes — это выходные handles;
- id — CEL-идентификаторы (`^[a-z][a-z0-9_]{0,39}$`);
- layout хранится отдельно от версии;
- каждое сохранение — immutable version, run закреплён за версией;
- optimistic concurrency (`VERSION_CONFLICT`);
- граф, сохранённый агентом, создаётся выключенным, включает его только человек.

Расширения SDK (свои фикстуры в том же формате, что у Plot):
- **Тенантность.** `tenant_id` у automations, versions, runs, connections; уникальные индексы считаются в рамках тенанта.
- **Subjects** обобщаются: `subject = {entity, key}` — любая сущность SDK или приложения (user, client, document, policy…) из `SubjectLoader`. Есть также `conversation` для коннекторов и `none`.
- **Новые behaviors:**
  - `parallel` — все outcomes запускаются одновременно, на каждую ветку создаётся отдельный token;
  - `join` — режимы `all | any | n_of`, есть `timeout`; outcomes `joined`, `timeout`; выход — карта выходов веток;
  - `foreach` — `items` (CEL list), `concurrency` (≤ лимита), тело — отдельный подграф (sub-automation) или ветка до `collect`; outcomes `done`, `partial_failed`; выход — список результатов.
- **Валидация** добавляет к правилам Plot:
  - каждый parallel закрывается join или концом ветки;
  - join достижим только из веток одного parallel;
  - тело foreach не выходит за его границы;
  - цикл проходит через `wait` **или** через call-ноду с ожиданием (в Plot это несоответствие нужно исправить в обоих местах).

---

## 4. Семантика исполнения

- **Run.** `running | waiting | done | failed | cancelled`. Внутри run активны несколько **tokens** (по одному на ветку parallel). Каждый token хранит текущую ноду.
- **Интеграция с extruntime — решена в S1: вариант H** (см. §0.1). Ниже исходный анализ вариантов. Изменения substrate по S1:
  - native-реализации workflow по контракту вместо одного `Attach`;
  - lazy-загрузка journal;
  - cancel отдельного wait и отмена открытых waits/tasks при завершении run;
  - terminate каскадом на дочерние instances;
  - human tasks на role/group + quorum (на уровне task);
  - tenant в `ScheduleDef`;
  - режим подписки, где run-as выбирает handler;
  - `invocation_id` в `host.Request`;
  - attempt в `wf.step.begin`.

  Порядок lock: subject → instance. Не проверено: окна крэша, `wf.call` из Go, prod resolvers (это первые задачи P2).

Исходный анализ: Спайк S1 выбирает между двумя вариантами. Задачи P2 стартуют только после S1.
  - **(A) Replaying Go Runner.** Граф-run — это `workflow_instance`. Go-реализация `workflow.Runner` переигрывает интерпретатор, occurrence = `(instance, "<node>#<token>", n)`. Используются `wf.waitFor`, `wf.sleep`, `wf.task`, `wf.call`.
  - **(B) State-machine executor.** Свои таблицы `runs`, `tokens`, `steps`, а общие с extruntime — outbox, dispatcher, receipts, timers, human_tasks, secrets и lease-механика. Это ближе к Plot: шаг — это транзакция, и journal не переигрывается.
- **Пробелы substrate, которые S1 обязан закрыть или измерить:**
  1. **Pinning.** Instance закреплён за `package/version/artifact_hash` через `Implementations`, activation общая на всё приложение. Графы же живут по тенантам, и каждое сохранение даёт новую версию.
  2. **Один Runner.** Engine принимает один `Runner` (`Attach`): нужен dispatcher по типу instance или изменение engine.
  3. **Стоимость replay.** Каждое пробуждение переигрывает весь journal (`RunTimeout` 5m). Циклы через wait и foreach ≤ 1000 приведут к росту journal — нужен замер. Это главный аргумент за (B).
  4. **Новые примитивы:**
     - lock по subject;
     - wait на событие с CEL-фильтром (signals адресуются конкретному instance, а нужен match по subject и kind);
     - human task на роль или группу и quorum (`wf.task` умеет только user или permission).
  5. **Недетерминизм.** `now` и `uuid` в CEL и code должны быть journaled.
  6. **Schedules.** `ScheduleDef` привязан к package и artifact. Для графов нужна своя `ScheduleSource` по тенантам или расширение `ScheduleDef`.
- **Внешние вызовы (правило Plot, ✅ одно правило).**
  - Вызов никогда не выполняется внутри locked-шага. Порядок: записать intent `started` → вызвать → применить результат отдельной транзакцией.
  - Если intent есть, а результата нет: для `Idempotent()` нод — повтор с тем же idempotency key. Для остальных — **всегда `outcome_unknown` → outcome `failed`**, если он подключён, иначе run `blocked` до ручного resolve/retry.
  - Семантика `wf.step` «перезапустить started» для неидемпотентных нод не используется.
  - Авто-retry ещё и при 429 / `retry_after` в пределах deadline ноды.
- **Ошибки.** Если у ноды есть outcome `failed`, ошибка идёт туда. Если нет — run `failed`, плюс уведомление владельцу графа и запись в «Требует внимания».
- **Параллелизм.** Шаги одного subject сериализуются (lock по subject). Разные subjects выполняются параллельно. Conversation-subject ставит новые старты в очередь.
- **Лимиты (настраиваемые):**
  - глубина каскада automation → event → automation ≤ 10;
  - foreach ≤ 1000 элементов, concurrency ≤ 20;
  - CEL cost limit;
  - wait ≤ 30 дней;
  - payload и output ноды ≤ 1 MB.
- **Отмена.** Выключение или удаление графа отменяет waiting runs, running шаг доходит до конца.
- **Dry-run и test node.** Чистый интерпретатор на заданной последовательности событий и контексте. Test node выполняет один реальный вызов без записи run.
- **Версии.** Run закреплён за своей версией. Live runs не мигрируют на новую версию; есть явная операция «перезапустить на новой версии» (`StartV2`).

---

## 5. Триггеры и каталог событий SDK

**Источники триггеров:**
1. **Доменные события** из `native.Event` + outbox, в одной транзакции с изменением.
2. **Schedule** — cron/recurrence с часовым поясом и `missed_after`, через `extruntime/events/schedule`.
3. **Manual** — кнопка «Запустить» с формой входных параметров.
4. **Inbound коннекторов** — `telegram.message_received`, `whatsapp.message_received`, `email.received`, …
5. **Webhook (inbound)** — отложен, в v1 не входит (Q3).
6. **Записи сущностей** — `record.created/updated` на основе business-history, как fallback для сущностей без типизированных событий (опционально, Q4).

**Работа по каталогу:**
- Сейчас нативных событий 3: `core.user.created.v1`, `crm.client.created.v1`, `crm.client.updated.v1`.
- Около 60 eventbus-событий в SDK переводим на `native.Event` по приоритетам:
  - core: user, role, group, membership;
  - crm: client, chat message;
  - finance: payment, debt settled / written off, expense;
  - hrm: employee;
  - warehouse: order created / completed;
  - billing: status changed.
- Каждое событие получает:
  - версионированный ключ;
  - типизированный payload;
  - локализованные label и description;
  - subject.
- Публикация события — в транзакции сервиса.
- Каталог уведомлений (`notifications.Definition`) ссылается на `native.Event` вместо отдельного объявления, и ключи совпадают.
- «Документ подписан» и прочие события документооборота объявляет приложение (Granite EDO: `edo.document.signed.v1`, `approved`, `rejected`, `resolution.assigned`/`overdue`). SDK даёт только механизм.

---

## 6. Каталог нод v1

| Группа | Ноды | Требует коннектор |
|---|---|---|
| Logic | branch, switch, parallel, join, foreach, wait (event/timeout), delay, end | — |
| Data | set (CEL map → output), code (JS/TS) | — |
| Human | human task: назначение user / role / permission, форма решения → outcomes по кнопкам, quorum, timeout | — |
| SDK | notify: recipients — users / groups / roles / event participants, каналы, уровень. Внимание: `EnqueueAddressedTx` обходит rules и не разрешает аудитории; разрешение групп и ролей и повторная проверка доступа к subject сейчас живут в `NotificationRoutingService.Publish`. Нужен общий API «resolve audience + subject access + enqueue» (задача P3). Также send to record contact | — |
| SDK actions | вызов зарегистрированной команды или сервиса (`wf.call`): создать задачу, сменить статус и т. п. Регистрируются модулями. | — |
| Flow | run automation (дочерний граф) | — |
| MCP | mcp_tool (вызов инструмента MCP-сервера, схема закреплена в версии) | mcp |
| HTTP | http request (SSRF guard, секреты, таймаут) | — (опц. credential) |
| AI | jev (классификация → outcomes), llm generate (текст / JSON по схеме) | jev / openai |
| Telegram | send, ask (кнопки → outcomes, ожидание ответа), read | telegram_bot |
| WhatsApp | send template, send message (в окне 24 ч), ask | whatsapp_cloud |
| Email | send (SMTP), wait reply / inbound (IMAP), label | smtp / imap |
| SMS | send (Eskiz, Twilio) | eskiz / twilio |
| Didox | создать / отправить документ, проверить статус; триггеры: документ подписан / отклонён | didox |

Правило доступности:
- `Registry.ForTenant` убирает ноды и триггеры, у которых нет `connected` connection нужного kind.
- Если граф ссылается на ставший недоступным connector: сохранение даёт `VALIDATION_FAILED (connection_unavailable)`, существующий run падает с `connection_unavailable`, в редакторе нода помечается.
- Credential может быть уровня платформы (env/config, по умолчанию для всех тенантов) или уровня тенанта (переопределяет платформенный).

---

## 7. Коннекторы и креды

- **Таблица `automation.connections`:** tenant_id, kind, name (slug), identity, status (`connected | needs_reauth | error | disabled`), зашифрованный secret, inbound state.
- **Шифрование:** AES-256-GCM, master key из config, AAD = `tenant_id|id|kind`. Ротация ключа через key id. Секрет никогда не уходит в API, UI или логи.
- **Auth flows:** api_key / basic, OAuth2 (PKCE, refresh за 15 минут до истечения под row lock), custom (Telegram webhook secret).
- **UI:** «Интеграции» в settings hub — список коннекторов, подключить, протестировать, статус, где используется. Удаление запрещено, пока connection используется (`CONNECTION_IN_USE`).
- **Мост к extruntime:** то же хранилище реализует `host.SecretResolver`, этого пока не хватает в SDK.
- **Существующие интеграции** (Telegram bot, Eskiz, Twilio, SMTP, OpenAI) оборачиваются в коннекторы. Env-config остаётся как платформенный credential.

---

## 8. Выражения (CEL)

- **Где используются:** trigger filter, branch / switch, wait filter, join condition, foreach items, аргументы нод (`{cel}` | `{template}` | `{value}`), шаблоны `{{ }}`.
- **Scope:**
  - `event` — payload триггера;
  - `nodes.<id>.output` — выходы предыдущих нод;
  - `subject` — загруженная сущность;
  - `tenant`, `now`;
  - `secrets` в CEL **недоступны**.
- **Проверка типов при сохранении** по catalog (field language → CEL decls). Неизвестное поле — ошибка, а не пустая строка. Ограничения: cost limit, глубина AST.
- **Editor API** отдаёт `validate(expr, scope)` и `complete(prefix, scope)`. Для полей выражений — CodeMirror или Monaco: Monaco уже есть в configurator.

---

## 9. Code-нода

- **Контракт:** `export default async function run(ctx, env): Promise<{outcome, output?}>`. `ctx` — типизированный контекст (сгенерированный `.d.ts` в редакторе). `env.fetch` и `env.secrets` — только объявленные в ноде.
- **Песочница:** QuickJS на wazero, свежий экземпляр на вызов (или пул с reset). Лимиты: память 128 MB, timeout (по умолчанию 30 с, максимум 5 мин), размер логов, размер ответа fetch 10 MB.
- **Fetch:** только https. DNS-проверка на каждом hop: блок private / CGNAT / loopback / link-local / metadata. Pin IP. Снятие `authorization` при cross-origin redirect.
- **Маскирование секретов** в логах, ошибках и output, включая JSON- и URL-escaped варианты.
- **Включение:** перед включением графа человек видит код и секреты каждой Code-ноды.
- **Проверка типов TS.** esbuild только вырезает типы. Plot проверяет TS при сохранении (tsc против сгенерированного `.d.ts`). Здесь (🟡): typecheck в редакторе (TS worker в Monaco), а на сервере — только транспиляция и синтаксис. Серверный tsc потребовал бы Node, это противоречит D5. Ошибки типов в редакторе блокируют сохранение мягко (warning), синтаксические — жёстко.

---

## 10. Редактор (SolidJS)

Паритет с `plot/packages/automation-editor`:
- канвас: сетка, snap, drag outcome → connect, drop на пустое место → палитра с фильтром;
- ⌘K палитра;
- панель ноды: Configure / Data, формы по JSON Schema из catalog, `fieldSlot` для переопределений;
- prompt drawer с автодополнением;
- панели Validation, Dry run, Test node;
- Run view overlay (путь, токены, input / output / logs нод);
- вкладка Runs;
- undo/redo, copy/paste, auto-layout (elkjs/dagre);
- read-only outline на мобильных;
- клавиатура, a11y.

Требования к пакету:
- Пакет без продуктового кода: получает `catalog` + `EditorApi` (`validate`, `dryRun`, `testNode`, `searchSubjects`, `save`, `runs`).
- Bounded subpath `@iota-uz/sdk/automation-editor`, lazy-chunk. Release check проверяет, что базовый host его не тянет.
- Монтаж через `solid.Import` + `solid.Query/Action` по образцу `pkg/extruntime/configurator`.
- Локали en / ru / uz / uz-Cyrl.

---

## 11. Мультитенантность, RBAC, агенты, наблюдаемость

- **Permissions:** `Automations.Read`, `Automations.Author`, `Automations.Enable`, `Automations.Operate` (cancel, retry, resolve), `Connections.Manage`.
- **Run as:** инициатор события, иначе owner-user графа (`trigger_owners`). Права проверяются на каждом шаге.
- **MCP / агенты:** могут авторить графы (по образцу `extruntime/mcptools/workflows.go`). Включение графа, ответ в human-ноде и управление connections — только человек (`HUMAN_REQUIRED`).
- **Наблюдаемость:**
  - журнал шагов (input, outcome, output, error, duration);
  - OTel span на run и на шаг;
  - метрики: runs started / failed, длительность шага, ошибки коннекторов, лаг очереди;
  - структурированные логи;
  - retention: события и step logs 30 / 90 дней, настраивается.

---

## 12. Замена системы уведомлений (фаза 2)

- **Notification rule = preset-граф:** `trigger(event) → [filter] → notify(recipients, channels, level)`. Текущая страница `/settings/notifications` становится «простым режимом» над этими графами. Для сложной логики есть «Открыть в редакторе».
- **Миграция:** каждый `core.notification_rules` превращается в граф-пресет. Dispatch, deliveries и channel preferences остаются как есть: граф использует существующий delivery-слой, а не заменяет его.
- **Новые каналы** (Telegram, WhatsApp) — `ChannelAdapter` поверх коннекторов. Ими пользуются и нода notify, и персональные предпочтения.
- **Granite:**
  - захардкоженные Telegram-алерты CRM (`notification_handler.go`) переносятся в графы;
  - разовые SMS при SOS — тоже в графы;
  - события EDO становятся `native.Event`;
  - согласование и подписание EDO — кандидат на фазу 3. Начинаем с уведомлений и эскалации просрочек, движок статусов пока не заменяем.
- **Предусловие:** Granite обновлён с SDK v0.35, а `iota-uz/erp` с SDK v0.28 до актуальной версии (каналы, preferences, recipient directory, native events).

---

## 13. Фазы

| Фаза | Содержание | Критерий готовности |
|---|---|---|
| P0 Спайки | S1 Go-runner на extruntime engine; S2 Solid-биндинг xyflow; S3 QuickJS/wazero (лимиты, fetch, производительность); S4 портирование conformance-фикстур и CEL-паритет cel-go ↔ cel-js | Решения D1, D5, D6 подтверждены или пересмотрены |
| P1 Ядро | `graph`, `expr`, валидатор и интерпретатор (вместе с parallel/join/foreach); набор фикстур зелёный | Скопированные фикстуры Plot (кроме сознательно изменённых, с пометкой) + фикстуры SDK проходят |
| P2 Движок | Runner, dispatch событий, waits / timeouts / human, calls с intent, лимиты, отмена, версии, таблицы и миграции | E2E: событие → run → wait → timeout → end; crash recovery |
| P3 Каталог и коннекторы | Реестры, ConnectionStore + шифрование, UI «Интеграции», встроенные ноды, notify, http, code | Нода скрыта без connection; секрет не утекает |
| P4 Редактор | Solid-редактор, runs view, dry run, test node | Паритет функций с Plot |
| P5 Интеграции | Telegram, Email, Eskiz, OpenAI / LLM, Jev, WhatsApp, Didox | Каждый коннектор: test, нода, inbound-триггер (где применимо) |
| P6 События SDK | Перевод приоритетных eventbus-событий на `native.Event`, связь с каталогом уведомлений | Триггеры доступны в палитре |
| P7 Уведомления | Rules → presets, Telegram / WhatsApp channel adapters | Паритет с текущими rules, миграция данных |
| P8 Внедрение | Granite (апгрейд SDK, EDO-события, Telegram-алерты), iota ERP | Продовые графы заменяют хардкод |
| P9 Согласования EDO | Маршруты согласования и подписания EDO как графы: human-ноды (quorum, делегирование, эскалация просрочек), события подписи | Маршруты EDO в Granite работают на графах |

---

## 14. Решения по открытым вопросам (все закрыты)

- **Q0 «Routing» ✅:** заменяем оба. Routing уведомлений — в фазе 7, маршруты согласования EDO — в фазе 9 (после P8). Human-нода сразу проектируется с quorum, делегированием и эскалацией.
- **Q3 ✅:** входящий webhook-триггер в v1 не делаем, откладываем.
- **Q1 Didox ✅:** коннектор в SDK, полный. Ноды: создать/обновить черновик, получить документ/статус, PDF. Human-нода «подписать в браузере» (E-IMZO): отправить, принять, отклонить. Триггеры через поллинг `/v2/documents` с дедупом `id+status+updated`: получен, подписан контрагентом, отклонён, отменён. Partner token получает владелец у менеджера Didox — это внешняя зависимость задачи. Для E-IMZO API-KEY нужен общий домен подписи.
- **Q2 WhatsApp ✅:** Meta Cloud API напрямую, креды тенанта, provider-интерфейс под BSP позже.
- **MCP-нода ✅ (v1):** коннектор `mcp` (Bearer / OAuth 2.1), нода `mcp_tool` со схемой, закреплённой в версии. Это общий слой коннекторов с Plot (задел на C).
- **S1-детали ✅:** quorum на уровне human-task (n-of-N); внешние вызовы — дочерние instances engine.
- **Run-as ✅:** выбирается на уровне графа: инициатор события или владелец графа. По умолчанию — владелец.
- **События v1 ✅:** 28 событий объявляются вручную как `native.Event` с dual-publish в eventbus (список — в R2). Каталог и dispatcher выносятся в Go-only компонент.
- **Q4 ✅ → v2.** Триггер на изменение записи через business-history — в v2.
- **Q5/Q6 ✅:** формат и фикстуры живут в SDK, с Plot не синхронизируются (D2). Точка пересмотра — Granite в проде на графах.
- **Q7 ✅:** платформенный AI-ключ доступен тенанту только после явного включения админом. Учёт расхода по тенанту, ключ тенанта переопределяет платформенный.
- **Q8 ✅:** LLM-нода в v1 — генерация текста и JSON по схеме, через коннектор OpenAI.

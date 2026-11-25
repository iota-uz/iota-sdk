# Модуль `automations`: страница «Интеграции»

`modules/automations` — модуль автоматизаций SDK. Он даёт права `Automations.*` и
`Connections.Manage`, раздел в settings hub и первую страницу — «Интеграции»
(`/settings/integrations`): администратор тенанта подключает аккаунты к
коннекторам приложения, проверяет их, видит статус и где подключение используется.

Коннектор почты (SMTP, IMAP, вход через Google и Microsoft 365) описан в
[automation-email.md](automation-email.md).

## Подключение в приложении

Модуль входит в `modules.Components()` без опций. Приложение, которое регистрирует
свои коннекторы (или usage index, или манифест бандла), заменяет эту запись своей —
два компонента `automations` в одном приложении дают ошибку композиции:

```go
components := slices.DeleteFunc(modules.Components(), func(c composition.Component) bool {
    return c.Descriptor().Name == "automations"
})
components = append(components, automations.NewComponent(
    automations.WithConnectors(telegram.New(), smtp.New()),
    // automations.WithUsageIndex(engine.Usage) — подключает #1339
))
```

Без `WithUsageIndex` модуль берёт `connectors.UsageIndex` из контейнера, если его
кто-то предоставил; иначе «Используется в» пусто, и удаление не блокируется.

Модуль предоставляет в контейнер (по умолчанию, приложение может заменить):
`connectors.Directory`, `catalog.Registry` (доступность — хранилище подключений),
`connectors.Store` (`ConnectionStore`, `CredentialSource`), `connectors.OAuth2`,
`connectors.PlatformCredentials`, `connectors.PlatformGrants` и
`host.SecretResolver`. Extension runtime использует этот `SecretResolver`, если в
`wiring.Options.Secrets` ничего не задано: секрет пакета `<connection name>.<field>`
берётся из подключения тенанта (только поле с `x-secret`, только подключённое, и только
если пакет объявил этот секрет в манифесте). **Это меняет поведение по умолчанию:**
раньше без `Options.Secrets` вызов `secrets.get` был недоступен. Приложение, которое
не хочет отдавать секреты подключений пакетам, задаёт `wiring.Options.Secrets` явно
(в том числе резолвер, который всегда отвечает «не найдено»).

## Конфигурация

| Переменная | Что задаёт |
| --- | --- |
| `AUTOMATION_CONNECTIONS_KEYS` | Ключи шифрования `id:key[,id:key…]` (32 байта, base64 или hex). Без них подключения не сохраняются, страница показывает предупреждение. |
| `AUTOMATION_CONNECTIONS_ACTIVE` | Активный ключ при ротации. |
| `AUTOMATION_PLATFORM_CREDENTIALS` | JSON `{"<kind или имя OAuth-клиента>": {"<поле>": "<значение>"}}`: ключи платформы и `client_id`/`client_secret` OAuth2-клиентов. |
| `AUTOMATION_OPENAI_APIKEY` | Платформенный ключ OpenAI для ноды `ai.llm`: ключ задан — коннектор `openai` предлагается тенантам (только после включения администратором тенанта). Это не ключ BiChat (`BICHAT_OPENAI_*`): расход автоматизаций учитывается отдельно. |
| `AUTOMATION_OPENAI_BASEURL` | OpenAI-совместимый endpoint платформенного ключа (только https); по умолчанию `https://api.openai.com/v1`. |
| `AUTOMATION_OPENAI_DEFAULTMODEL` | Точный id модели, который записывается в версию графа, если в ноде `ai.llm` модель не указана (по умолчанию снапшот `gpt-4o-mini-2024-07-18`). |
| `AUTOMATION_JEV_APIKEY` | Платформенный ключ классификатора `ai.jev`. |
| `AUTOMATION_JEV_PROVIDER` | Чей это ключ: `openrouter` (по умолчанию) или `openai`. |
| `AUTOMATION_JEV_DEFAULTMODEL` | Точный id модели по умолчанию для `ai.jev`; по умолчанию `typesafe/jev-1.14` для `openrouter` и `gpt-4o-mini-2024-07-18` для `openai`. |
| `ESKIZ_EMAIL`, `ESKIZ_PASSWORD`, `ESKIZ_URL`, `ESKIZ_FROM` | Платформенный аккаунт Eskiz (`url` по умолчанию `https://notify.eskiz.uz`, `from` — sender id). Включается наличием `ESKIZ_EMAIL` и `ESKIZ_PASSWORD`. |
| `TWILIO_ACCOUNTSID`, `TWILIO_AUTHTOKEN`, `TWILIO_PHONENUMBER` | Платформенный аккаунт Twilio для автоматизаций; включается наличием всех трёх. Запись `eskiz`/`twilio` в `AUTOMATION_PLATFORM_CREDENTIALS` приоритетнее. |
| `AUTOMATIONS_ASSETS_DIR` | Разработка: каталог свежесобранного бандла экрана вместо встроенного. |

Платформенные аккаунты SMS платные, поэтому у коннекторов `eskiz` и `twilio` политика `opt_in`:
тенант получает их только после включения администратором (`PlatformGrants`).

Адрес возврата OAuth2 — `<origin приложения>/settings/integrations/oauth/callback`;
его нужно зарегистрировать у провайдера.

## Клиентский бандл

Экран работает без сборки на стороне приложения: модуль встраивает собранный бандл
(`modules/automations/presentation/integrations/dist`, закоммичен) и отдаёт его сам на
`/assets/automations/`. Достаточно строки в `modules.Components()` — менеджер с
`Connections.Manage` видит пункт «Интеграции» в settings hub и открывает страницу.

Приложение, которое уже собирает свой клиентский бандл, может вместо этого включить
фичу в свой каталог и передать манифест через `automations.WithHostManifest`:

```ts
import { solidFeatures as integrations } from "<iota-sdk>/modules/automations/presentation/integrations/web/solid-features.generated"
mountSolidFeatureCatalogFromDocument({ catalog: { ...app, ...integrations } })
```

После изменения экрана бандл пересобирают и коммитят; `pnpm check:bundle` падает, если
закоммиченный бандл устарел:

```sh
cd modules/automations/presentation/integrations/web
pnpm build:bundle   # dist/ и dist/bundle.json (идентичность бандла — хэш его содержимого)
pnpm check:bundle   # собирает во временный каталог и сравнивает, dist не трогает
```

## RPC

Методы объявлены в `modules/automations/presentation/integrations/controller.go`
(`solid.Import(…, solid.WithRPC(…))`), контракт TypeScript генерирует `iota-solid`:

```sh
go run ./cmd/iota-solid -module modules/automations/presentation/integrations -catalog web/solid-features.generated.ts
```

Сервер — `solid.RPCHandler` (`pkg/clienthost/solid/rpc.go`): JSON-RPC 2.0, который
шлёт `FetchRPCTransport`, на `POST /settings/integrations/rpc`. Тенант и пользователь —
из сессии. Ошибки с кодом `forbidden` (нет права), `not_found` (чужой или удалённый
`id`), `unavailable` (нет ключей или OAuth2-клиента); прочие отказы — в результате
(`test_failed`, `name_taken`, `invalid_input`, `version_conflict`, `CONNECTION_IN_USE`).
Значения полей с `x-secret` не возвращаются ни в одном ответе; сообщение коннектора
при неудачной проверке маскирует все введённые значения.

## Проверки

```sh
go test ./modules/automations/... ./pkg/clienthost/solid/
cd modules/automations/presentation/integrations/web
pnpm typecheck && pnpm test          # модель экрана
pnpm test:browser                    # Playwright: сценарии и VR+axe всех состояний (1440/390, светлая/тёмная)
pnpm test:browser --update-snapshots # обновить Linux-baseline (browser-tests/__screenshots__)
```

Fixture `fixtures/integrations` открывается так же (сначала стили fixture — `fixture.generated.css` не
коммитится): `node ./fixtures/integrations/build-css.mjs && pnpm vite fixtures/integrations`
с параметрами `?scenario=empty|error|loading|fresh`, `?lang=ru`, `?theme=dark`,
`?delay=<мс>`, `?testDelay=<мс>`, `?testFailOnce`, `?lossy`, `?conflictOnce[=error]`, `?refreshFails`, `?oauthFail=<код>`, `?platformFails`,
`?manage=0`, `?unconfigured`.

## Ответы провайдеров

Текст ответа сервиса никогда не покидает сервер. Отказ проверки классифицируется (`auth`,
`network`, `rate_limit`, `config`, `reauth`, `unknown`); экран показывает фразу и шаг
исправления на языке пользователя, в `status_reason` хранится только категория. Сырого
ответа нет ни в RPC, ни в UI: маскирование значений внутри произвольного текста нельзя сделать
полным (провайдер может вернуть кред закодированным вместе с другими данными).

Identity показывается только с явным происхождением: коннектор объявляет её тип
(`connectors.Identity.Kind`: `handle` — `@name`, `email`, `phone`), и значение должно иметь форму
этого типа. Нетипизированный текст провайдера не показывается никогда. Кроме того, ни значение, ни
результат до четырёх слоёв декодирования (URL, base64 любого алфавита при любом смещении, hex, в
любом порядке) не должны содержать секретное значение кредов. Проверка выполняется и при записи, и
на выходе — для каждой строки, которую отдаёт сервис, включая записанные через общий API коннекторов
(`Store.Save`, `OAuth2.Complete`): identity, отображаемое имя и системное имя сверяются с текущими
кредами подключения — метаданные и креды берутся из одного чтения строки, так что ротация между
чтениями не пропускает старые метаданные. При любой замене кредов — изменение подключения,
повторный OAuth-вход и автоматическое обновление токена (`OAuth2.Token`) — сохраняемые отображаемое
имя и identity сверяются и со старыми, и с новыми кредами до записи
(`connectors.CarriesSecret`, `connectors.SafeIdentity`); не прошедшее очищается (отображаемое имя
становится системным).

Системное имя (`name`) не меняется никогда и показывается как есть: его выбирает администратор (или
оно выводится из вида коннектора, но не из ответа провайдера), и по нему разрешаются ссылки
`<имя>.<поле>` и ноды.

## Повтор запросов

Форма шлёт `idempotency_key` (один на открытие формы). Ключ резервируется до обращения к
провайдеру и привязан к HMAC-отпечатку запроса (`connectors.Store.Fingerprint`, ключ
шифрования подключений): тот же ключ с другим содержимым — ошибка. Отпечаток сверяется ключом, которым
он сделан (`connectors.Store.MatchesFingerprint`), поэтому повтор после ротации ключа, пока старый ключ
читается, получает результат первого запроса. Пока первый запрос
идёт, повтор получает `in_progress` и не обращается к провайдеру. Связь ключа с
подключением пишется в той же транзакции, что и само изменение
(`connectors.Store.SaveWith`), поэтому повтор после потерянного ответа всегда получает
результат первого запроса. Ключ запроса, который не дошёл до сохранения (отказ, отмена),
освобождается; резерв без результата истекает через 2 минуты, записи живут сутки
(`automation.connection_requests`). Резерв принадлежит владельцу (`owner`): запрос,
чей просроченный резерв перехватили, не может ни завершить, ни освободить новый.

## Бандл и релизы

Бандл собирается с заглушкой версии SDK вместо самой версии; модуль подставляет версию
работающего бинарника, когда отдаёт файл, и кладёт её в URL
(`/assets/automations/<версия>/…`). Кандидат релиза, который только проставляет новую
версию в Go и client-host, отдаёт тот же закоммиченный бандл без пересборки.

## AI-ноды: `ai.llm` и `ai.jev`

Коннектор `openai` (`pkg/automation/connectors/openai`) даёт ноду **LLM** (`ai.llm`), коннектор `jev`
(`pkg/automation/connectors/jev`) — ноду **Jev** (`ai.jev`). Обе работают на ключе тенанта или на платформенном
ключе (`Platform = opt_in`: тенант видит ноды без своего подключения только после того, как администратор включил
платформенный ключ; расход каждого вызова пишется на тенанта — `token_in` и `token_out` в
`automation.connector_usage`, у платформенного ключа `connection_id = platform:<kind>`).

Обе ноды идемпотентны (`Idempotent() = true`, `DryRun: execute`): вызов не имеет внешних эффектов, при неизвестном
исходе движок повторяет его (повтор оплачивается и учитывается). 429 и 5xx повторяет движок в пределах `timeout`
ноды, после `Retry-After` провайдера, если он есть. Ошибки — `CallError`: `invalid_request` (пустой prompt после
рендера, `input` не map, превышен контекст, ответ вне схемы или вне choices — без повторного запроса), `rejected`
(отказ или модерация), `unauthorized`, `rate_limited`, `unavailable`.

Модель задаётся точным id. Алиасы (`latest`, `*-latest`, `~…`, `openrouter/auto`) и «плавающие» id OpenAI без даты
(`gpt-4o`, `gpt-4o-mini`, `o3`) не сохраняются: OpenAI закрепляет модель датированным снапшотом
(`gpt-4o-mini-2024-07-18`). Смена модели — это новая версия графа. Пустая модель при сохранении заменяется на `defaultmodel` коннектора (`catalog.CompletedNode`), так что
смена env не меняет модель уже сохранённой версии.

### `ai.llm`

| Параметр | |
| --- | --- |
| `model` | Точный id; пусто — `defaultmodel` на сохранении. |
| `system`, `prompt` | Шаблоны; `prompt` не пустой после рендера. |
| `format` | `text` (по умолчанию) или `json`. |
| `schema` | JSON Schema ответа, обязательна при `format: json`, проверяется при сохранении (подмножество `output_schema`: object, array, string/enum, integer, number, boolean). Провайдеру уходит строгий режим structured output; ответ всё равно проверяется по схеме у нас. |
| `max_tokens` | 1..32000, по умолчанию 2000. |
| `temperature` | 0..2, необязательно. |
| `timeout_ms` | Срок вызова в миллисекундах, повторы включены: 1000..300000, по умолчанию 60000. Движок читает его как дедлайн вызова. |
| `images` | Список id загрузок (до 8 файлов по 5 МБ) или CEL-выражение такого списка. Только для модели с vision, иначе ошибка сохранения `model_no_vision`. |

Исходы: `done`, `failed`. Выход: `text` (для `text`), `json` (для `json`, тип выводится из `schema`), `model`, `usage`
(`input_tokens`, `output_tokens`, `cost_usd` или `null`), `latency_ms`.

Файлы для `images` читает `openai.ImageSource`; модуль `automations` подключает загрузки core (чужие загрузки не
находятся). Vision определяется по проверенному списку моделей (точные id и их снапшоты; `gpt-4o-mini-transcribe`, `gpt-4o-audio-preview` и т. п. в нём нет). Приложение с метаданными возможностей своего endpoint заменяет проверку через `openai.WithVision`.

### `ai.jev`

| Параметр | |
| --- | --- |
| `model` | Точный id модели провайдера подключения: для `openrouter` закреплённая версия TypeSafe Jev (`typesafe/jev-1.14`), для `openai` датированный снапшот чат-модели. Если модель не указана, при сохранении записывается умолчание провайдера выбранного подключения; модель другого провайдера не сохраняется (`model_provider_mismatch`). |
| `timeout_ms` | Как у `ai.llm`. |
| `input` | CEL: map с данными для решения. Значения любых типов — строка, число, bool, список, map; ключи сериализуются отсортированными. |
| `prompt` | Шаблон инструкции; по умолчанию — инструкция с правилом «при сомнении выбирай безопасный вариант». |
| `choices` | 2..100 вариантов `{value, description}`; `value` — `^[a-z][a-z0-9_]{0,39}$`, уникален, становится исходом; `low_confidence` и `failed` запрещены. |
| `min_confidence` | 0..1, по умолчанию 0.6. |

Исходы: по одному на каждый `choice`, `low_confidence` (уверенность ниже `min_confidence`; выбранное значение всё
равно в `output.choice`) и `failed`. Выход: `choice` (enum значений `choices`), `confidence`, `probabilities` (по всем
вариантам, сумма 1), `provider`, `model`, `usage`, `latency_ms`. Провайдер решения — поле подключения `provider`:
`openrouter` или `openai`. Для `openrouter` нода вызывает Decisions API OpenRouter (`POST /api/alpha/decisions`, TypeSafe Jev): типизированный выбор, вероятности и собственная уверенность провайдера — по ней и маршрутизируется исход; ответ без вероятностей или без уверенности (или с null/строкой вместо числа) — `invalid_request`. Для `openai` — чат-модель со строгим structured output (`choice` и `probabilities`; уверенность — вероятность выбранного варианта). Вероятности проверяются: по всем вариантам, каждая в [0, 1], сумма 1; ответ вне этого — `invalid_request` без повторного запроса.

### Кеш решений

Ключ решения включает отрендеренный prompt, input, модель, параметры модели, эндпоинт и подключение. Dry run в режиме `cached` берёт решение
из `automation.decision_cache` и не вызывает провайдера; смена prompt, input, модели или параметров — промах кеша.

## SMS: Eskiz и Twilio

Модуль регистрирует коннекторы `eskiz` и `twilio` (`pkg/automation/connectors/sms`) с нодами
`eskiz.send_sms` и `twilio.send_sms`; приложение может зарегистрировать свой коннектор того же вида через
`WithConnectors`. Нода видна тенанту с подключением нужного вида: своим или платформенным.

Нода не идемпотентна, потому что ни Eskiz, ни Twilio не принимают ключ идемпотентности. Запрос, который мог
дойти до провайдера (ответа нет, ответ не читается), заканчивается `failed` с кодом `outcome_unknown` и не
повторяется. Повтор — только после ответа 429 или 408, по `Retry-After`. Принятое сообщение записывается в
учёт (`unit = sms`, по числу сегментов). Входящие SMS и статусы доставки не поддерживаются.

Проверка с реальными аккаунтами — вручную: `CONTRACT_TARGET=live` и ключи в окружении
(см. `pkg/automation/connectors/sms/contract_test.go`); без него тесты идут на фейках `sms/smstest`.

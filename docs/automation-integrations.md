# Модуль `automations`: страница «Интеграции»

`modules/automations` — модуль автоматизаций SDK. Он даёт права `Automations.*` и
`Connections.Manage`, раздел в settings hub и первую страницу — «Интеграции»
(`/settings/integrations`): администратор тенанта подключает аккаунты к
коннекторам приложения, проверяет их, видит статус и где подключение используется.

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
| `AUTOMATIONS_ASSETS_DIR` | Разработка: каталог свежесобранного бандла экрана вместо встроенного. |

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

Fixture `fixtures/integrations` открывается так же: `pnpm vite fixtures/integrations`
с параметрами `?scenario=empty|error|loading|fresh`, `?lang=ru`, `?theme=dark`,
`?delay=<мс>`, `?testDelay=<мс>`, `?testFailOnce`, `?lossy`, `?oauthFail=<код>`, `?platformFails`,
`?manage=0`, `?unconfigured`.

## Ответы провайдеров

Текст ответа сервиса никогда не становится текстом интерфейса. Отказ проверки
классифицируется (`auth`, `network`, `rate_limit`, `config`, `reauth`, `unknown`); экран
показывает фразу и шаг исправления на языке пользователя, а в `status_reason` хранится
только категория. Сам ответ доступен под «Показать ответ сервиса», только когда это
безопасно: если у подключения есть секретное значение короче 8 символов, ответ не
показывается вовсе (короткое значение нельзя надёжно узнать в base64, hex или заголовке
Basic); иначе все формы значений (как есть, URL, base64, hex) и любые длинные
токеноподобные фрагменты замаскированы, ответ обрезан до 500 символов. Identity, которую
вернул коннектор (в том числе после OAuth-обмена), очищается так же; часть, где встречается
любая форма короткого секрета, не сохраняется.

## Повтор запросов

Форма шлёт `idempotency_key` (один на открытие формы). Ключ резервируется до обращения к
провайдеру и привязан к HMAC-отпечатку запроса (`connectors.Store.Fingerprint`, ключ
шифрования подключений): тот же ключ с другим содержимым — ошибка. Пока первый запрос
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

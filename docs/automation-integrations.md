# Модуль `automations`: страница «Интеграции»

`modules/automations` — модуль автоматизаций SDK. Он даёт права `Automations.*` и
`Connections.Manage`, раздел в settings hub и первую страницу — «Интеграции»
(`/settings/integrations`): администратор тенанта подключает аккаунты к
коннекторам приложения, проверяет их, видит статус и где подключение используется.

## Подключение в приложении

Модуль входит в `modules.Components()`. Приложение регистрирует свои коннекторы:

```go
automations.NewComponent(
    automations.WithConnectors(telegram.New(), smtp.New()),
    // automations.WithUsageIndex(engine.Usage) — подключает #1339
)
```

Без `WithUsageIndex` модуль берёт `connectors.UsageIndex` из контейнера, если его
кто-то предоставил; иначе «Используется в» пусто, и удаление не блокируется.

Модуль предоставляет в контейнер (по умолчанию, приложение может заменить):
`connectors.Directory`, `catalog.Registry` (доступность — хранилище подключений),
`connectors.Store` (`ConnectionStore`, `CredentialSource`), `connectors.OAuth2`,
`connectors.PlatformCredentials`, `connectors.PlatformGrants` и
`host.SecretResolver`. Extension runtime использует этот `SecretResolver`, если в
`wiring.Options.Secrets` ничего не задано: секрет пакета `<connection name>.<field>`
берётся из подключения тенанта.

## Конфигурация

| Переменная | Что задаёт |
| --- | --- |
| `AUTOMATION_CONNECTIONS_KEYS` | Ключи шифрования `id:key[,id:key…]` (32 байта, base64 или hex). Без них подключения не сохраняются, страница показывает предупреждение. |
| `AUTOMATION_CONNECTIONS_ACTIVE` | Активный ключ при ротации. |
| `AUTOMATION_PLATFORM_CREDENTIALS` | JSON `{"<kind или имя OAuth-клиента>": {"<поле>": "<значение>"}}`: ключи платформы и `client_id`/`client_secret` OAuth2-клиентов. |
| `EXTRUNTIME_UI_HOSTMANIFEST` | Манифест клиентского бандла. Без него (и без `automations.WithHostManifest`) страница не монтируется и пункта в settings hub нет; RPC и OAuth callback работают. |

Адрес возврата OAuth2 — `<origin приложения>/settings/integrations/oauth/callback`;
его нужно зарегистрировать у провайдера.

## Клиентский бандл

Экран — Solid-фича SDK, как конфигуратор расширений. Приложение добавляет её
каталог к своему:

```ts
import { solidFeatures as integrations } from "<iota-sdk>/modules/automations/presentation/integrations/web/solid-features.generated"
import { solidFeatures as app } from "./solid-features.generated"
mountSolidFeatureCatalogFromDocument({ catalog: { ...app, ...integrations } })
```

Экран использует `@iota-uz/sdk/solid` и `@iota-uz/sdk/solid-ui`; утилиты Tailwind
экрана входят в основной CSS SDK (`modules/**/*.tsx` в `styles/tailwind/pipeline.config.json`).

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
pnpm test:browser                    # Playwright: экран на fake-сервере (fixtures/integrations)
```

Fixture `fixtures/integrations` открывается так же: `pnpm vite fixtures/integrations`
с параметрами `?scenario=empty|error|loading|fresh`, `?lang=ru`, `?theme=dark`,
`?delay=<мс>`, `?platformFails`, `?manage=0`, `?unconfigured`.

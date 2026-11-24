# Коннектор Telegram bot (`telegram_bot`)

`pkg/automation/connectors/telegram` — коннектор графов автоматизаций (#1341): отправка сообщения,
вопрос с кнопками, ожидание текстового ответа и триггер на входящее сообщение боту. Клиент —
`gotgbot/v2`; токен бота никогда не попадает в output, логи и ошибки (URL Bot API содержит токен, поэтому
ни одна ошибка запроса его не несёт).

## Подключение в приложении

```go
tg := telegram.New(telegram.Config{
    PlatformToken: cfg.App.TelegramBotToken, // APP_TELEGRAMBOTTOKEN; пусто — платформенного бота нет
    PublicOrigin:  origin,                   // https-origin приложения для setWebhook
    Inbound:       runtime.Inbound(),        // engine.Inbound
    Uploads:       services.NewTelegramUploads(uploadService, pool), // вложения; без него файл не сохраняется
    Connections:   store,                    // needs_reauth и список известных чатов
})
automations.NewComponent(automations.WithConnectors(tg))
_ = tg.RegisterStart(userLinking) // необязательно: StartHandler (#1349)
```

- Креды: `token` (x-secret). Webhook secret генерирует коннектор при `Provision` (32 символа
  `[A-Za-z0-9_-]`) и отдаёт сервису, тот пишет его в inbound state подключения (ключ `telegram_webhook`) в
  той же транзакции, что и сохранение; `Verify` читает его оттуда (`connectors.InboundReader`). У бота платформы
  строки подключения нет, его секрет выводится HMAC-ом из токена (`WebhookSecret`).
- Сохранение подключения вызывает `setWebhook` на `/automations/hooks/{connection_id}`; удаление и
  отключение — `deleteWebhook` (только если webhook ещё наш; ошибка вызова не мешает удалению). Это
  `connectors.Provisioner`, его вызывает `IntegrationsService`. Бот платформы регистрируется при старте
  модуля `automations` (`connectors.PlatformStarter`) на `/automations/hooks/platform/telegram_bot`.
- Бот платформы (политика `auto`) доступен всем тенантам как платформенный credential. Его входящие
  апдейты не публикуются, кроме нажатия кнопки (тенант — по токену корреляции) и `/start <payload>` с
  зарегистрированным префиксом (`StartHandler.TenantOf`). Поэтому триггер «входящее сообщение»
  работает только для собственного бота тенанта.

## Один бот — одно подключение

У бота один webhook, поэтому бот не может принадлежать двум подключениям, включая бота платформы. Владение
фиксируется в БД (`automation.exclusive_identities`, `connectors.ExclusiveIdentity`): id бота резервируется за
подключением (или за платформенным credential при старте модуля) в транзакции сохранения под advisory-lock на
этот id, поэтому два одновременных сохранения не могут оба настроить webhook; проигравшее получает ошибку поля
`token` (`in_use`), подтверждение её не снимает. Резерв держит и отключённое подключение, снимает удаление.
Для старых подключений без резерва остаётся проверка `getWebhookInfo`: webhook этого приложения на живое
другое подключение (`ConnectionExists`) или на маршрут бота платформы — тоже `bot_in_use`; webhook удалённого
подключения ничей. `Deprovision` удаляет webhook только при точном совпадении URL.
Платформенный бот регистрируется и резервируется для credential, который действует (конфигурация имеет
приоритет над `APP_TELEGRAMBOTTOKEN`): `StartPlatform(ctx, creds)`.

## Чужой webhook

Перед сохранением и перед повторным включением отключённого подключения `Advise` (`connectors.Advisor`) читает `getWebhookInfo`; если у бота webhook на чужой
URL (`foreign_webhook`) или webhook нет вовсе (`polling_unknown`: опрашивает ли бота другой сервис, узнать нельзя —
запрос `getUpdates` перехватил бы долгий опрос), форма
показывает предупреждение и не завершает подключение без подтверждения (`confirm` в
`create`/`update`/`set_disabled`, ошибка `confirmation_required`). `Test` при этом проходит.

`Provision` авторитетен: он заново смотрит webhook в момент изменения и сам требует подтверждение (`consent`), а
`Advise`, не сумевший получить состояние провайдера, отказывает в сохранении. Секрет подключения привязан к аккаунту бота и сохраняется между
сохранениями и повторными включениями того же бота (при смене бота генерируется новый), `Provision` читает inbound state
из транзакции сохранения (одно соединение с БД), поэтому не завершившееся сохранение или
конкурирующее не оставляет Telegram с секретом, которого нет в SDK; такое сохранение снимает webhook, который успело
поставить. Повторное включение — одна транзакция за проверкой версии строки. Очистка webhook (отключение, замена токена, неудачное
сохранение) выполняется под тем же advisory-lock на аккаунт и по свежему состоянию строки (`Store.WithLatest`): запоздавшее
отключение не удалит webhook успешного включения. Резерв аккаунта следует за кредами при любом статусе (в том числе у
отключённого подключения). Платформенный credential, чей аккаунт занят подключением тенанта, снимается с выдачи
(`PlatformWithdrawer`); при удалении настройки платформенного бота его резерв освобождается (webhook без токена не
снять). Повторное включение и замена токена на бота без webhook требуют подтверждения `polling_unknown`.

## Ноды

| Нода | Поведение | Outcomes |
| --- | --- | --- |
| `telegram.send` | call, не идемпотентна | `sent`, `failed` |
| `telegram.ask` | call + ожидание нажатия | `<options[].outcome>`, `timeout`, `failed` |
| `telegram.read` | wait на `telegram.message_received.v1` | `received`, `timeout` |

- `chat_id` — число (id чата) или выражение CEL (`event.chatId`, `"@username"`); `thread_id` — тема форума.
  В редакторе поле выбирается из известных чатов («Личные», «Группы»), вводится вручную (id или
  @username) или задаётся выражением (`x-options-source: telegram.chats`, `EditorApi.searchOptions`).
- `send.files` — id загрузок: один файл идёт как фото/документ с подписью, несколько — альбомом документов.
- `ask`: кнопка — `callback_data = a:<токен корреляции>:<номер>` (≤ 64 байт), таблицы соответствий нет.
  Каждое нажатие подтверждается `answerCallbackQuery`; после первого принятого ответа (после коммита, см.
  `engine.PublishedAnswerer`) клавиатура снимается, а текст дополняется `→ <выбранный вариант>`. Нажатие
  закрытого или просроченного вопроса получает «Ответ уже принят или время истекло» и снимает клавиатуру;
  при самом таймауте сообщение не меняется. Outcome по номеру кнопки определяет `engine.ParamAnswerer`.
- `read` работает только с собственным ботом тенанта (платформенный credential не предлагается и не принимается,
  `catalog.TenantConnectionNode`); если беседу назвать нельзя (чат по @username, подключение не определилось), нода
  падает, а не слушает беседу запуска. Ждёт только сообщение с текстом (вложение без подписи — событие для триггеров, но не ответ); подключение,
  если оно не указано, движок определяет как для вызова и строит conversation `<connection_id>:<chat_id>`
  из отрендеренного `chat_id`. `from_user_id` — id или выражение, читаемое в момент прихода сообщения (`nodes.<id>.output…`;
  `event` там — входящее сообщение, поэтому выражение с `event` отклоняется). Output — `{event: <payload>}`.
- Шаблоны внутри списков (`options[].label`, `url_buttons[].text`) — путь `items[].field` в
  `NodeSpec.Templates`/`Values`.

## События

`telegram.message_received.v1` (conversation `<connection_id>:<chat_id>`; только собственные боты тенанта — для
этого события каталог не предлагает и не принимает платформенный credential (`catalog.TenantConnectionTriggers`);
свои сообщения бота — `by_self`; сообщение без текста, но с файлом тоже событие, `event.text` пуст;
`files[]` — `upload_id, name, mime, size, url` или `error`: `too_large` (>20 МБ), `download_failed`,
`storage_failed`, `no_storage`). Параметры триггера: `commands: ignore|only|include` (по умолчанию
`ignore`), `command_names`, `chat_ids`, `private_only`. `/start` с префиксом `StartHandler` не публикуется.

`telegram.callback_answered.v1` — нажатие кнопки; только будит `ask`.

## Ошибки и лимиты

- `403` и `400 chat not found` и подобные — `failed`, код `rejected`; соединение остаётся `connected`.
- `401` — `failed` с кодом `unauthorized`, соединение получает `needs_reauth`.
- `429` — `CallResult.RetryAfter` из `retry_after`; повтор по правилу движка, второго сообщения нет.
- Потерянный ответ после отправки — `outcome_unknown`, а не повтор.
- Каждое доставленное сообщение (альбом файлов — несколько) пишется в `UsageRecorder` (`unit: message`), для
  платформы и тенанта раздельно; ответ на `/start` считается тому тенанту, чья ссылка (`Config.Usage`).
- Альбом из нескольких файлов не принимает url-кнопки (проверка при сохранении графа и перед отправкой).

## Тесты и фейк

`telegramtest` — in-memory Bot API (`sendMessage`, `sendPhoto/Document/MediaGroup`, `editMessageText`,
`editMessageReplyMarkup`, `answerCallbackQuery`, `getFile`, `getMe`, `setWebhook`, `getWebhookInfo`,
`deleteWebhook`, `getUpdates`), доставляющий webhook в SDK (сообщение, фото, документ, нажатие кнопки,
сбои `Fail`, блокировка `Block`). Контрактный тест:

```sh
go test ./pkg/automation/connectors/telegram -run TestBotAPIContract                       # CONTRACT_TARGET=fake
CONTRACT_TARGET=live TELEGRAM_CONTRACT_TOKEN=… TELEGRAM_CONTRACT_CHAT=… go test … -run TestBotAPIContract  # вручную, не в CI
```

Сценарии с движком и базой — `pkg/automation/engine/telegram_test.go`.

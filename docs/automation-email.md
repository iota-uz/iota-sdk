# Коннектор Email: SMTP, IMAP, ожидание ответа, вход через Google и Microsoft 365

Пакет `pkg/automation/connectors/email` даёт графам автоматизаций почту: отправить письмо, дождаться
ответа на него, стартовать от входящего письма, пометить или перенести письмо. Это протоколы SMTP и
IMAP, а не API провайдеров: для Gmail и Microsoft 365 меняется только аутентификация (SASL XOAUTH2).

## Подключение в приложении

```go
cs := email.New(email.Config{
    Platform:    &smtpCfg,                                  // SMTP_* приложения (smtpconfig.Config)
    Inbound:     rt.Inbound(),
    OAuth:       oauth,                                     // connectors.OAuth2 (Token обновляет токены)
    Uploads:     services.NewEmailUploads(uploadSvc, pool, uploadsCfg.PrivatePath()), // вложения входящих писем
    Connections: store,                                     // связывает два connection одного входа
})
automations.NewComponent(automations.WithConnectors(cs...))
```

`New` возвращает два коннектора: `smtp` (отправка) и `imap` (входящие, ожидание ответа, пометки; он же
`engine.Poller`).

Что подключает модуль `automations` сам:

| Что | Как |
| --- | --- |
| SMTP приложения как платформенный credential | `smtp.PlatformCredential()` берёт `Config.Platform`; модуль добавляет его в `connectors.PlatformCredentials` (`platform:smtp`, политика `auto`: доступен каждому тенанту, только для отправки), если `AUTOMATION_PLATFORM_CREDENTIALS` не называет `smtp`. |
| Маршрут вложений | `GET /automations/attachments/{id}` (`presentation/attachments`) — только для пользователей с `Automations.Read` того же тенанта. |
| Вход через Google / Microsoft | Форма подключения: поле «Аккаунт» (`preset`) предлагает «любой сервер» и провайдеров из `oauth_presets` (скрыты те, для кого нет OAuth2-клиента); выбор провайдера отправляет в `oauth_start` с `preset`, «Войти снова» повторяет вход с тем же `preset`. Сервис: `IntegrationsService.OAuthStartPreset` и `OAuthComplete`; RPC `automations.connections.oauth_start` принимает `preset`, список коннекторов отдаёт `oauth_presets` — только пресеты, для которых в приложении есть OAuth2-клиент. |

Что остаётся приложению:

| Что | Как |
| --- | --- |
| Триггер `subject_contains` | `engine.Config.MatchHooks` ← `email.MatchHooks()`. Без этого граф с триггером `email.received.v1` не сохранится: правило сопоставления не зарегистрировано. |
| Опрос IMAP | `rt.NewPoller(...).Serve(ctx)`; опрашиваются connection, которые использует включённый граф: триггером `email.received.v1` или нодой `email.wait_reply`. Connection, который не использует ни один граф, опрашивается только чтобы держать курсор в конце ящика (`engine.Baseliner`): граф, включённый позже, не получит старой почты и не пропустит ответ, пришедший до его первого опроса. Курсор ставится не позже чем через минуту после подключения; кроме того, `email.send` перед отправкой ставит курсор ящиков отправителя и `Reply-To` (`Config.Connections`), у которых его нет, а базовая линия уступает графу, включённому после скана, который назвал подключение свободным (сколько бы он ни оставался в кэше). |
| Вложения | `services.NewEmailUploads` — адаптер над сервисом загрузок SDK; файлы лежат в приватном пространстве (`.private/automations-email`), анонимного URL у них нет. |

## Подключение тенанта

* **Логин и пароль (пароль приложения)** — для любого сервера. Форма `smtp`: `host`, `port` (465 — TLS,
  587 — STARTTLS), `username`, `password`, `from_address`, `from_name`; форма `imap`: `host`, `port`
  (993 — TLS), `username`, `password`, `mailbox` (по умолчанию `INBOX`). `security` (`auto`, `tls`,
  `starttls`) задаёт защиту для нестандартных портов. Соединение всегда шифруется: сервер без TLS или
  STARTTLS не получает логина.
* **Gmail** и **Microsoft 365** (`preset: gmail | m365`) — OAuth2. Basic-аутентификация для Microsoft 365
  не предлагается (Exchange Online отключает basic SMTP AUTH). Клиентские id и secret берутся из
  платформенных credentials `google_oauth` и `microsoft_oauth` (`AUTOMATION_PLATFORM_CREDENTIALS`);
  пресет скрывается, когда клиента нет (`x-oauth-client` в схеме).

Scopes: Gmail — `https://mail.google.com/`; Microsoft 365 — `https://outlook.office.com/SMTP.Send`,
`https://outlook.office.com/IMAP.AccessAsUser.All`, `offline_access`. К ним добавлены `openid email`
(у Microsoft и `profile`): из identity-токена коннектор узнаёт адрес ящика, для которого выдан токен
(в XOAUTH2 он нужен как `user=`). Identity-токен не сохраняется.

### Один вход — два connection

Страница вызывает `automations.connections.oauth_start` с `preset` (`gmail` или `m365`) — сервис
(`OAuthStartPreset`) отдаёт адрес провайдера; на callback `OAuthComplete` сохраняет **оба** connection
в одной транзакции (`connectors.GroupSaver`): `smtp` и `imap` с общим адресом ящика (`username`) и `login_id`.
Повторный вход под тем же адресом обновляет оба.

Набор токенов один: connection, с которого начат вход, владеет им, второй хранит `token_from` — id
владельца. Обновляет токены только владелец (`OAuth2.Token` под его блокировкой), второй копирует
обновлённый набор, поэтому провайдер, который ротирует refresh-токен, никогда не видит два. Статус владельца
на это не влияет: выключенный владелец продолжает обслуживать включённый connection; при удалении владельца
набор токенов переходит к второму connection. Вход не перезаписывает connection того же ящика, сделанный
логином и паролем, и не включает выключенный connection. Если
провайдер отказал в обновлении (доступ отозван в аккаунте) или сервер отверг токен, оба connection входа
становятся `needs_reauth`.

`email.Config.Providers` переопределяет эндпоинты пресета (суверенные облака Microsoft, тестовые серверы).

## Ноды

| Нода | Поведение | Параметры | Исходы |
| --- | --- | --- | --- |
| `email.send` | call, не идемпотентна, требует `smtp` | `to` (1–50), `cc`, `bcc`, `subject`, `text`, `html`, `reply_to` — шаблоны; `in_reply_to` — CEL с Message-ID | `sent`, `failed` |
| `email.wait_reply` | wait, требует `imap` | `message_id` (CEL, обычно `nodes.<send>.output.message_id`), `thread_id` (CEL, см. ниже), `from` (шаблон), `timeout` (по умолчанию 3d, ≤ 30d) | `replied`, `timeout` |
| `email.label` | call, идемпотентна, требует `imap` | `message_id` (CEL), `mark_seen`, `keywords`, `move_to` | `done`, `not_found`, `failed` |

`email.send` делает Message-ID до отправки: `<{хэш ключа идемпотентности}@{домен отправителя}>`; он же в
`output.message_id`, его же получит повтор того же вызова. Выход: `message_id`, `accepted`, `rejected`.

Ошибки отправки:

* ответ SMTP `4xx` — `failed` с кодом `unavailable` (`Retryable`); автоповтора нет: нода неидемпотентна.
  `4xx` для любого одного получателя прерывает отправку до `DATA`: письмо не уходит никому;
* `5xx` по всем получателям — `failed` с кодом `rejected`; отвергнута часть адресов — `sent`, отвергнутые в
  `output.rejected`;
* обрыв соединения после передачи письма, до ответа сервера, — `failed` с кодом `outcome_unknown`;
  повторной отправки нет;
* отказ AUTH — `unauthorized`. Каждое отправленное письмо — запись в `UsageRecorder` (`unit: message`, 1).

### Ожидание ответа

`email.wait_reply` ждёт `email.received.v1` с subject `conversation`
`<imap_connection_id>:<message_id>`. Входящему письму ключ даёт **первый элемент `References`, иначе
`In-Reply-To`**; своих маркеров в теме и адресе нет. Поэтому ответ из почтового клиента (с `References`)
будит именно тот запуск, который отправил письмо, а ответ на другое письмо ящика — нет. Если `from`
задан, будит только письмо с этого адреса (регистр не важен).

Ответ должен ещё и отвечать на это письмо: его `In-Reply-To` называет письмо, а при его отсутствии последний
элемент `References` — письмо, на которое он отвечает. Старшие письма в `References` — только история. Два запуска, отправивших письма в одну цепочку, будятся каждый своим ответом.

Ответ ссылается на **первое** письмо цепочки. Если письмо было отправлено как ответ (`in_reply_to`), его
ответ принадлежит цепочке исходного: передайте в `thread_id` `Message-ID` первого письма цепочки
(`event.threadId` входящего, взятый нодой раньше). Без `connection` нода выбирает единственное подключение
`imap`; ожидание всё равно сужает фильтр `event.threadId == …`.

В `event` вывода — payload найденного письма.

## Событие `email.received.v1`

Payload: `connectionId`, `mailbox`, `messageId`, `inReplyTo?`, `threadId`, `references`, `from`, `to`, `cc`,
`subject`, `text` (текстовая часть или html→text, ≤ 100 000 символов), `hasHtml`, `isReply`, `isAuto`,
`attachments`, `receivedAt`. **HTML письма в payload не передаётся**, ссылки не открываются.

Вложение: `filename`, `contentType`, `size`, и после сохранения в хранилище загрузок SDK — `upload_id` и
`url` (маршрут `/automations/attachments/{id}`: отдаёт файл как скачивание, с `nosniff` и `sandbox`). До 25 MB каждое и до 50 MB на письмо; сверх лимита, при сбое хранилища или без него —
`attachments[].error` (`too_large`, `message_too_big`, `storage_failed`, `no_storage`).

Параметры триггера:

| Параметр | Что делает |
| --- | --- |
| `from` | адреса (`from.address`, регистр не важен) |
| `subject_contains` | подстрока темы (регистр не важен); нужен `email.MatchHooks()` |
| `skip_replies` (по умолчанию `true`) | пропускать ответы в цепочке: есть `In-Reply-To` или тема `Re:` / `Ответ:` |
| `skip_auto` (по умолчанию `true`) | пропускать автоматические письма: `Auto-Submitted`, `List-Id`, `Precedence: bulk` |

### Опрос

Интервал 60 с (минимум для движка 30 с), IDLE не используется. Курсор `{mailbox, uidvalidity, last_uid}`:

* первый опрос после подключения и первый после смены `UIDVALIDITY` ставят курсор на последний UID и ничего
  не публикуют — старые письма графы не стартуют;
* `DedupKey` — `Message-ID`, без него `uidvalidity:uid`; повторная доставка того же письма события не создаёт;
* отказ AUTH (неверный пароль, отозванный OAuth) переводит connection в `needs_reauth` и останавливает опрос;
  сетевая ошибка курсор не двигает;
* за опрос читается до 50 писем, остальные — следующим; ошибка чтения в середине пачки возвращается как
  ошибка опроса, курсор остаётся прежним, пачка читается заново (дубли отсекает dedup);
* письмо больше 100 MB читается по структуре (`BODYSTRUCTURE`): заголовки, первый мегабайт текста и каждая
  часть-вложение отдельно (лимиты считаются по реально декодированным байтам, а не по размеру закодированных октетов): часть в пределах лимитов (25 MB после декодирования, 50 MB на письмо) сохраняется, как
  из обычного письма, а часть сверх лимита описывается с `error` (`too_large` или `message_too_big`), её тело
  не скачивается; часть, которую нельзя декодировать, получает `error: unreadable`, и письмо всё равно публикуется.

`email.label` с `move_to` использует нативный `MOVE`; без него — `COPY` с проверкой, затем `\Deleted` и
`UID EXPUNGE` (при `UIDPLUS`), без `UIDPLUS` оригинал остаётся помеченным `\Deleted`: письмо не теряется.
Повтор вызова находит живую копию в целевой папке по точному `Message-ID` (подстрока заголовка и письмо с флагом `\\Deleted` копией не считаются) и не копирует письмо второй раз.

## Безопасность

* Пароли и токены не попадают в логи: диалог с сервером нигде не пишется; при `Config.Debug` каждая
  AUTH-строка и каждое значение credentials (и их base64) маскируются.
* Тексты ошибок сервера не возвращаются: `Test` и узлы отдают код и статус.
* `Message-ID`, адреса и тема проверяются, переводов строк в заголовках нет; `bcc` только в конверте.

## Тесты

`emailtest` — фейковый сервер SMTP и IMAP (логин, XOAUTH2, TLS и STARTTLS, сбои по сценарию):

```sh
go test ./pkg/automation/connectors/email/...
go test ./pkg/automation/engine -run TestEmail        # приёмка: send → wait_reply, UIDVALIDITY, триггер
```

Контрактный тест — то же против реального ящика:

```sh
CONTRACT_TARGET=live EMAIL_CONTRACT_SMTP_HOST=smtp.example.com EMAIL_CONTRACT_IMAP_HOST=imap.example.com \
EMAIL_CONTRACT_USER=me@example.com EMAIL_CONTRACT_PASSWORD=<пароль приложения> \
  go test ./pkg/automation/connectors/email -run TestMailServerContract -v
```

---
id: notification-recipients-targets
title: Получатель, субъект и канальная цель уведомления
status: accepted
date: 2026-11-11
deciders: [unknown]
area: core
applies_to:
  - modules/core/notifications/addressee.go
  - modules/core/notifications/content.go
  - modules/core/notifications/channel.go
  - modules/core/services/notification_channel_service.go
  - modules/core/services/notification_channel_resolve.go
  - modules/core/services/notification_dispatch_service.go
  - modules/core/services/notification_routing_service.go
  - modules/core/infrastructure/persistence/notification_delivery_repository.go
  - migrations/changes-1793400000.sql
tags: [core, notifications, runtime]
refs: [iota-uz/iota-sdk#926, iota-uz/iota-sdk#920, iota-uz/iota-sdk#925, iota-uz/iota-sdk#927, iota-uz/iota-sdk#411]
supersedes: []
superseded_by: []
---

## Контекст

Модель доставки каналов хранила одну строку на событие, пользователя ERP и
канал (`user_id integer NOT NULL`, внешний ключ на `users`). Внешний клиент без
учётной записи не мог быть получателем, push не раскрывался в несколько
устройств одного человека, субъект уведомления не отделялся от получателя, а
адрес искался при отправке одним значением на пользователя.

## Решение

- Получатель — `Addressee` одного из трёх видов: `user` (inbox плюс каналы по
  адресам и устройствам пользователя), `record` (типизированная ссылка
  `{entity, key}`, раскрывается `ContactResolver`, зарегистрированным на entity)
  и `address` (прямые адреса по каналам). Субъект — отдельная ссылка на объект
  в самом уведомлении (`Event.Subject`). Канальная цель (адрес или устройство)
  — третья независимая идентичность.
- Экземпляр уведомления — строка `core.notification_notices`: ключ и версия
  события, ссылка на снимок определения, идентичность дедупликации, продюсер
  (invocation/event/instance+occurrence), субъект, режим `rule|explicit` и
  закреплённый конверт события. Создаётся в транзакции продюсера вместе с
  задачей диспетчера.
- Доставка — строка `core.notification_deliveries` на (уведомление, получатель,
  канал, цель) с `recipient_kind`, nullable `recipient_user_id` и ссылкой на
  запись, `target_address`/`target_device_id`. In-app существует только для
  пользователя и ссылается на строку inbox; read/unread остаётся свойством
  inbox.
- Попытки — `core.notification_delivery_attempts` (номер, начало и конец, исход
  `sent|failed|permanent|ambiguous`, квитанция провайдера, код ошибки без
  текста провайдера). Неизвестный исход различим: `ambiguous` не повторяется
  автоматически, оператор повторяет его вручную; ключ идемпотентности
  провайдера стабилен между попытками.
- Транзакция продюсера пишет только намерения: уведомление, задачу и строки
  (получатель, канал) в статусе `unresolved`. После коммита воркер без
  транзакции раскрывает цели (адреса пользователя, реестр устройств
  `core.notification_push_devices`, контакты записи), применяет бизнес-политику
  и рендерит содержимое закреплённой версией через `ContentRenderer`, затем
  заменяет намерение строками по целям. Один сбой цели не затрагивает соседние.
- Отсутствие адреса или устройства — видимый исход `no_address` на
  получателя и канал; отказ в доступе к субъекту или политикой — `skipped` с
  кодом.
- Явная отправка принимает все три вида получателей, не читает правило события
  и не перенацеливается им. Правила сохраняют семантику USER/GROUP/ROLE/EVENT_RECIPIENT.
- Доступ пользователя к субъекту (`SubjectAccess`) и согласие/отказ внешнего
  контакта (`DeliveryPolicy`) — подключаемые интерфейсы; схемы согласия в ядре
  нет, по умолчанию разрешено всё.

## Последствия

Новые адаптеры, ретраи и TS-рендереры подключаются через интерфейсы без смены
схемы. Внешним контактам не создаются фиктивные пользователи. Токен push
сохраняется в строке доставки на время истории; отозванное устройство может
получить сообщение, если оно уже раскрыто. Рендер in-app содержимого остаётся
в транзакции воркера диспетчера, поэтому его реализация должна быть быстрой и
локальной.

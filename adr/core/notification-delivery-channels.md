---
id: notification-delivery-channels
title: Каналы доставки уведомлений и состояние по каждому каналу
status: accepted
date: 2026-11-09
deciders: [unknown]
area: core
applies_to:
  - modules/core/notifications/channel.go
  - modules/core/services/notification_channel_service.go
  - modules/core/services/notification_routing_service.go
  - modules/core/services/notification_dispatch_service.go
  - modules/core/infrastructure/persistence/notification_delivery_repository.go
tags: [core, notifications, email, sms, push]
refs: [iota-uz/iota-sdk#926, EAI-UZ/granite#3780]
supersedes: []
superseded_by: []
---

## Контекст

Маршрутизация уведомлений доставляла только во внутренний inbox. Issue #926
требует email, SMS и push как каналы общей системы, различимый исход по каждому
каналу и повтор одного канала без повторной отправки остальных.

## Решение

- Канал — значение `notifications.Channel` (`in_app`, `email`, `sms`, `push`),
  выбирается в `Rule.Channels`; пустой набор означает только in-app, поэтому
  существующие правила не меняются.
- SDK содержит только контракты `ChannelAdapter` и `AddressResolver`.
  Конкретные провайдеры и хранилище адресов/устройств подключает приложение через
  `ModuleOptions`; канал без адаптера выбрать нельзя.
- Состояние хранится в `core.notification_deliveries`: одна строка на
  tenant/событие/получателя/канал со статусом `pending|sent|failed|no_address`,
  попытками, квитанцией провайдера и отрисованным локализованным контентом с
  версией события. Строки создаются в той же транзакции, что и запись в inbox и
  чекпоинт диспетчера.
- Провайдеры вызываются воркером после коммита: короткая транзакция захватывает
  строки (счётчик попыток + аренда), затем вызов без транзакции, затем запись
  исхода. Доставка at-least-once; exactly-once вне объёма.
- Адресная отправка (`EnqueueAddressed`) использует ту же очередь, но не читает
  правило события, поэтому настройки администратора не подменяют аудиторию
  продюсера.

## Последствия

Повтор одной доставки меняет только её строку. Текст ошибки провайдера не
сохраняется, чтобы секреты не попадали в историю. Автоматическая сверка
провайдеров, quiet hours и предпочтения пользователей остаются вне объёма.

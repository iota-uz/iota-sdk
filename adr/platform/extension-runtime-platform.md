---
id: extension-runtime-platform
title: Расширения — обычные TS-пакеты с чистыми контрактами, общий Node pool и транзакции, которыми владеет Go
status: accepted
date: 2026-11-11
deciders: []
area: platform
applies_to:
  - pkg/extruntime/**
  - pkg/listext/**
  - web/runtime/**
  - docs/extension-runtime.md
tags: [extensions, runtime, typescript, workflows, notifications]
refs:
  - https://github.com/iota-uz/iota-sdk/issues/411
  - https://github.com/iota-uz/iota-sdk/issues/412
  - https://github.com/iota-uz/iota-sdk/issues/413
  - https://github.com/iota-uz/iota-sdk/issues/414
  - https://github.com/iota-uz/iota-sdk/issues/415
  - https://github.com/iota-uz/iota-sdk/issues/417
  - https://github.com/iota-uz/iota-sdk/issues/418
  - https://github.com/iota-uz/iota-sdk/issues/419
supersedes: []
superseded_by: []
---

## Решение

Расширение — обычный ESM TypeScript-пакет с тремя раздельно собираемыми графами:
чистые контракты (`@iota-uz/sdk/runtime`), серверные реализации
(`@iota-uz/sdk/runtime/server`) и клиентский UI на Solid
(`@iota-uz/sdk/runtime/client`). ID ресурса объявляется один раз в контракте;
реализация связывается по типизированной ссылке; registry строится из экспортов
entrypoints. Серверный импорт в клиентском графе — ошибка сборки
(`graph.serverInClient`), а не вырезание обработчиков.

Серверный код исполняется в общем прогретом пуле Node-процессов из той же
поставки, что и Go. Транзакциями владеет Go: обычная команда делает внешние
вызовы вне транзакции и открывает `ctx.transaction`; атомарная команда целиком
идёт в одной Go-транзакции; `tx.call` принимает только операции и атомарные
команды. Данные — PostgreSQL-таблица на сущность со стабильными физическими
колонками; дополнительные поля штатных сущностей — отдельные overlay-таблицы,
видимые в штатных списках и экспортах через `pkg/listext`.

Долговечные процессы исполняет собственный replay-journal движок в Go
(журнал вхождений, квитанции `wf.transaction` в той же транзакции, fencing по
`lease_epoch`, таймеры, сигналы, задачи людям, закреплённые артефакты). DBOS
квалифицирован анализом и отклонён: его транзакции не коммитятся в Go-owned
транзакции, а восстановление по версии приложения не совместимо с общим пулом
нескольких версий.

## Последствия

- Одна активная реализация команды, экрана и оболочки на приложение
  (`extruntime.bindings`), без fallback на штатную реализацию после ошибки.
- Уведомления идут через общий сервис ядра: адресаты user/record/address,
  subject и target различаются, исходы по каждой цели.
- Управление (черновик, Git-импорт, сборка, review по хешам, активация) — один
  backend для HTTP, MCP и конфигуратора.
- Общий пул — доверенный домен: изоляции памяти между пакетами нет; внешняя сеть
  не exactly-once; обычная команда может частично закоммитить до потери ответа.

## Отклонённые альтернативы

- Bun-апплеты с JSONB data API: неудобный конечный API и универсальное хранилище
  вместо типизированных таблиц.
- Обычная очередь вместо durable engine: не даёт replay, закрепления кода и
  ожиданий без занятого worker.

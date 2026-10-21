---
id: reusable-lens-reports
title: Отчёт Lens отделён от размещения и результата исполнения
status: accepted
date: 2026-10-12
deciders: [unknown]
area: platform
applies_to:
  - pkg/lens/report/**
  - pkg/lens/document/**
  - web/lens/src/reports/**
tags: [lens, reports, composition]
refs:
  - https://github.com/iota-uz/iota-sdk/issues/1056
supersedes: []
superseded_by: []
---

## Контекст

Системные DashboardSpec и execution snapshots не описывают библиотеку пользовательских отчётов. Повторное использование требует идентичности отчёта, отдельной идентичности размещения и восстановления после истечения snapshot. Пользователь запросил реализацию #1056 и пилот Granite Sales без сохранения обратной совместимости.

## Решение

Definition хранит постоянную ссылку на host source, версию его контракта, вид и параметры. Placement ссылается на Definition и владеет подписью, размером и переопределениями. Composition объявляет строки размещений. SDK не хранит SQL, ACL или записи БД в определении отчёта.

Host предоставляет определения и авторизованные Source builders. Compose преобразует блоки в один существующий Lens execution graph с независимыми namespace размещений. Grid передаёт вложенные строки, подпись и span на wire вместо прежнего flatten. Открытый Resolved неизменен; обновление источника предлагается через Changed и применяется при новом Resolve. Версия source contract несовместима — FailedPrecondition.

Solid-черновик отделён от подтверждённого host состояния. Save/Publish передаёт expectedRevision; только acknowledgement обновляет saved base. Поздние ручные правки не теряются. SDK не публикует изменения автоматически.

## Обоснование

Один контракт позволяет объявлять системные дашборды короткой композицией и сохранять пользовательские варианты тех же отчётов. Повторное размещение не должно копировать источник или временный результат. Исполнение, drill и export остаются у существующего runtime.

## Последствия

Сохранение, ownership, права и жизненный цикл серверных записей принадлежат consumer. Полная семантика наследования и пересечения фильтров продолжается в #1057. Совместимость прежнего поведения Grid, терявшего структуру, не сохраняется.

## Как проверить

Go report tests сохраняют определение в memory/file hosts, исполняют две независимые композиции через реальный Lens runtime и валидируют wire. Solid tests проверяют Save/Cancel, конфликт и правки во время сохранения. lens-report-example генерирует fixture тем же Go compiler/runtime, а story и browser lane проверяют составные размещения.

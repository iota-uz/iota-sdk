---
id: extension-ui-reads-one-activated-snapshot
title: UI расширений читает один снимок активации, страницы и замены отдаёт один контроллер
status: accepted
date: 2026-11-11
deciders: []
area: platform
applies_to:
  - pkg/extruntime/ui/**
  - pkg/listext/**
  - pkg/shellext/**
  - web/runtime/src/client-host/**
  - modules/core/presentation/templates/layouts/authenticated.templ
tags: [extensions, shell, navigation, lists, solidjs]
refs:
  - https://github.com/iota-uz/iota-sdk/issues/416
  - https://github.com/iota-uz/iota-sdk/issues/953
supersedes: []
superseded_by: []
---

## Решение

Страницы, навигация, колонки нативных списков, замена экрана и замена оболочки
читают один неизменяемый `Snapshot` активированных манифестов и открытых строк
`extruntime.bindings`. Индекс строится один раз на ревизию; Node не вызывается
при рендере страницы или меню, активация и откат действуют со следующего
запроса без перезапуска.

Один контроллер `pkg/extruntime/ui` регистрируется при старте до остальных
контроллеров и совпадает только с путями, которые сейчас обслуживает активная
страница или привязка `screen`. Поэтому замена нативного маршрута не требует
изменения нативного контроллера, а при пустой активации контроллер невидим.
Права страницы проверяются на сервере независимо от меню.

Навигация проецируется в `NavigationItem` запроса через `pkg/shellext`; из тех же
элементов строятся Templ-sidebar и `pkg/shellcontext`. Привязка `shell` публикует
контекст запуска и модуль на каждой аутентифицированной странице, а клиент
активирует renderer через `ShellHost` с откатом на стандартную оболочку.

Нативные списки объявляют стабильный ID (`crm.lists.clients`) и получают от
`pkg/listext` join/select/filter/sort SQL по полям overlay с учётом доступа к
полям, значения по умолчанию и доступа к строкам overlay до пагинации, подсчёта
и экспорта.

## Последствия

Закрытие привязки возвращает стандартную оболочку и родной экран; `?iota-shell=standard`
остаётся ручным откатом. Скрытие пункта патчем меню не закрывает маршрут.

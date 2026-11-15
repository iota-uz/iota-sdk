---
id: shell-context-is-published-by-authenticated-layout
title: Базовая навигация оболочки публикуется layouts.Authenticated и читается одним реестром документа
status: accepted
date: 2026-11-10
deciders: [moknelis]
area: platform
applies_to:
  - pkg/shellcontext/**
  - modules/core/presentation/templates/layouts/authenticated.templ
  - web/client-host/src/shell-context.ts
tags: [shell, navigation, templ, solidjs]
refs:
  - https://github.com/iota-uz/iota-sdk/issues/953
supersedes: []
superseded_by: []
---

## Решение

`layouts.Authenticated` встраивает `<script id="iota-shell-context" type="application/json">`
с версионированным контекстом оболочки: плоский список пунктов навигации,
которые уже отфильтрованы по правам и показаны в sidebar. Ключ пункта берётся из
`NavigationItem.Key`, затем из `Href`, затем из позиции; повторы получают
суффикс, поэтому расширение всегда может адресовать пункт.

Клиент читает контекст через `readShellContext` и создаёт один `ShellRegistry` на
документ (`documentShellRegistry`). Страницы Templ/HTMX и Solid-маршруты внутри
того же `Authenticated` видят один реестр. Страница без контекста (например,
логин) получает пустой реестр. Несовместимая major-версия контекста — ошибка.

## Обоснование и последствия

Единственная точка публикации — общий layout, поэтому граница одинакова для
legacy и Solid страниц, а `clienthost.WithShell` не нужен для старых страниц.
Права проверяет сервер; контекст не расширяет доступ. Иконки в контекст не
входят: они остаются Templ-компонентами sidebar.

## Проверка

`go test ./pkg/shellcontext/...` проверяет ключи, родителей, workspace и
экранирование JSON. `pnpm --dir web/client-host test` проверяет чтение, версию и
общий реестр документа.

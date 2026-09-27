---
id: solid-features-use-go-import-and-generated-catalog
title: Solid-фичи объявляются Go-импортом и собираются через generated catalog
status: accepted
date: 2026-09-15
deciders: [diyorkhaydarov]
area: platform
applies_to:
  - pkg/clienthost/solid/**
  - pkg/clienthost/**
  - cmd/iota-solid/**
  - internal/solidgen/**
  - web/client-host/src/solid.ts
tags: [solidjs, codegen, client-host, developer-experience]
refs:
  - https://github.com/iota-uz/iota-sdk/issues/947
  - https://github.com/iota-uz/iota-sdk/issues/949
  - https://github.com/iota-uz/iota-sdk/pull/1077
supersedes: []
superseded_by: []
---

## Решение

Автор объявляет целый Solid-экран из owning Go package через
`solid.Import[Props]("./Screen.tsx", ...)` и передаёт initial props через
`Screen.Render(props)`. Query/action contracts находятся в той же декларации.
Фича использует Solid целиком либо Templ + HTMX целиком; общий server shell не
считается смешиванием renderer. `clienthost.NewController` отклоняет наборы
маршрутов со смешанными client/React renderer; граница между Solid и отдельным
Templ-контроллером остаётся архитектурным контрактом code review.

`iota-solid` статически обнаруживает импорты, выпускает committed colocated
bindings и один generated lazy catalog. Generated outputs детерминированы,
имеют ownership marker и проверяются `-check`. Watch-режим выполняет ту же
reconciliation при add/move/delete в dev. Неподдерживаемые wire-типы,
отсутствующий source, duplicate feature/RPC и несовместимый default component
останавливают build или typecheck.

SDK host владеет route loading, безопасной bundle error, Retry, mount и
idempotent dispose. Feature владеет состояниями внутри смонтированного экрана.
Runtime не запускает generator и не требует Node.

## Обоснование и последствия

TSX может находиться рядом с Go-контроллером без ручного TypeScript registry,
mount-функции или feature ID. Go остаётся источником wire-контракта, а PR явно
показывает generated diff. Второй экран не требует менять центральный frontend
entrypoint или Vite route table.

## Проверка

`go test ./internal/solidgen ./pkg/clienthost/...` проверяет discovery,
reconciliation, wire safety и lifecycle. `pnpm --dir web/client-host test`
проверяет loading, bundle failure/Retry, duplicate ownership и late load after
dispose. Consumer выполняет `iota-solid -check`, TypeScript typecheck и build.

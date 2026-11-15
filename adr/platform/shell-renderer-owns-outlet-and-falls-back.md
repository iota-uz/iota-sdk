---
id: shell-renderer-owns-outlet-and-falls-back
title: Один renderer оболочки получает существующий outlet и откатывается на стандартную оболочку
status: accepted
date: 2026-11-10
deciders: [moknelis]
area: platform
applies_to:
  - web/client-host/src/shell-host.ts
  - web/client-host/src/portal-host.ts
  - web/lens/src/runtime/drawer.tsx
  - modules/core/presentation/templates/layouts/authenticated.templ
tags: [shell, extensions, solidjs, portals]
refs:
  - https://github.com/iota-uz/iota-sdk/issues/953
supersedes: []
superseded_by: []
---

## Решение

`layouts.Authenticated` помечает стандартную оболочку (`data-iota-shell-standard`)
и контейнер содержимого страницы (`data-iota-shell-outlet`). `ShellHost`
активирует не более одного app-wide renderer: прячет стандартную оболочку,
переносит тот же DOM-элемент outlet (не копию) в корень renderer и передаёт ему
реестр навигации, theme, locale, пользователя, действие выхода и подписку на
`notificationsChanged`. Так HTMX-подмены, скрипты и модальные окна страницы
продолжают работать, а Solid-маршруты рендерятся внутри того же outlet.

Ошибка `mount`, синхронный или асинхронный `ctx.fail`, сбой cleanup возвращают
outlet на прежнее место и показывают стандартную оболочку; причина записывается
в `diagnostics()`. Второй renderer отклоняется диагностикой `renderer-conflict`.
Параметр `?iota-shell=standard` вручную возвращает стандартную оболочку без
отдельного rescue-фреймворка. Bootstrap, сессия, CSRF и права backend от
компоновки renderer не зависят.

Порталы документа обслуживает `documentPortalRegistry`: корни по поверхностям,
scroll lock, inert фона и возврат фокуса принадлежат хосту. Эталонный
потребитель — `LensDrawer`: он больше не блокирует прокрутку и не восстанавливает
фокус сам, а сохраняет свой слой (`--lens-drawer-z`), focus trap и Escape.

## Обоснование и последствия

Перенос того же элемента сохраняет владение и обработчики страницы. Откат прост,
потому что стандартная оболочка не удаляется. Остальные оверлеи Lens
(`useOverlayContainer`) мигрируют на тот же реестр отдельными изменениями.

## Проверка

`pnpm --dir web/client-host test` проверяет перенос и возврат outlet, конфликт,
откат при ошибке, принудительную стандартную оболочку и уведомления.
`pnpm --dir web/lens test` проверяет, что drawer живёт в корне хоста и снимает
блокировку и inert при закрытии.

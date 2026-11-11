# Architecture decisions

## Platform

- **UI расширений читает один снимок активации, страницы и замены отдаёт один контроллер** — [запись](adr/platform/extension-ui-reads-one-activated-snapshot.md) · accepted · 2026-11-11 (#416, #953)
- **Один renderer оболочки получает существующий outlet и откатывается на стандартную оболочку** — [запись](adr/platform/shell-renderer-owns-outlet-and-falls-back.md) · accepted · 2026-11-10 (#953)

- **Базовая навигация оболочки публикуется layouts.Authenticated и читается одним реестром документа** — [запись](adr/platform/shell-context-is-published-by-authenticated-layout.md) · accepted · 2026-11-10 (#953)

- **Навигация и регионы оболочки описываются реестром с владельцами и диагностикой** — [запись](adr/platform/shell-registry-navigation-and-region-contributions.md) · accepted · 2026-11-10 (#953)

- **Проценты в Excel округляются до двух знаков в процентных пунктах** — [запись](adr/platform/excel-percent-points-rounding.md) · accepted · 2026-10-29

- **Контекст фильтров вычисляется до исполнения отчёта** — [запись](adr/platform/scoped-report-filter-context.md) · accepted · 2026-10-13 (#1057)

- **Отчёт Lens отделён от размещения и результата исполнения** — [запись](adr/platform/reusable-lens-reports.md) · accepted · 2026-10-12 (#1056)

- **В выгрузках формула — только явный тип ячейки, строка всегда данные** — [запись](adr/platform/spreadsheet-export-formula-cells.md) · accepted · 2026-09-19
- **Проверенный релиз SDK по запросу потребителя** — [запись](adr/platform/verified-sdk-releases.md) · superseded · 2026-09-06
- **Локальное возобновляемое продвижение SDK** — [запись](adr/platform/local-sdk-promotion.md) · accepted · 2026-09-06

## Core

- **Проекции вариантов назначения в формах пользователя** — [запись](adr/core/user-form-assignment-options.md) · superseded · 2026-09-11
- **Проекции назначений в существующем query-слое** — [запись](adr/core/assignment-query-projections.md) · accepted · 2026-09-11
- **Онбординг пользователя через временный пароль** — [запись](adr/core/user-onboarding-temporary-password.md) · accepted · 2026-09-29
- **Каналы доставки уведомлений и состояние по каждому каналу** — [запись](adr/core/notification-delivery-channels.md) · accepted · 2026-11-09
- **Получатель, субъект и канальная цель уведомления** — [запись](adr/core/notification-recipients-targets.md) · accepted · 2026-11-11

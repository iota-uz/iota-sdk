# Architecture decisions

## Platform

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

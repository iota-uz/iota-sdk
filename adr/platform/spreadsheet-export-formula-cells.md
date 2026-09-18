---
id: spreadsheet-export-formula-cells
title: В выгрузках формула — только явный тип ячейки, строка всегда данные
status: accepted
date: 2026-09-19
deciders: [diyorkhaydarov]
area: platform
applies_to:
  - pkg/excel/**
  - modules/crm/presentation/controllers/client_controller.go
tags: [export, csv, xlsx, security]
refs: []
supersedes: []
superseded_by: []
---

## Контекст

Потребители SDK собирали CSV вручную через `encoding/csv` и писали
пользовательский текст как есть. Ячейка, начинающаяся с `=`, `+`, `-` или `@`,
открывается в Excel, LibreOffice и Google Sheets как формула (CSV/formula
injection, OWASP): через неё можно вставить `HYPERLINK` на внешний адрес или
DDE-команду. При этом часть выгрузок (дашборды lens, аудиторские листы) формулы
пишет намеренно — в XLSX и явным типом ячейки (`lens/frame.Formula`,
`excelize.SetCellFormula`).

## Решение

- Правило одно для всех выгрузок: обычное значение — всегда данные; формулой
  ячейка становится только при явном типе `excel.Formula`.
- CSV пишется одной реализацией `pkg/excel` — `CSVExporter` для `DataSource` и
  `CSVWriter` для потоковых выгрузок. Строки проходят `NeutralizeFormula`:
  апостроф перед `=`, `@`, табуляцией, CR и перед `+`/`-`, за которыми идёт не
  число. Числа, форматированные суммы (`-1 234,56`) и телефоны
  (`+998 90 123 45 67`) не меняются.
- `ExcelExporter` (оба пути) пишет `excel.Formula` как формулу; строки и так
  хранятся текстом, их не трогаем.

## Отклонённые альтернативы

- **Экранировать каждую ячейку, начинающуюся с `+`/`-`.** Отклонено: телефоны
  и отрицательные суммы открывались бы с видимым апострофом.
- **Отдельный помощник в каждом приложении.** Отклонено: правило должно быть
  одно, иначе новые выгрузки его обходят.

## Последствия

- Намеренные формулы в XLSX (lens, `SetCellFormula`) не меняются; CSV формулы
  отдаёт только через `excel.Formula`.
- Выгрузка клиентов CRM в SDK теперь с UTF-8 BOM (кириллица в Excel под
  Windows открывается корректно).

## Как проверить

`go test ./pkg/excel/` — `TestNeutralizeFormula`, `TestCSVExporter_ExportToWriter`,
`TestExcelExporter_OnlyFormulaTypeIsAFormula` (строка `=1+2` хранится текстом в
обоих путях XLSX, `excel.Formula` — формулой).

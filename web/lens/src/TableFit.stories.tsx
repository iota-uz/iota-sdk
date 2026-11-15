import type { DashboardDocument, Frame, Panel } from './contract'
import { TablePanel } from './panels'
import { DashboardRuntimeProvider, DocumentProvider } from './runtime'
import './styles.css'

// The record table of an explorer level, as a finance dashboard draws it: a
// date, three text columns (one of them empty in most rows), an amount and the
// row action. Its container is what the story varies — a wide drawer, half a
// dashboard, a phone and a full-width card — because the table has to fit the
// space it is given before it is allowed to scroll.
const panel: Panel = {
  id: 'rows', kind: 'table', title: 'Исходные записи', semantics: 'evidence', frame: 'panel:rows', deferred: true,
  encoding: { id: 'id', label: 'label', value: 'value' },
  format: { value: { kind: 'money', currency: 'UZS', minorUnits: false, precision: 0 } },
  columns: [
    { field: 'day', label: 'Дата', cell: { kind: 'plain' } },
    { field: 'label', label: 'Категория', cell: { kind: 'plain' } },
    { field: 'acc_label', label: 'Счёт', cell: { kind: 'plain' } },
    { field: 'note', label: 'Комментарий', cell: { kind: 'plain' } },
    { field: 'value', label: 'Сумма', align: 'right', cell: { kind: 'plain' }, total: true },
  ],
  actions: [{
    kind: 'navigate_to_leaf', urlTemplate: '/records/{id}',
    params: [{ name: 'id', source: { kind: 'field', name: 'id' } }], payload: {}, preserveQuery: true,
  }],
  terminal: true,
}

const data: Frame = {
  columns: [
    { name: 'id', type: 'string' }, { name: 'day', type: 'string' }, { name: 'label', type: 'string' },
    { name: 'acc_label', type: 'string' }, { name: 'note', type: 'string' }, { name: 'value', type: 'number' },
  ],
  rows: [
    ['1', '03.11.2026', 'Аренда офиса', 'Основной счёт', '', 48_000_000],
    ['2', '05.11.2026', 'Зарплата', 'Касса', '', 126_500_000],
    ['3', '07.11.2026', 'Маркетинг', 'Основной счёт', 'Реклама в сети', 18_250_000],
    ['4', '10.11.2026', 'Налоги и сборы', 'Расчётный счёт', '', 31_900_000],
    ['5', '12.11.2026', 'Транспорт', 'Касса', '', 4_700_000],
    ['6', '14.11.2026', 'Подписки на сервисы', 'Основной счёт', 'Лицензии', 9_300_000],
  ],
}

const document_: DashboardDocument = {
  version: '1.0.0', snapshotId: 'table-fit',
  meta: { dashboardId: 'table-fit', title: 'Расходы', generatedAt: '2026-11-15T00:00:00Z', locale: 'ru' },
  layout: { rows: [{ panels: [{ panelId: panel.id, span: 12 }] }] },
  panels: [panel], frames: {},
  drill: { inlineDepth: 0, edges: {} }, perspectives: [], endpoints: { panel: '/story/panel', export: '/story/export' },
  i18n: {}, theme: { palette: {}, series: {} },
}

const fetcher: typeof fetch = () => Promise.resolve(new Response(`${JSON.stringify({
  panelId: 'rows',
  result: {
    frames: { 'panel:rows': data },
    calculation: { durationMs: 12, cacheHit: true, calculatedAt: '2026-11-15T00:00:00Z' },
    summary: { filteredRows: 6, totalRows: 6, values: { value: 238_650_000 } },
  },
})}\n${JSON.stringify({ complete: true })}\n`, { status: 200, headers: { 'Content-Type': 'application/x-ndjson' } }))

function Table() {
  return (
    <DocumentProvider initialDocument={document_}>
      <DashboardRuntimeProvider fetcher={fetcher} locale="ru">{() => (<TablePanel panel={panel} />)}</DashboardRuntimeProvider>
    </DocumentProvider>
  )
}

/** The wide drawer on a 1280px screen: 768px of slab, 694px of table container. */
export const Drawer1280 = () => (
  <div class="lens-root lens-story-shell lens-drawer lens-drawer-wide" data-testid="table-fit" style={{ height: 'auto', width: '768px' }}>
    <div class="lens-drawer-document"><Table /></div>
  </div>
)

/** A half-width dashboard panel on the same screen. */
export const HalfWidthPanel = () => (
  <div class="lens-root lens-story-shell" data-testid="table-fit" style={{ width: '620px' }}><Table /></div>
)

/** A phone: 390px of screen, 16px gutters. */
export const Phone390 = () => (
  <div class="lens-root lens-story-shell" data-testid="table-fit" style={{ width: '358px' }}><Table /></div>
)

export const FullWidth = () => (
  <div class="lens-root lens-story-shell" data-testid="table-fit" style={{ width: '1200px' }}><Table /></div>
)

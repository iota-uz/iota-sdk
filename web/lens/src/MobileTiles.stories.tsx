import type { DashboardDocument, Frame, Panel } from './contract'
import { DashboardPanels } from './DashboardPanels'
import { DashboardRuntimeProvider, DocumentProvider } from './runtime'
import './styles.css'

/**
 * A board at phone width (360px), the way the ERP dashboard reaches a phone.
 *
 * The document ships an empty `i18n` catalogue on purpose: every word of
 * chrome — «Итого», «Скрыть все», «Развернуть панель» — must come from the
 * runtime's own Russian, not from a host that forgot to send it. The money
 * formats carry the host's grapheme, so tiles and charts must agree on «so’m».
 */

const exactMoney = { kind: 'money', currency: 'UZS', symbol: 'so’m', minorUnits: false, precision: 0 } as const
const compactMoney = {
  kind: 'money', currency: 'UZS', symbol: 'so’m', minorUnits: false, precision: 2, compact: true, decimalSeparator: ',',
} as const

function tile(id: string, title: string): Panel {
  return {
    id, kind: 'stat', title, semantics: 'series', frame: `${id}:frame`,
    encoding: { label: 'label', value: 'value' },
    format: { value: exactMoney },
    terminal: false,
    actions: [{ kind: 'navigate', urlTemplate: `/finance/${id}`, params: [], payload: {} }],
  }
}

function tileFrame(title: string, value: number): Frame {
  return {
    columns: [{ name: 'label', type: 'string' }, { name: 'value', type: 'number' }],
    rows: [[title, value]],
  }
}

const tiles: Array<[string, string, number]> = [
  ['balance', 'Остаток на счетах', 1_106_129_860],
  ['income', 'Пришло за месяц', 14_975_780],
  ['spent', 'Ушло за месяц', 27_150_001],
  ['operations', 'Операций за месяц', 4],
]

const expensesPanel: Panel = {
  id: 'expenses', kind: 'pie', title: 'На что ушли деньги за год', semantics: 'partition', frame: 'expenses:frame',
  encoding: { label: 'label', value: 'amount', id: 'id' },
  format: { amount: compactMoney },
  total: 691_180_000,
  presentation: { legend: 'below', sliceLabels: 'percent', totalBadge: 'header' },
  terminal: true,
  actions: [],
}

const expenses: Array<[string, number]> = [
  ['Канцелярия', 124_900_000], ['Транспорт', 81_500_000], ['Облачные сервисы', 72_380_000],
  ['Представительские расходы', 69_900_000], ['Аренда офиса', 63_850_000], ['Оборудование', 59_000_000],
  ['Маркетинг', 54_650_000], ['Обучение сотрудников', 51_550_000], ['Налоги', 46_200_000],
  ['Банковские комиссии', 35_800_000], ['Связь и интернет', 18_850_000], ['Подписки на ПО', 6_900_000],
]

const balancesPanel: Panel = {
  id: 'balances', kind: 'hbar', title: 'Остатки по счетам', semantics: 'series', frame: 'balances:frame',
  encoding: { category: 'account', value: 'amount' },
  format: { amount: compactMoney },
  terminal: true,
  actions: [],
}

const balances: Array<[string, number]> = [
  ['Расчётный счёт Kapitalbank', 480_000_000],
  ['Валютный счёт USD', 310_000_000],
  ['Payoneer', 62_000_000],
  ['Корпоративная карта Visa', -4_000_000],
  ['Наличные касса', -38_000_000],
]

function phoneDocument(): DashboardDocument {
  const panels = [...tiles.map(([id, title]) => tile(id, title)), expensesPanel, balancesPanel]
  return {
    version: '1.0.0', snapshotId: 'mobile-snapshot',
    meta: { dashboardId: 'mobile', title: 'Финансовый обзор', generatedAt: '2026-07-19T00:00:00Z', locale: 'ru' },
    layout: {
      rows: [
        { panels: tiles.map(([id]) => ({ panelId: id, span: 3 })) },
        { panels: [{ panelId: 'expenses', span: 6 }, { panelId: 'balances', span: 6 }] },
      ],
    },
    panels,
    frames: {
      ...Object.fromEntries(tiles.map(([id, title, value]) => [`${id}:frame`, tileFrame(title, value)])),
      'expenses:frame': {
        columns: [{ name: 'id', type: 'string' }, { name: 'label', type: 'string' }, { name: 'amount', type: 'number' }],
        rows: expenses.map(([label, amount], index) => [`e${index}`, label, amount]),
      },
      'balances:frame': {
        columns: [{ name: 'account', type: 'string' }, { name: 'amount', type: 'number' }],
        rows: balances,
      },
    },
    drill: { inlineDepth: 0, edges: {} },
    perspectives: [], endpoints: { export: '/lens/export' }, i18n: {},
    theme: { palette: {}, series: {} },
  }
}

export const PhoneBoard = () => (
  <div class="lens-root" data-testid="phone-board" style={{ width: '360px' }}>
    <DocumentProvider initialDocument={phoneDocument()}>
      <DashboardRuntimeProvider locale="ru">{() => <DashboardPanels />}</DashboardRuntimeProvider>
    </DocumentProvider>
  </div>
)
PhoneBoard.storyName = 'Phone board'

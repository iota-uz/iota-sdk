import type { Story } from '@ladle/react'
import type { DashboardDocument, Frame, Panel } from './contract'
import { DashboardPanels } from './DashboardPanels'
import { CoveragePanel, DashboardSkeleton, PanelSkeletonBody, TablePanel } from './panels'
import { DashboardRuntimeProvider, DocumentProvider } from './runtime'
import './styles.css'

/**
 * Stories for the density treatments a dashboard opts into through the
 * contract: metric groups, tab groups, coverage panels, compact table cells
 * and pie panels with an in-plot total badge and a legend below.
 */

const money = { kind: 'money', currency: 'UZS', minorUnits: false, precision: 0 } as const
const compactMoney = {
  kind: 'money', currency: 'UZS', minorUnits: false, precision: 2, compact: true, decimalSeparator: '.',
} as const
const compactNumber = {
  kind: 'number', minorUnits: false, precision: 2, compact: true, decimalSeparator: '.',
} as const

function statPanel(id: string, title: string, accent: string, status?: Panel['status']): Panel {
  return {
    id, kind: 'stat', title, semantics: 'series', frame: `${id}:frame`,
    encoding: { label: 'label', value: 'value' },
    format: { value: { kind: 'percent', minorUnits: false, precision: 1, decimalSeparator: '.' } },
    accent, status, actions: [],
  }
}

function statFrame(title: string, value: number): Frame {
  return {
    columns: [{ name: 'label', type: 'string' }, { name: 'value', type: 'number' }],
    rows: [[title, value]],
  }
}

const metrics: Array<{ panel: Panel; value: number }> = [
  { panel: statPanel('loss', 'Коэффициент убыточности', '#2f56d9', { label: 'ОЦЕНКА', tone: 'warning' }), value: 3.1 },
  { panel: statPanel('expense', 'Коэффициент расходов', '#d97824'), value: 41.7 },
  { panel: statPanel('combined', 'Комбинированный коэффициент', '#7c3aed'), value: 44.8 },
  { panel: statPanel('retention', 'Собственное удержание', '#059669'), value: 92.4 },
]

const coveragePanel: Panel = {
  id: 'payouts', kind: 'coverage', title: 'Выплаты по убыткам', semantics: 'partition', frame: 'payouts:frame',
  encoding: { label: 'label', value: 'amount' },
  format: { amount: money },
  caption: 'ВСЕ ВЫПЛАТЫ ПОКРЫТЫ РЕЗЕРВОМ',
  headline: 5_458_561_140,
  actions: [],
}

const coverageFrame: Frame = {
  columns: [{ name: 'label', type: 'string' }, { name: 'amount', type: 'number' }],
  rows: [['В пределах резерва', 5_458_561_140], ['Сверх резерва', 0]],
}

const underwritingPanel: Panel = {
  ...coveragePanel, id: 'payouts-uw', title: 'Андеррайтинговый результат',
  caption: 'РЕЗЕРВ ПОКРЫВАЕТ ЗАЯВЛЕННЫЕ УБЫТКИ',
}

const groupsPanel: Panel = {
  id: 'groups', kind: 'table', title: 'Группы А/Б/В', semantics: 'evidence', frame: 'groups:frame',
  encoding: { id: 'group_id', label: 'name' },
  format: { earned: compactNumber, balance: compactMoney, delta: compactNumber, delta_pct: { kind: 'percent', minorUnits: false, precision: 1, decimalSeparator: '.' } },
  columns: [
    { field: 'name', label: 'Продукт', cell: { kind: 'plain' }, clamp: 2, widthPx: 200 },
    {
      field: 'earned', label: 'Заработанная премия', align: 'right', cell: { kind: 'plain' }, affordance: 'pill',
      action: { kind: 'navigate_to_leaf', urlSource: { kind: 'field', name: 'detail_url' }, params: [], payload: {} },
    },
    { field: 'balance', label: 'Остаток', align: 'right', cell: { kind: 'underline' } },
    { field: 'delta', label: 'К прошлому', align: 'right', cell: { kind: 'delta', secondaryField: 'delta_pct', layout: 'stacked' } },
  ],
  actions: [],
}

const groupsFrame: Frame = {
  columns: [
    { name: 'group_id', type: 'string' }, { name: 'name', type: 'string' },
    { name: 'earned', type: 'number' }, { name: 'balance', type: 'number' },
    { name: 'delta', type: 'number' }, { name: 'delta_pct', type: 'number' },
    { name: 'detail_url', type: 'string' },
  ],
  rows: [
    ['a', 'Обязательное страхование гражданской ответственности владельцев транспортных средств', 9_364_442_607, 150_530_000, -12_030_000, -0.6, '/groups/a'],
    ['b', 'Добровольное страхование имущества', 6_070_000_000, -930_000_000, 13_400_000, 13, '/groups/b'],
    ['c', 'Страхование от несчастных случаев', 3_870_000_000, 47_100_000, 900_000, 2.4, '/groups/c'],
  ],
}

function storyDocument(panels: Panel[], frames: Record<string, Frame>, layout: DashboardDocument['layout']): DashboardDocument {
  return {
    version: '1.0.0', snapshotId: 'parity-snapshot',
    meta: { dashboardId: 'parity', title: '', generatedAt: '2026-07-19T00:00:00Z', locale: 'ru' },
    layout, panels, frames,
    drill: { inlineDepth: 0, edges: {} },
    perspectives: [], endpoints: {}, i18n: {},
    theme: { palette: { blue: '#2f56d9', orange: '#d97824', purple: '#7c3aed' }, series: { 'payouts:0': '#2f56d9', 'payouts:1': '#d97824' } },
  }
}

function Runtime({ children, doc }: { children: React.ReactNode; doc: DashboardDocument }) {
  return (
    <div className="lens-root">
      <DocumentProvider initialDocument={doc}>
        <DashboardRuntimeProvider locale="ru">{children}</DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  )
}

export const MetricGroup: Story = () => {
  const frames = Object.fromEntries(metrics.map(({ panel, value }) => [`${panel.id}:frame`, statFrame(panel.title, value)]))
  const doc = storyDocument(
    metrics.map(({ panel }) => panel),
    frames,
    {
      rows: [{
        heading: 'КЛЮЧЕВЫЕ КОЭФФИЦИЕНТЫ',
        panels: metrics.map(({ panel }) => ({
          panelId: panel.id, span: 3,
          group: { id: 'earned', kind: 'metrics' as const, label: 'ПО ЗАРАБОТАННОЙ ПРЕМИИ', layout: 'columns' as const, span: 12 },
        })),
      }],
    },
  )
  return <Runtime doc={doc}><DashboardPanels /></Runtime>
}

export const TabGroup: Story = () => {
  const doc = storyDocument(
    [coveragePanel, underwritingPanel, groupsPanel],
    { 'payouts:frame': coverageFrame, 'groups:frame': groupsFrame },
    {
      rows: [{
        heading: 'ФОРМИРОВАНИЕ РЕЗУЛЬТАТА',
        panels: [
          { panelId: 'payouts', span: 4, group: { id: 'result', kind: 'tabs' as const, span: 12, tab: 'Денежный результат' } },
          { panelId: 'groups', span: 8, group: { id: 'result', kind: 'tabs' as const, span: 12, tab: 'Денежный результат' } },
          { panelId: 'payouts-uw', span: 12, group: { id: 'result', kind: 'tabs' as const, span: 12, tab: 'Андеррайтинговый результат' } },
        ],
      }],
    },
  )
  return <Runtime doc={doc}><DashboardPanels /></Runtime>
}

export const CoverageComposite: Story = () => {
  const doc = storyDocument([coveragePanel], { 'payouts:frame': coverageFrame }, {
    rows: [{ panels: [{ panelId: 'payouts', span: 12 }] }],
  })
  return <Runtime doc={doc}><CoveragePanel panel={coveragePanel} /></Runtime>
}

export const CompactTableCells: Story = () => {
  const doc = storyDocument([groupsPanel], { 'groups:frame': groupsFrame }, {
    rows: [{ panels: [{ panelId: 'groups', span: 12 }] }],
  })
  return <Runtime doc={doc}><TablePanel panel={groupsPanel} /></Runtime>
}

const premiumPanel: Panel = {
  id: 'premium', kind: 'pie', title: 'Брутто подписанная премия', semantics: 'partition', frame: 'premium:frame',
  encoding: { label: 'label', value: 'amount', id: 'id' },
  format: { amount: money },
  total: 118_800_000_000,
  presentation: { legend: 'below', sliceLabels: 'percent', totalBadge: 'plot', fill: true },
  actions: [],
}

const premiumFrame: Frame = {
  columns: [{ name: 'id', type: 'string' }, { name: 'label', type: 'string' }, { name: 'amount', type: 'number' }],
  rows: [
    ['earned', 'Заработанная премия', 105_814_921_823],
    ['unearned', 'Незаработанная премия', 12_985_078_177],
  ],
}

export const PieWithLegendBelow: Story = () => {
  const doc = storyDocument([premiumPanel], { 'premium:frame': premiumFrame }, {
    rows: [{ heading: 'ПРЕМИИ', panels: [{ panelId: 'premium', span: 6 }] }],
  })
  return <Runtime doc={doc}><DashboardPanels /></Runtime>
}

const skeletonRows = [
  { items: [{ span: 3, kind: 'stat' as const }, { span: 3, kind: 'stat' as const }, { span: 3, kind: 'stat' as const }, { span: 3, kind: 'stat' as const }] },
  { heading: true, items: [{ span: 6, kind: 'pie' as const }, { span: 6, kind: 'coverage' as const }] },
  { heading: true, items: [{ span: 12, kind: 'table' as const }] },
]

function SkeletonStory({ theme }: { theme: 'light' | 'dark' }) {
  return (
    <div className="lens-root" data-theme={theme}>
      <DashboardSkeleton rows={skeletonRows} />
    </div>
  )
}

export const DashboardLoadingSkeletonLight: Story = () => <SkeletonStory theme="light" />
export const DashboardLoadingSkeletonDark: Story = () => <SkeletonStory theme="dark" />

function PanelSkeletonStory({ theme }: { theme: 'light' | 'dark' }) {
  const shapes: Array<{ title: string; kind: 'pie' | 'coverage' | 'table' }> = [
    { title: 'БРУТТО ПОДПИСАННАЯ ПРЕМИЯ', kind: 'pie' },
    { title: 'ВЫПЛАТЫ ПО УБЫТКАМ', kind: 'coverage' },
    { title: 'ГРУППЫ А/Б/В', kind: 'table' },
  ]
  return (
    <div className="lens-root" data-theme={theme}>
      <div className="lens-panel-grid">
        {shapes.map(({ title, kind }) => (
          <div className="lens-grid-item" key={kind} style={{ '--lens-panel-span': 4 } as React.CSSProperties}>
            <section aria-busy="true" className="lens-panel">
              <header className="lens-panel-header"><h3 className="lens-panel-title">{title}</h3></header>
              <div className="lens-panel-body"><PanelSkeletonBody kind={kind} /></div>
            </section>
          </div>
        ))}
      </div>
    </div>
  )
}

export const PanelSkeletonsLight: Story = () => <PanelSkeletonStory theme="light" />
export const PanelSkeletonsDark: Story = () => <PanelSkeletonStory theme="dark" />

const drillPillPanel: Panel = {
  ...groupsPanel,
  id: 'drill-pills',
  title: 'Аффордансы drill-ячеек',
  columns: [
    { field: 'name', label: 'Продукт', cell: { kind: 'plain' }, clamp: 2, widthPx: 200 },
    {
      field: 'earned', label: 'С действием', align: 'right', cell: { kind: 'plain' }, affordance: 'pill',
      action: { kind: 'navigate_to_leaf', urlSource: { kind: 'field', name: 'detail_url' }, params: [], payload: {} },
    },
    // No wire action: the host renderer owns this drill, so the pill appears
    // without an arrow rather than promising navigation.
    { field: 'delta', label: 'Без действия', align: 'right', cell: { kind: 'plain' }, affordance: 'pill' },
  ],
}

export const DrillPillAffordances: Story = () => {
  const doc = storyDocument([drillPillPanel], { 'groups:frame': groupsFrame }, {
    rows: [{ panels: [{ panelId: 'drill-pills', span: 12 }] }],
  })
  return <Runtime doc={doc}><TablePanel panel={drillPillPanel} /></Runtime>
}

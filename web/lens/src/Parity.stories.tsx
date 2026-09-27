import type { Story } from '@ladle/react'
import { useEffect, useRef } from 'react'
import type { DashboardDocument, Frame, Panel } from './contract'
import { DashboardPanels } from './DashboardPanels'
import {
  ArrowClockwise, ArrowsIn, ArrowsLeftRight, ArrowsOut, ArrowUpRight, CaretDown, CaretLeft, CaretRight,
  CircleNotch, DownloadSimple, X,
} from './icons'
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
    accent, status, terminal: true, actions: [],
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
  caption: 'ВСЕ ВЫПЛАТЫ ПОКРЫТЫ РЕЗЕРВОМ\nПРЕДВАРИТЕЛЬНЫЕ ДАННЫЕ',
  headline: 5_458_561_140,
  terminal: true,
  actions: [],
}

const coverageFrame: Frame = {
  columns: [{ name: 'label', type: 'string' }, { name: 'amount', type: 'number' }],
  rows: [['В пределах резерва', 5_458_561_140], ['Сверх резерва', 0]],
}

const coverageSplitPanel: Panel = {
  ...coveragePanel, id: 'payouts-split', frame: 'payouts-split:frame',
  caption: 'ЧАСТЬ ВЫПЛАТ ВЫШЛА ЗА РЕЗЕРВ',
  headline: 6_913_561_140,
}

const coverageSplitFrame: Frame = {
  columns: [{ name: 'label', type: 'string' }, { name: 'amount', type: 'number' }],
  rows: [['В пределах резерва', 5_458_561_140], ['Сверх резерва', 1_455_000_000]],
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
    // `trend:*` pins deliberately run against the palette's own order: the
    // plot must take its colours from the panel's pins like the legend does,
    // so a line drawn in palette order instead shows up as a mismatch here.
    theme: {
      palette: { blue: '#2f56d9', orange: '#d97824', purple: '#7c3aed' },
      series: { 'payouts:0': '#2f56d9', 'payouts:1': '#d97824', 'trend:0': 'purple', 'trend:1': 'orange' },
    },
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
  const contextualMetrics = metrics.map(({ panel, value }, index) => ({
    panel: {
      ...panel,
      caption: [
        'Claims paid ÷ earned premium',
        'Expenses ÷ earned premium',
        '(claims + expenses) ÷ earned premium',
        'Earned premium − claims − expenses ± reserves',
      ][index],
    },
    value,
  }))
  const frames = Object.fromEntries(contextualMetrics.map(({ panel, value }) => [`${panel.id}:frame`, statFrame(panel.title, value)]))
  const doc = storyDocument(
    contextualMetrics.map(({ panel }) => panel),
    frames,
    {
      rows: [{
        heading: 'КЛЮЧЕВЫЕ КОЭФФИЦИЕНТЫ',
        panels: contextualMetrics.map(({ panel }) => ({
          panelId: panel.id, span: 3,
          groups: [{
            id: 'earned', kind: 'metrics' as const, label: 'ПО ЗАРАБОТАННОЙ ПРЕМИИ',
            // The group caption names the basis for the whole strip — the one
            // sentence that would otherwise be repeated on every metric card.
            caption: 'База — заработанная премия периода.',
            layout: 'columns' as const, span: 12,
          }],
        })),
      }],
    },
  )
  return <Runtime doc={doc}><DashboardPanels /></Runtime>
}

const responsiveMetrics = metrics.map(({ panel, value }, index) => ({
  panel: index === 0
    ? {
      ...panel,
      terminal: false,
      actions: [{ kind: 'navigate' as const, urlTemplate: '/analytics/drill/loss-ratio', params: [], payload: {} }],
    }
    : panel,
  value,
}))

export const MetricGroupResponsive: Story = () => {
  const frames = Object.fromEntries(responsiveMetrics.map(({ panel, value }) => [`${panel.id}:frame`, statFrame(panel.title, value)]))
  const doc = storyDocument(
    responsiveMetrics.map(({ panel }) => panel),
    frames,
    {
      rows: [{
        heading: 'АДАПТИВНАЯ KPI-ПОЛОСА',
        panels: responsiveMetrics.map(({ panel }) => ({
          panelId: panel.id,
          span: 3,
          groups: [{ id: 'responsive', kind: 'metrics' as const, layout: 'columns' as const, span: 12 }],
        })),
      }],
    },
  )
  return <Runtime doc={doc}><DashboardPanels /></Runtime>
}
MetricGroupResponsive.storyName = 'Metric group responsive'

/**
 * The compact metric form now carries the same quiet trend line the hero stat
 * card does when the wire supplies `Panel.sparkline`. The first two metrics
 * carry one; the last two do not, so the story also proves a metric without a
 * sparkline stays exactly as it was — the gate keeps non-trend cards identical.
 */
const sparkMetrics: Array<{ panel: Panel; value: number }> = [
  { panel: { ...metrics[0]!.panel, sparkline: { values: [3.6, 3.4, 3.5, 3.2, 3.3, 3.0, 3.1] } }, value: 3.1 },
  { panel: { ...metrics[1]!.panel, sparkline: { values: [38.1, 39.4, 40.2, 40.9, 41.1, 41.4, 41.7] } }, value: 41.7 },
  { panel: metrics[2]!.panel, value: 44.8 },
  { panel: metrics[3]!.panel, value: 92.4 },
]

export const MetricGroupSparkline: Story = () => {
  const frames = Object.fromEntries(sparkMetrics.map(({ panel, value }) => [`${panel.id}:frame`, statFrame(panel.title, value)]))
  const doc = storyDocument(
    sparkMetrics.map(({ panel }) => panel),
    frames,
    {
      rows: [{
        heading: 'КЛЮЧЕВЫЕ КОЭФФИЦИЕНТЫ',
        panels: sparkMetrics.map(({ panel }) => ({
          panelId: panel.id, span: 3,
          groups: [{ id: 'earned', kind: 'metrics' as const, label: 'ПО ЗАРАБОТАННОЙ ПРЕМИИ', layout: 'columns' as const, span: 12 }],
        })),
      }],
    },
  )
  return <Runtime doc={doc}><DashboardPanels /></Runtime>
}
MetricGroupSparkline.storyName = 'Metric group sparkline'

/**
 * The same strip before any of its figures have landed.
 *
 * A KPI strip is the first thing on a board and the last thing to finish
 * computing, so its loading state is what a reader looks at for the whole first
 * second — and it was the one state nothing exercised. The figure slot has no
 * text of its own while the panel is deferred, so a placeholder that borrowed
 * its width from that slot resolved to zero and the strip painted a row of
 * names over an empty gap: indistinguishable, for that whole second, from a
 * board whose numbers had failed to arrive.
 *
 * The row beside the figure is crowded on purpose: the first metric is a link
 * (so it draws its drill mark) and the second carries a sparkline, which are
 * the two things the placeholder shares its flex row with.
 */
const loadingMetrics: Panel[] = metrics.map(({ panel }, index) => ({
  ...panel,
  deferred: true,
  caption: ['Claims paid ÷ earned premium', 'Expenses ÷ earned premium', '(claims + expenses) ÷ earned premium', 'Earned premium − claims − expenses ± reserves'][index],
  sparkline: index === 1 ? { values: [38.1, 39.4, 40.2, 40.9, 41.1, 41.4, 41.7] } : undefined,
  terminal: index !== 0,
  actions: index === 0
    ? [{ kind: 'navigate' as const, urlTemplate: '/analytics/drill/loss-ratio', params: [], payload: {} }]
    : [],
}))

/** A request that never settles: the panels stay in flight for the capture. */
const neverSettles: typeof fetch = () => new Promise<Response>(() => undefined)

export const MetricGroupLoading: Story = () => {
  const doc: DashboardDocument = {
    ...storyDocument(loadingMetrics, {}, {
      rows: [{
        heading: 'КЛЮЧЕВЫЕ КОЭФФИЦИЕНТЫ',
        panels: loadingMetrics.map((panel) => ({
          panelId: panel.id, span: 3,
          groups: [{ id: 'earned', kind: 'metrics' as const, label: 'ПО ЗАРАБОТАННОЙ ПРЕМИИ', layout: 'columns' as const, span: 12 }],
        })),
      }],
    }),
    // Deferred panels are computed per panel after the document lands, which is
    // the shape that puts every cell of the strip in flight at once.
    endpoints: { panel: '/lens/panel' },
  }
  return (
    <div className="lens-root">
      <DocumentProvider fetcher={neverSettles} initialDocument={doc}>
        <DashboardRuntimeProvider fetcher={neverSettles} locale="ru"><DashboardPanels /></DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  )
}
MetricGroupLoading.storyName = 'Metric group loading'

/**
 * A cube dashboard's KPI band is a headerless strip whose metrics carry both a
 * caption and a supporting note, and whose first metric is a link to its own
 * evidence. Three things have to hold at once here, and each of them broke a
 * naive version of the compact ⓘ:
 *
 *  - the glyph is sized for a 10px label, not the 28px header hit box;
 *  - it out-ranks the card-wide navigate anchor stretched over the cell, so
 *    the note can be opened at all on a linked metric;
 *  - its bubble escapes the strip card, which is far shorter than the note.
 *
 * The tip is opened on mount so the bubble itself is the subject.
 */
const notedMetrics: Array<{ panel: Panel; value: number }> = [
  {
    panel: {
      ...metrics[0]!.panel,
      status: undefined,
      caption: 'Проданные полисы',
      info: 'Число уникальных полисов, выпущенных в выбранном периоде. Аннулированные полисы исключены.',
      terminal: false,
      actions: [{
        kind: 'navigate', urlTemplate: '/insurance/sales-report/drill/policies', params: [], payload: {},
      }],
    },
    value: 3.1,
  },
  {
    panel: {
      ...metrics[1]!.panel,
      caption: 'GWP по дате заключения',
      info: 'Подписанная премия по дате заключения договора, без вычета исходящего перестрахования.',
    },
    value: 41.7,
  },
  { panel: { ...metrics[2]!.panel, caption: 'Страховая сумма по действующим полисам' }, value: 44.8 },
]

export const MetricGroupInfo: Story = () => {
  const frames = Object.fromEntries(notedMetrics.map(({ panel, value }) => [`${panel.id}:frame`, statFrame(panel.title, value)]))
  const doc = storyDocument(
    notedMetrics.map(({ panel }) => panel),
    frames,
    {
      rows: [{
        panels: notedMetrics.map(({ panel }) => ({
          panelId: panel.id, span: 4,
          groups: [{ id: 'kpi', kind: 'metrics' as const, label: '', layout: 'columns' as const, span: 12 }],
        })),
      }],
    },
  )
  return (
    <Runtime doc={doc}>
      <OpenFirstInfoTip><DashboardPanels /></OpenFirstInfoTip>
    </Runtime>
  )
}
MetricGroupInfo.storyName = 'Metric group info'

function OpenFirstInfoTip({ children }: { children: React.ReactNode }) {
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => runWhenReady(() => {
    const button = ref.current?.querySelector<HTMLElement>('.lens-info-tip-button-inline')
    if (!button) return false
    button.click()
    return true
  }), [])
  return <div ref={ref}>{children}</div>
}

export const TabGroup: Story = () => {
  const doc = storyDocument(
    [coveragePanel, underwritingPanel, groupsPanel],
    { 'payouts:frame': coverageFrame, 'groups:frame': groupsFrame },
    {
      rows: [{
        heading: 'ФОРМИРОВАНИЕ РЕЗУЛЬТАТА',
        panels: [
          { panelId: 'payouts', span: 4, groups: [{ id: 'result', kind: 'tabs' as const, span: 12, tab: 'Денежный результат' }] },
          { panelId: 'groups', span: 8, groups: [{ id: 'result', kind: 'tabs' as const, span: 12, tab: 'Денежный результат' }] },
          { panelId: 'payouts-uw', span: 12, groups: [{ id: 'result', kind: 'tabs' as const, span: 12, tab: 'Андеррайтинговый результат' }] },
        ],
      }],
    },
  )
  doc.filters = [{
    id: 'business-type',
    kind: 'segmented',
    label: 'Тип бизнеса',
    placement: { groupId: 'result', tab: 'Денежный результат' },
    segmented: {
      param: 'BusinessType',
      value: 'all',
      options: [
        { value: 'all', label: 'Все' },
        { value: 'direct', label: 'Прямое страхование' },
        { value: 'reinsurance', label: 'Перестрахование' },
      ],
    },
  }]
  return <Runtime doc={doc}><DashboardPanels /></Runtime>
}

const logarithmicProductsPanel: Panel = {
  id: 'log-products',
  kind: 'hbar',
  title: 'Продажи по продуктам',
  semantics: 'series',
  frame: 'log-products:frame',
  encoding: { category: 'product', value: 'policies' },
  format: { policies: { kind: 'number', minorUnits: false, precision: 0 } },
  valueAxis: { scale: 'logarithmic', logBase: 10 },
  presentation: { valueSpreadThreshold: 100 },
  terminal: true,
  actions: [],
}

const logarithmicProductsFrame: Frame = {
  columns: [{ name: 'product', type: 'string' }, { name: 'policies', type: 'number' }],
  rows: [
    ['ОСАГО', 18_000],
    ['Страхование имущества', 470],
    ['КАСКО', 36],
    ['Профессиональная ответственность', 2],
  ],
}

export const LogarithmicHorizontalBar: Story = () => {
  const doc = storyDocument([logarithmicProductsPanel], { 'log-products:frame': logarithmicProductsFrame }, {
    rows: [{ heading: 'ПРОДУКТОВЫЙ ПОРТФЕЛЬ', panels: [{ panelId: 'log-products', span: 12 }] }],
  })
  return <Runtime doc={doc}><DashboardPanels /></Runtime>
}
LogarithmicHorizontalBar.storyName = 'Logarithmic horizontal bar'

export const CoverageComposite: Story = () => {
  const doc = storyDocument([coveragePanel], { 'payouts:frame': coverageFrame }, {
    rows: [{ panels: [{ panelId: 'payouts', span: 12 }] }],
  })
  return <Runtime doc={doc}><CoveragePanel panel={coveragePanel} /></Runtime>
}

/**
 * The plain multi-segment track — the shape a partition bar takes when it has
 * no target and more than one non-zero part. CoverageComposite next door has a
 * single positive segment and a zero remainder, so Lens draws no track for it
 * at all: this variant is the only story that exercises the track, its hover
 * emphasis, and the quieting of the segments the pointer is not on.
 */
export const CoverageSplit: Story = () => {
  const doc = storyDocument([coverageSplitPanel], { 'payouts-split:frame': coverageSplitFrame }, {
    rows: [{ panels: [{ panelId: 'payouts-split', span: 12 }] }],
  })
  return <Runtime doc={doc}><CoveragePanel panel={coverageSplitPanel} /></Runtime>
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
  terminal: true,
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
PieWithLegendBelow.storyName = 'Pie with legend right - light'

/** The same right-hand legend layout in the dark theme. */
export const PieWithLegendBelowDark: Story = () => {
  const doc = storyDocument([premiumPanel], { 'premium:frame': premiumFrame }, {
    rows: [{ heading: 'ПРЕМИИ', panels: [{ panelId: 'premium', span: 6 }] }],
  })
  return (
    <div className="lens-root" data-theme="dark">
      <DocumentProvider initialDocument={doc}>
        <DashboardRuntimeProvider locale="ru"><DashboardPanels /></DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  )
}
PieWithLegendBelowDark.storyName = 'Pie with legend right - dark'

/**
 * The same panel with the legend the real dashboard has: enough long Russian
 * labels to fill the right-hand column top to bottom. Two entries centre
 * themselves mid-column and leave the top corner empty, which is why this
 * layout passed its stories for months while the live «Итого» sat on the first
 * legend row. The total must clear the legend at every list length.
 */
const premiumTallFrame: Frame = {
  columns: premiumFrame.columns,
  rows: [
    ['earned', 'Заработанная часть премии, подписанной в периоде', 105_814_921_823],
    ['unearned', 'Незаработанная часть премии, подписанной в периоде', 57_985_078_177],
    ['direct', 'Чистая прямая подписанная премия', 6_900_000_000],
    ['inward', 'Входящее перестрахование', 255_700_000_000],
    ['ceded', 'Исходящее перестрахование, переданное партнёрам', 14_360_000_000],
    ['coinsurance', 'Сострахование', 2_140_000_000],
  ],
}

export const PieWithTallLegend: Story = () => {
  const doc = storyDocument([premiumPanel], { 'premium:frame': premiumTallFrame }, {
    rows: [{ heading: 'ПРЕМИИ', panels: [{ panelId: 'premium', span: 6 }] }],
  })
  return <Runtime doc={doc}><DashboardPanels /></Runtime>
}
PieWithTallLegend.storyName = 'Pie with tall legend'

const trendPanel: Panel = {
  id: 'trend', kind: 'line', title: 'Премия и поступления', semantics: 'series', frame: 'trend:frame',
  encoding: { category: 'period', series: 'metric', value: 'amount' },
  format: { amount: money },
  presentation: { legend: 'below' },
  terminal: true,
  actions: [],
}

const trendFrame: Frame = {
  columns: [
    { name: 'period', type: 'string' },
    { name: 'metric', type: 'string' },
    { name: 'amount', type: 'number' },
  ],
  rows: [
    ['2024', 'Подписанная премия', 72_000_000_000],
    ['2024', 'Заработанная премия', 64_000_000_000],
    ['2025', 'Подписанная премия', 260_000_000_000],
    ['2025', 'Заработанная премия', 218_000_000_000],
    ['2026', 'Подписанная премия', 94_000_000_000],
    ['2026', 'Заработанная премия', 91_000_000_000],
  ],
}

export const LineWithSeriesLegend: Story = () => {
  const doc = storyDocument([trendPanel], { 'trend:frame': trendFrame }, {
    rows: [{ heading: 'ТЕНДЕНЦИИ', panels: [{ panelId: 'trend', span: 12 }] }],
  })
  return <Runtime doc={doc}><DashboardPanels /></Runtime>
}
LineWithSeriesLegend.storyName = 'Line with series legend'

/**
 * Written premium is the sum of its two sources, so the columns stack; earned
 * premium is a different basis and runs over them as a line rather than
 * pretending to be a third source.
 */
const compositionPanel: Panel = {
  id: 'composition', kind: 'bar', title: 'Премия по источникам', semantics: 'series', frame: 'composition:frame',
  encoding: { category: 'period', series: 'metric', value: 'amount' },
  format: { amount: money },
  presentation: { legend: 'below', stack: true, lineSeries: ['Заработанная премия'] },
  terminal: true,
  actions: [],
}

const compositionFrame: Frame = {
  columns: [
    { name: 'period', type: 'string' },
    { name: 'metric', type: 'string' },
    { name: 'amount', type: 'number' },
  ],
  rows: [
    ['2024', 'Прямое страхование', 30_000_000_000],
    ['2024', 'Входящее перестрахование', 42_000_000_000],
    ['2024', 'Заработанная премия', 64_000_000_000],
    ['2025', 'Прямое страхование', 38_000_000_000],
    ['2025', 'Входящее перестрахование', 222_000_000_000],
    ['2025', 'Заработанная премия', 218_000_000_000],
    ['2026', 'Прямое страхование', 21_000_000_000],
    ['2026', 'Входящее перестрахование', 73_000_000_000],
    ['2026', 'Заработанная премия', 91_000_000_000],
  ],
}

export const StackedComposition: Story = () => {
  const doc = storyDocument([compositionPanel], { 'composition:frame': compositionFrame }, {
    rows: [{ heading: 'СОСТАВ ПРЕМИИ', panels: [{ panelId: 'composition', span: 12 }] }],
  })
  return <Runtime doc={doc}><DashboardPanels /></Runtime>
}
StackedComposition.storyName = 'Stacked composition with a line'

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

function runWhenReady(action: () => boolean): () => void {
  let cancelled = false
  let attempts = 0
  const run = () => {
    if (cancelled) return
    if (action()) return
    if (attempts++ < 60) window.requestAnimationFrame(run)
  }
  void window.document.fonts.ready.then(() => {
    window.requestAnimationFrame(() => window.requestAnimationFrame(run))
  })
  return () => { cancelled = true }
}

function ExpandOnMount({ label }: { label: string }) {
  useEffect(() => runWhenReady(() => {
    const button = window.document.querySelector<HTMLButtonElement>(`button[aria-label="${label}"]`)
    if (!button) return false
    button.click()
    return true
  }), [label])
  return null
}

function ExpandedStory({ theme }: { theme: 'light' | 'dark' }) {
  const doc = storyDocument([premiumPanel, coveragePanel], {
    'premium:frame': premiumFrame, 'payouts:frame': coverageFrame,
  }, {
    rows: [{
      heading: 'ПРЕМИИ',
      panels: [{ panelId: 'premium', span: 6 }, { panelId: 'payouts', span: 6 }],
    }],
  })
  return (
    <div className="lens-root" data-theme={theme}>
      <DocumentProvider initialDocument={doc}>
        <DashboardRuntimeProvider locale="ru">
          <DashboardPanels />
          <ExpandOnMount label="Expand panel" />
        </DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  )
}

export const ExpandedPanelLight: Story = () => <ExpandedStory theme="light" />
export const ExpandedPanelDark: Story = () => <ExpandedStory theme="dark" />

/**
 * One place to see every glyph the runtime draws, at the size it is used, so a
 * VR diff catches any accidental path or weight drift.
 */
const iconSet: Array<{ name: string; size: number; Glyph: (props: { size?: number }) => React.ReactElement }> = [
  { name: 'ArrowsOut', size: 14, Glyph: ArrowsOut },
  { name: 'ArrowsIn', size: 14, Glyph: ArrowsIn },
  { name: 'DownloadSimple', size: 14, Glyph: DownloadSimple },
  { name: 'ArrowClockwise', size: 14, Glyph: ArrowClockwise },
  { name: 'CircleNotch', size: 14, Glyph: CircleNotch },
  { name: 'X', size: 12, Glyph: X },
  { name: 'CaretLeft', size: 16, Glyph: CaretLeft },
  { name: 'CaretRight', size: 11, Glyph: CaretRight },
  { name: 'CaretDown', size: 14, Glyph: CaretDown },
  { name: 'ArrowUpRight', size: 12, Glyph: ArrowUpRight },
  { name: 'ArrowsLeftRight', size: 12, Glyph: ArrowsLeftRight },
]

function IconSetStory({ theme }: { theme: 'light' | 'dark' }) {
  return (
    <div className="lens-root" data-theme={theme}>
      <div className="lens-panel-grid">
        <div className="lens-grid-item" style={{ '--lens-panel-span': 12 } as React.CSSProperties}>
          <section className="lens-panel">
            <header className="lens-panel-header"><h3 className="lens-panel-title">ГЛИФЫ</h3></header>
            <div className="lens-panel-body">
              <ul className="lens-icon-specimens">
                {iconSet.map(({ name, size, Glyph }) => (
                  <li key={name}>
                    <span className="lens-icon-button"><Glyph /></span>
                    <span>{name}</span>
                    <span className="lens-icon-specimen-size">{size}px</span>
                  </li>
                ))}
              </ul>
            </div>
          </section>
        </div>
      </div>
    </div>
  )
}

export const IconSetLight: Story = () => <IconSetStory theme="light" />
export const IconSetDark: Story = () => <IconSetStory theme="dark" />

function HiddenSeries({ label }: { label: string }) {
  useEffect(() => runWhenReady(() => {
    const entries = [...window.document.querySelectorAll<HTMLButtonElement>('.lens-chart-legend-toggle')]
    const button = entries.find((entry) => entry.textContent?.includes(label))
    if (!button) return false
    button.click()
    return true
  }), [label])
  return null
}

function HideSeriesAndSelectTab({ label, tab }: { label: string; tab: string }) {
  useEffect(() => runWhenReady(() => {
    const entries = [...window.document.querySelectorAll<HTMLButtonElement>('.lens-chart-legend-toggle')]
    const legendButton = entries.find((entry) => entry.textContent?.includes(label))
    const tabs = [...window.document.querySelectorAll<HTMLButtonElement>('[role="tab"]')]
    const tabButton = tabs.find((entry) => entry.textContent === tab)
    if (!legendButton || !tabButton) return false
    legendButton.click()
    tabButton.click()
    return true
  }), [label, tab])
  return null
}

/**
 * A hidden legend entry leaves the plot entirely, so the remaining slice reads
 * 100% and the total badge drops to the visible sum.
 */
export const LegendHiddenSeries: Story = () => {
  const doc = storyDocument([premiumPanel], { 'premium:frame': premiumFrame }, {
    rows: [{ heading: 'ПРЕМИИ', panels: [{ panelId: 'premium', span: 6 }] }],
  })
  return (
    <Runtime doc={doc}>
      <DashboardPanels />
      <HiddenSeries label="Незаработанная премия" />
    </Runtime>
  )
}
LegendHiddenSeries.storyName = 'Legend hidden series'

const dailyRevenuePanel: Panel = {
  id: 'daily-revenue',
  kind: 'bar',
  title: 'Доход',
  semantics: 'series',
  frame: 'daily-revenue:frame',
  encoding: { category: 'day', series: 'product', value: 'value' },
  format: { value: money },
  presentation: { legend: 'below' },
  terminal: true,
  actions: [],
}

const dailyCountPanel: Panel = {
  ...dailyRevenuePanel,
  id: 'daily-count',
  title: 'Количество',
  frame: 'daily-count:frame',
  format: { value: { kind: 'number', minorUnits: false, precision: 0 } },
}

const dailySeriesFrame = (osago: number, kasko: number): Frame => ({
  columns: [
    { name: 'day', type: 'string' },
    { name: 'product', type: 'string' },
    { name: 'value', type: 'number' },
  ],
  rows: [
    ['30.07', 'ОСАГО', osago],
    ['30.07', 'КАСКО', kasko],
    ['31.07', 'ОСАГО', osago * 1.1],
    ['31.07', 'КАСКО', kasko * 1.2],
  ],
})

export const TabbedLegendState: Story = () => {
  const group = (tab: string) => ({ id: 'daily', kind: 'tabs' as const, span: 12, tab })
  const doc = storyDocument(
    [dailyRevenuePanel, dailyCountPanel],
    {
      'daily-revenue:frame': dailySeriesFrame(48_000_000, 13_000_000),
      'daily-count:frame': dailySeriesFrame(120, 9),
    },
    {
      rows: [{
        heading: 'ЕЖЕДНЕВНЫЕ ПРОДАЖИ',
        panels: [
          { panelId: dailyRevenuePanel.id, span: 12, groups: [group('Доход')] },
          { panelId: dailyCountPanel.id, span: 12, groups: [group('Количество')] },
        ],
      }],
    },
  )
  return (
    <Runtime doc={doc}>
      <DashboardPanels />
      <HideSeriesAndSelectTab label="ОСАГО" tab="Количество" />
    </Runtime>
  )
}
TabbedLegendState.storyName = 'Tabbed legend state'

const readabilityBar: Panel = {
  id: 'readability-bar', kind: 'hbar', title: 'Продажи по продуктам', semantics: 'series', frame: 'readability-bar:frame',
  encoding: { category: 'label', value: 'value' }, format: { value: compactNumber }, terminal: true, actions: [],
}
const compactLogPanel: Panel = {
  ...readabilityBar, id: 'compact-log', title: 'Продажи по региону', frame: 'compact-log:frame',
  valueAxis: { scale: 'logarithmic', logBase: 10 },
}
const expandableTopNPanel: Panel = {
  ...readabilityBar, id: 'expandable-top-n', title: 'Продажи по продуктам · Top 3', frame: 'expandable-top-n:frame',
  encoding: { id: 'id', category: 'label', value: 'value' },
}

export const ChartReadabilityStates: Story = () => {
  const frames = {
    'readability-bar:frame': {
      columns: [{ name: 'label', type: 'string' as const }, { name: 'value', type: 'number' as const }],
      rows: [
        ['Договор страхования спортивных профессиональных рисков', 152],
        ['13-24. Страхование профессиональной ответственности', 38],
        ['Обязательное страхование ответственности работодателя', 12],
      ],
    },
    'compact-log:frame': {
      columns: [{ name: 'label', type: 'string' as const }, { name: 'value', type: 'number' as const }],
      rows: [['Ташкент', 1_800]],
    },
    'expandable-top-n:frame': {
      columns: [
        { name: 'id', type: 'string' as const }, { name: 'label', type: 'string' as const }, { name: 'value', type: 'number' as const },
        { name: '__lens_topn_group', type: 'string' as const },
      ],
      rows: [
        ['osago', 'ОСАГО', 1_800, null], ['kasko', 'КАСКО', 152, null], ['travel', 'Путешествия', 38, null],
        ['employer', 'Ответственность работодателя', 5, 'Прочее'], ['cargo', 'Страхование грузов', 4, 'Прочее'],
        ['property', 'Страхование имущества', 3, 'Прочее'],
      ],
    },
  }
  const doc = storyDocument([readabilityBar, compactLogPanel, expandableTopNPanel], frames, {
    rows: [
      { heading: 'ЧИТАЕМОСТЬ', panels: [{ panelId: readabilityBar.id, span: 7 }, { panelId: compactLogPanel.id, span: 5 }] },
      { heading: 'СВЁРНУТЫЙ ХВОСТ', panels: [{ panelId: expandableTopNPanel.id, span: 12 }] },
    ],
  })
  return <Runtime doc={doc}><DashboardPanels /></Runtime>
}
ChartReadabilityStates.storyName = 'Chart readability states'

const donutTailPanel: Panel = {
  id: 'donut-tail', kind: 'donut', title: 'Тип оплаты', semantics: 'partition', frame: 'donut-tail:frame',
  encoding: { id: 'id', label: 'label', value: 'value' }, format: { value: compactMoney },
  presentation: { legend: 'below' }, terminal: true, actions: [],
}
const donutTailFrame: Frame = {
  columns: [{ name: 'id', type: 'string' }, { name: 'label', type: 'string' }, { name: 'value', type: 'number' }],
  rows: [
    ['card', 'Банковская карта', 9_600_000], ['cash', 'Наличные', 260_000],
    ['octo', 'Octo', 70_000], ['transfer', 'Банковский перевод', 45_000], ['other', 'Другое', 25_000],
  ],
  total: 10_000_000,
}

export const DonutCollapsedTail: Story = () => {
  const doc = storyDocument([donutTailPanel], { 'donut-tail:frame': donutTailFrame }, {
    rows: [{ heading: 'ДОЛИ', panels: [{ panelId: donutTailPanel.id, span: 7 }] }],
  })
  return <Runtime doc={doc}><DashboardPanels /></Runtime>
}
DonutCollapsedTail.storyName = 'Donut collapsed tail'

const labelledBar: Panel = {
  ...readabilityBar, id: 'labelled-bar', title: 'Полисы', frame: 'labelled-bar:frame', kind: 'bar',
  presentation: { dataLabels: true },
}
const labelledLine: Panel = {
  ...trendPanel, id: 'labelled-line', title: 'Премия', frame: 'labelled-line:frame', presentation: { dataLabels: true },
}

export const OptInDataLabels: Story = () => {
  const frames = {
    'labelled-bar:frame': { columns: [{ name: 'label', type: 'string' as const }, { name: 'value', type: 'number' as const }], rows: [['Янв', 31], ['Фев', 47], ['Мар', 55]] },
    'labelled-line:frame': trendFrame,
  }
  const doc = storyDocument([labelledBar, labelledLine], frames, {
    rows: [{ heading: 'ПОДПИСИ ДАННЫХ', panels: [{ panelId: labelledBar.id, span: 6 }, { panelId: labelledLine.id, span: 6 }] }],
  })
  return <Runtime doc={doc}><DashboardPanels /></Runtime>
}
OptInDataLabels.storyName = 'Opt-in data labels'

const legendSeries = Array.from({ length: 11 }, (_, index) => `Продукт ${String(index + 1).padStart(2, '0')}`)
const legendPanel: Panel = {
  id: 'legend-workstation', kind: 'bar', title: 'Ежедневный доход', semantics: 'series', frame: 'legend-workstation:frame',
  encoding: { category: 'day', series: 'product', value: 'value' }, format: { value: compactMoney },
  presentation: { legend: 'below', stack: true }, terminal: true, actions: [],
}
const legendFrame: Frame = {
  columns: [{ name: 'day', type: 'string' }, { name: 'product', type: 'string' }, { name: 'value', type: 'number' }],
  rows: ['30.07', '31.07'].flatMap((day, dayIndex) => legendSeries.map((series, index) => [day, series, (index + 1) * (dayIndex + 1) * 1_000_000])),
}

function HideEverySeries() {
  useEffect(() => runWhenReady(() => {
    const button = window.document.querySelector<HTMLButtonElement>('.lens-chart-legend-tools button')
    if (!button) return false
    button.click()
    return true
  }), [])
  return null
}

/**
 * What a plot says when the reader has switched every series off.
 *
 * A donut with nothing in it drew a featureless grey ring and kept printing the
 * full total in its hub while the chip above it read «Итого: 0 UZS» — a panel
 * that looks broken and contradicts itself in the same card. Nothing shown,
 * nothing drawn, nothing totalled, and one way back stated in words.
 */
export const LegendAllSeriesHidden: Story = () => {
  const doc = storyDocument([premiumPanel], { 'premium:frame': premiumFrame }, {
    rows: [{ heading: 'ВСЁ СКРЫТО', panels: [{ panelId: 'premium', span: 6 }] }],
  })
  return (
    <Runtime doc={doc}>
      <DashboardPanels />
      <HideEverySeries />
    </Runtime>
  )
}
LegendAllSeriesHidden.storyName = 'Legend all series hidden'

export const LegendControlsAndSearch: Story = () => {
  const doc = storyDocument([legendPanel], { 'legend-workstation:frame': legendFrame }, {
    rows: [{ heading: 'ЛЕГЕНДА', panels: [{ panelId: legendPanel.id, span: 7 }] }],
  })
  return <Runtime doc={doc}><DashboardPanels /></Runtime>
}
LegendControlsAndSearch.storyName = 'Legend controls and search'

function SoloLegendOnMount() {
  useEffect(() => runWhenReady(() => {
    const button = [...document.querySelectorAll<HTMLButtonElement>('.lens-chart-legend-toggle')]
      .find((entry) => entry.textContent?.includes('Продукт 04'))
    if (!button) return false
    button.dispatchEvent(new MouseEvent('dblclick', { bubbles: true, detail: 2 }))
    return true
  }), [])
  return null
}

export const LegendSoloState: Story = () => {
  const doc = storyDocument([legendPanel], { 'legend-workstation:frame': legendFrame }, {
    rows: [{ heading: 'ИЗОЛИРОВАННАЯ СЕРИЯ', panels: [{ panelId: legendPanel.id, span: 12 }] }],
  })
  return <Runtime doc={doc}><DashboardPanels /><SoloLegendOnMount /></Runtime>
}
LegendSoloState.storyName = 'Legend solo state'

function HideLegendSeriesOnMount({ label }: { label: string }) {
  useEffect(() => runWhenReady(() => {
    const button = [...document.querySelectorAll<HTMLButtonElement>('.lens-chart-legend-toggle')]
      .find((entry) => entry.textContent?.includes(label))
    if (!button) return false
    button.click()
    return true
  }), [label])
  return null
}

export const HiddenStackCollapsed: Story = () => {
  const doc = storyDocument([compositionPanel], { 'composition:frame': compositionFrame }, {
    rows: [{ heading: 'СТЕК ПОСЛЕ СКРЫТИЯ', panels: [{ panelId: compositionPanel.id, span: 12 }] }],
  })
  return <Runtime doc={doc}><DashboardPanels /><HideLegendSeriesOnMount label="Прямое страхование" /></Runtime>
}
HiddenStackCollapsed.storyName = 'Hidden stack collapsed'

const gaugePanel: Panel = {
  id: 'budget-gauge', kind: 'gauge', title: 'Использование месячного бюджета', semantics: 'series', frame: 'budget-gauge:frame',
  encoding: { value: 'value' }, format: { value: { kind: 'percent', minorUnits: false, precision: 1, decimalSeparator: '.' } }, terminal: true, actions: [],
}

export const Gauge: Story = () => {
  const doc = storyDocument([gaugePanel], {
    'budget-gauge:frame': { columns: [{ name: 'value', type: 'number' }], rows: [[68.4]] },
  }, { rows: [{ heading: 'БЮДЖЕТ', panels: [{ panelId: gaugePanel.id, span: 5 }] }] })
  return <Runtime doc={doc}><DashboardPanels /></Runtime>
}
Gauge.storyName = 'Gauge'

const crowdedHeaderPanel: Panel = {
  ...premiumPanel,
  id: 'crowded',
  title: 'Финансовые расходы и прочие операционные затраты',
  total: 118_800_000_000_000,
  presentation: { legend: 'below', sliceLabels: 'percent', fill: true },
}

/**
 * A long title next to a wide total badge. The title is the panel's identity,
 * so it holds its width and the badge ellipsizes (its tooltip keeps the full
 * number reachable).
 */
export const PanelHeaderPressure: Story = () => {
  const doc = storyDocument([crowdedHeaderPanel], { 'premium:frame': premiumFrame }, {
    rows: [{ heading: 'ПРЕМИИ', panels: [{ panelId: 'crowded', span: 5 }] }],
  })
  return <Runtime doc={doc}><DashboardPanels /></Runtime>
}
PanelHeaderPressure.storyName = 'Panel header pressure'

const referralHeaderMetrics = [
  { panel: statPanel('earned-total', 'Всего заработано', '#2f56d9'), value: 8.69 },
  { panel: statPanel('available-balance', 'Доступный баланс', '#059669'), value: 2.04 },
  { panel: statPanel('pending-balance', 'Баланс в ожидании', '#d97824'), value: 6.65 },
  { panel: statPanel('withdrawn-total', 'Всего выведено', '#7c3aed'), value: 0 },
]

function ReferralHeaderStrip({ width }: { width: number }) {
  const frames = Object.fromEntries(referralHeaderMetrics.map(({ panel, value }) => [
    `${panel.id}:frame`, statFrame(panel.title, value),
  ]))
  const doc = storyDocument(
    referralHeaderMetrics.map(({ panel }) => panel),
    frames,
    {
      rows: [{
        heading: `${width} PX HOST CONTENT · FOUR-UP`,
        panels: referralHeaderMetrics.map(({ panel }) => ({ panelId: panel.id, span: 3 })),
      }],
    },
  )
  return <div style={{ width }}><Runtime doc={doc}><DashboardPanels /></Runtime></div>
}

/** Desktop-host and tablet-host content widths keep the same four-up layout.
 * Every long title must remain visible while all export/expand controls keep
 * their own stable row inside the card. */
export const PanelHeaderFourUpPressure: Story = () => (
  <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
    <ReferralHeaderStrip width={1030} />
    <ReferralHeaderStrip width={640} />
  </div>
)
PanelHeaderFourUpPressure.storyName = 'Panel header four-up pressure'

const navigateAction = (urlTemplate: string, params: Panel['actions'][number]['params'] = []) => ({
  kind: 'navigate' as const, method: 'GET', urlTemplate, params, payload: {},
})

const linkedMetrics = metrics.map(({ panel, value }) => ({
  panel: { ...panel, terminal: false, actions: [navigateAction(`/analytics/metrics/${panel.id}`)] },
  value,
}))

const linkedCoveragePanel: Panel = {
  ...coveragePanel,
  id: 'payouts-linked',
  terminal: false,
  actions: [navigateAction('/claims/{bucket}', [{ name: 'bucket', source: { kind: 'field', name: 'label' } }])],
}

/**
 * Panel-level navigate actions: the KPI strip is a row of links again, and a
 * coverage card whose action reads a row field links each segment separately.
 */
export const ClickablePanels: Story = () => {
  const frames = Object.fromEntries(
    linkedMetrics.map(({ panel, value }) => [`${panel.id}:frame`, statFrame(panel.title, value)]),
  )
  const doc = storyDocument(
    [...linkedMetrics.map(({ panel }) => panel), linkedCoveragePanel],
    { ...frames, 'payouts:frame': coverageFrame },
    {
      rows: [
        {
          heading: 'КЛЮЧЕВЫЕ КОЭФФИЦИЕНТЫ',
          panels: linkedMetrics.map(({ panel }) => ({
            panelId: panel.id, span: 3,
            groups: [{ id: 'earned', kind: 'metrics' as const, label: 'ПО ЗАРАБОТАННОЙ ПРЕМИИ', layout: 'columns' as const, span: 12 }],
          })),
        },
        { heading: 'ВЫПЛАТЫ', panels: [{ panelId: 'payouts-linked', span: 6 }] },
      ],
    },
  )
  return <Runtime doc={doc}><DashboardPanels /></Runtime>
}
ClickablePanels.storyName = 'Clickable panels'

/**
 * A stat panel sharing a row with a table long enough to set the row's height.
 *
 * The row stretches every card to its tallest sibling, which is right for a
 * plot and wrong for a stat: the stat used to grow to the table's height and
 * float its delta chip at the bottom of the resulting gap. The figure, its
 * caption and its chip stay one block at the top of the card here, and the
 * slack falls below them.
 */
const stretchStatPanel: Panel = {
  id: 'stretch-stat', kind: 'stat', title: 'Комбинированный коэффициент', semantics: 'series',
  frame: 'stretch-stat:frame',
  encoding: { label: 'label', value: 'value', final: 'delta' },
  format: {
    value: { kind: 'percent', minorUnits: false, precision: 1, decimalSeparator: '.' },
    delta: { kind: 'percent', minorUnits: false, precision: 1, decimalSeparator: '.' },
  },
  caption: 'Убытки и расходы к заработанной премии',
  trend: { percent: -4.2, polarity: 'lower_better', absoluteField: 'delta', percentField: 'deltaPercent' },
  terminal: true, actions: [],
}

const stretchStatFrame: Frame = {
  columns: [
    { name: 'label', type: 'string' }, { name: 'value', type: 'number' },
    { name: 'delta', type: 'number' }, { name: 'deltaPercent', type: 'number' },
  ],
  rows: [['Комбинированный коэффициент', 44.8, -2, -4.2]],
}

const stretchTablePanel: Panel = {
  id: 'stretch-table', kind: 'table', title: 'Продукты', semantics: 'partition', frame: 'stretch-table:frame',
  encoding: { label: 'product', value: 'amount' },
  format: { amount: money },
  columns: [
    { field: 'product', label: 'Продукт', cell: { kind: 'plain' } },
    { field: 'amount', label: 'Премия', align: 'right', cell: { kind: 'plain' } },
  ],
  terminal: true, actions: [],
}

const stretchTableFrame: Frame = {
  columns: [{ name: 'product', type: 'string' }, { name: 'amount', type: 'number' }],
  rows: Array.from({ length: 25 }, (_, index) => [`Продукт ${index + 1}`, (25 - index) * 41_000_000]),
}

export const StatBesideTallTable: Story = () => {
  const doc = storyDocument(
    [stretchStatPanel, stretchTablePanel],
    { 'stretch-stat:frame': stretchStatFrame, 'stretch-table:frame': stretchTableFrame },
    { rows: [{ panels: [{ panelId: 'stretch-stat', span: 3 }, { panelId: 'stretch-table', span: 9 }] }] },
  )
  return <Runtime doc={doc}><DashboardPanels /></Runtime>
}
StatBesideTallTable.storyName = 'Stat beside a tall table'

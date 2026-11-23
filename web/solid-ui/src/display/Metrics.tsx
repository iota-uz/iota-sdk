import { createSignal, For, Show, splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'
import { ArrowDownLeftIcon, ArrowUpRightIcon, CalendarBlankIcon, FunnelIcon, InfoIcon, ScalesIcon } from '../internal/icons'
import {
  formatAmount,
  formatCompact,
  formatCount,
  isNegative,
  sign,
  toMinor,
  valueText,
  VISIBLE_CURRENCIES,
  type Amount,
  type MetricKind,
  type MinorUnits,
} from './metricsFormat'

export {
  compactUnits,
  formatAmount,
  formatCompact,
  formatCount,
  money,
  order,
  sign,
  VISIBLE_CURRENCIES,
  type Amount,
  type CompactOptions,
  type CompactParts,
  type MetricKind,
  type MetricValues,
  type MinorUnits,
} from './metricsFormat'

/** One card. Values are keyed by currency code; a `count` metric keys its single value by "". */
export interface Metric {
  label: string
  kind: MetricKind
  values: Readonly<Record<string, Amount | undefined>>
  hint?: string
}

/** The selection the totals are computed over. */
export interface MetricsScope {
  /** A ready label such as "1–30 ноября 2026"; empty hides it. */
  period?: string
  /** The number of applied filters; zero hides it. */
  filters?: number
  /** Records in the unfiltered list. */
  total: MinorUnits
  /** Records left after search; absent when the list is not narrowed by search. */
  matched?: MinorUnits
  /** Overrides the record-count label (templ `CountMessageID`), e.g. a debts noun. */
  countLabel?: (count: string, n: number) => string
}

export type MetricsState = 'ready' | 'empty'
export type MetricsAttach = 'table' | 'standalone'

/**
 * The band's words. Plural labels receive the formatted number and the raw
 * count, as the templ `Metrics.*` messages receive `Count` and `PluralCount`.
 */
export interface MetricsLabels {
  title: string
  records: (count: string, n: number) => string
  matched: (matched: string, total: string) => string
  filters: (n: number) => string
  showAll: string
  moreCurrencies: (n: number) => string
  fewerCurrencies: string
  empty: string
}

export const defaultMetricsLabels: MetricsLabels = {
  title: 'Totals for selection',
  records: (count) => `${count} records`,
  matched: (matched, total) => `Found ${matched} of ${total}`,
  filters: (n) => `${n} filters`,
  showAll: 'Show all',
  moreCurrencies: (n) => `${n} more currencies`,
  fewerCurrencies: 'Fewer currencies',
  empty: 'Nothing found.',
}

export interface MetricsProps extends Omit<JSX.HTMLAttributes<HTMLElement>, 'children'> {
  scope: MetricsScope
  items: readonly Metric[]
  /** The one order for the whole band, base currency first; build it with `order`. */
  currencies: readonly string[]
  state?: MetricsState
  attach?: MetricsAttach
  /** Replaces the default hint shown when the selection is empty. */
  emptyNote?: JSX.Element
  /** Wires "Show all" when the list is narrowed; absent hides the link. */
  onReset?: () => void
  labels?: Partial<MetricsLabels>
}

function numberOf(value: MinorUnits): number {
  return Number(toMinor(value))
}

function KindIcon(props: { kind: MetricKind }) {
  return (
    <>
      <Show when={props.kind === 'in'}><span class="iota-mx-icon"><ArrowDownLeftIcon size={14} /></span></Show>
      <Show when={props.kind === 'out'}><span class="iota-mx-icon"><ArrowUpRightIcon size={14} /></span></Show>
      <Show when={props.kind === 'net'}><span class="iota-mx-icon"><ScalesIcon size={14} /></span></Show>
    </>
  )
}

function MetricLabel(props: { label: string; kind: MetricKind; hint?: string }) {
  return (
    <div class="iota-mx-label">
      <KindIcon kind={props.kind} />
      <span>{props.label}</span>
      <Show when={props.hint}>
        {(hint) => <span class="iota-mx-hint" role="img" title={hint()} aria-label={hint()}><InfoIcon size={14} /></span>}
      </Show>
    </div>
  )
}

function lookup(item: Metric, code: string): Amount | undefined {
  return item.values[item.kind === 'count' ? '' : code]
}

function Value(props: { item: Metric; code: string; big: boolean }) {
  const amount = () => lookup(props.item, props.code)
  return (
    <Show
      when={amount()}
      fallback={<span class={classes('iota-mx-v', 'iota-mx-v--none', props.big && 'iota-mx-v--big')}>—</span>}
    >
      {(value) => (
        <span
          class={classes('iota-mx-v', props.big && 'iota-mx-v--big', isNegative(props.item.kind, value()) && 'iota-mx-v--neg')}
          title={[valueText(props.item.kind, value()), props.code].filter(Boolean).join(' ')}
        >
          {valueText(props.item.kind, value())}
          <Show when={props.code}>{' '}<small>{props.code}</small></Show>
        </span>
      )}
    </Show>
  )
}

function gridStyle(n: number): JSX.CSSProperties {
  return { '--mx-n': String(Math.max(n, 1)) }
}

/** The KPI band: the scope line, then one equal card per metric, one row per currency. */
export function Metrics(props: MetricsProps) {
  const [local, native] = splitProps(props, ['scope', 'items', 'currencies', 'state', 'attach', 'emptyNote', 'onReset', 'labels', 'class'])
  const labels = (): MetricsLabels => ({ ...defaultMetricsLabels, ...local.labels })
  const [all, setAll] = createSignal(false)
  const visible = () => local.currencies.slice(0, VISIBLE_CURRENCIES)
  const hidden = () => local.currencies.slice(VISIBLE_CURRENCIES)
  const empty = () => local.state === 'empty'

  const countText = () => {
    const scope = local.scope
    if (scope.matched !== undefined) return labels().matched(formatCount(scope.matched), formatCount(scope.total))
    const count = formatCount(scope.total)
    return (scope.countLabel ?? labels().records)(count, numberOf(scope.total))
  }

  return (
    <section {...native} class={classes('iota-mx', `iota-mx--${local.attach ?? 'table'}`, local.class)}>
      <div class="iota-mx-scope">
        <span class="iota-mx-scope-title">{labels().title}</span>
        <Show when={local.scope.period}>
          <span class="iota-mx-scope-meta"><CalendarBlankIcon size={14} />{local.scope.period}</span>
        </Show>
        <Show when={(local.scope.filters ?? 0) > 0}>
          <span class="iota-mx-scope-meta"><FunnelIcon size={14} />{labels().filters(local.scope.filters ?? 0)}</span>
        </Show>
        <span class="iota-mx-scope-count">
          {countText()}
          <Show when={local.onReset}>
            <span aria-hidden="true">·</span>
            <button type="button" class="iota-mx-a" onClick={() => local.onReset?.()}>{labels().showAll}</button>
          </Show>
        </span>
      </div>
      <Show when={local.items.length > 0}>
        <div class="iota-mx-grid" style={gridStyle(local.items.length)}>
          <For each={local.items}>
            {(item) => (
              <div class={classes('iota-mx-card', empty() && 'iota-mx-card--empty')} data-kind={item.kind}>
                <MetricLabel label={item.label} kind={item.kind} hint={item.hint} />
                <Show
                  when={item.kind !== 'count' && local.currencies.length > 0}
                  fallback={<Value item={item} code="" big />}
                >
                  <Value item={item} code={local.currencies[0]!} big />
                  <Show when={local.currencies.length > 1}>
                    <dl class="iota-mx-rows">
                      <For each={visible().slice(1)}>
                        {(code) => <div><dt>{code}</dt><dd><Value item={item} code={code} big={false} /></dd></div>}
                      </For>
                      <For each={hidden()}>
                        {(code) => (
                          <div style={all() ? undefined : { display: 'none' }} data-folded="">
                            <dt>{code}</dt><dd><Value item={item} code={code} big={false} /></dd>
                          </div>
                        )}
                      </For>
                    </dl>
                  </Show>
                </Show>
              </div>
            )}
          </For>
        </div>
      </Show>
      <Show when={empty()}>
        <div class="iota-mx-note">{local.emptyNote ?? labels().empty}</div>
      </Show>
      <Show when={hidden().length > 0}>
        <div class="iota-mx-note">
          <button type="button" class="iota-mx-a" aria-expanded={all()} onClick={() => setAll(!all())}>
            {all() ? labels().fewerCurrencies : labels().moreCurrencies(hidden().length)}
          </button>
        </div>
      </Show>
    </section>
  )
}

export interface MetricsSkeletonProps extends Omit<JSX.HTMLAttributes<HTMLElement>, 'children'> {
  /** Cards of the band it stands for. */
  items: number
  /** Currencies of the band it stands for. */
  currencies: number
  attach?: MetricsAttach
}

const range = (n: number) => Array.from({ length: Math.max(n, 0) }, (_, index) => index)

/** Keeps the exact box of the band while it loads, so the table below does not jump. */
export function MetricsSkeleton(props: MetricsSkeletonProps) {
  const [local, native] = splitProps(props, ['items', 'currencies', 'attach', 'class'])
  return (
    <section
      {...native}
      class={classes('iota-mx', `iota-mx--${local.attach ?? 'table'}`, local.class)}
      aria-busy="true"
      aria-live="polite"
    >
      <div class="iota-mx-scope">
        <span class="iota-mx-sk iota-mx-sk--label" />
        <span class="iota-mx-scope-count"><span class="iota-mx-sk iota-mx-sk--count" /></span>
      </div>
      <div class="iota-mx-grid" style={gridStyle(local.items)}>
        <For each={range(Math.max(local.items, 1))}>
          {() => (
            <div class="iota-mx-card">
              <span class="iota-mx-sk iota-mx-sk--label" />
              <span class="iota-mx-sk iota-mx-sk--big" />
              <Show when={local.currencies > 1}>
                <div class="iota-mx-rows">
                  <For each={range(Math.min(local.currencies, VISIBLE_CURRENCIES) - 1)}>
                    {() => <span class="iota-mx-sk iota-mx-sk--row" />}
                  </For>
                </div>
              </Show>
            </div>
          )}
        </For>
      </div>
      <Show when={local.currencies > VISIBLE_CURRENCIES}>
        <div class="iota-mx-note"><span class="iota-mx-sk iota-mx-sk--count" /></div>
      </Show>
    </section>
  )
}

export interface KpiTileProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'children'> {
  label: string
  /** The value: an `Amount`, or whole units as a bigint or safe integer. */
  v: Amount | MinorUnits
  /** Currency code; a money tile shows a compact number, a tile without one the full value. */
  cur?: string
  /** A caption under the value, e.g. "96,2 % от премии". */
  delta?: JSX.Element
  kind?: MetricKind
  hint?: string
  /** Locale of the magnitude words (CLDR: ru → млрд, en → B). Defaults to "en-US". */
  locale?: string
  /** Fraction digits of a compact mantissa. Defaults to 2. */
  digits?: number
}

function asAmount(v: Amount | MinorUnits): Amount {
  return typeof v === 'object' ? v : { minor: v, decimals: 0 }
}

/**
 * A dashboard tile: one value with the band's typography. A money value is
 * compact («19,30 млрд UZS») and its full value is in the tooltip.
 */
export function KpiTile(props: KpiTileProps) {
  const [local, native] = splitProps(props, ['label', 'v', 'cur', 'delta', 'kind', 'hint', 'locale', 'digits', 'class'])
  const kind = () => local.kind ?? 'plain'
  const amount = () => asAmount(local.v)
  const compact = () => local.cur
    ? formatCompact(amount(), { locale: local.locale, digits: local.digits })
    : { number: formatAmount(amount()), unit: '' }
  const unit = () => [compact().unit, local.cur].filter(Boolean).join('\u00A0')
  const full = () => local.cur ? `${valueText(kind(), amount())} ${local.cur}` : undefined
  return (
    <div {...native} class={classes('iota-mx-card', local.class)} data-kind={kind()}>
      <MetricLabel label={local.label} kind={kind()} hint={local.hint} />
      <span
        class={classes('iota-mx-v', 'iota-mx-v--big', isNegative(kind(), amount()) && 'iota-mx-v--neg')}
        title={full()}
      >
        {sign(kind(), amount().minor) + compact().number}
        <Show when={unit()}>{' '}<small>{unit()}</small></Show>
      </span>
      <Show when={local.delta}><div class="iota-mx-d">{local.delta}</div></Show>
    </div>
  )
}

export interface KpiTilesProps extends JSX.HTMLAttributes<HTMLDivElement> {
  /** Columns from 640px up; defaults to 4. */
  columns?: number
}

/** A framed row of KPI tiles; one column under 640px. */
export function KpiTiles(props: KpiTilesProps) {
  const [local, native] = splitProps(props, ['columns', 'class', 'style', 'children'])
  return (
    <div
      {...native}
      class={classes('iota-mx-tiles', local.class)}
      style={{ ...gridStyle(local.columns ?? 4), ...(typeof local.style === 'object' ? local.style : {}) }}
    >
      {local.children}
    </div>
  )
}

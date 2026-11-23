import { afterEach, describe, expect, it } from 'vitest'
import { render } from 'solid-js/web'
import { KpiTile, KpiTiles, Metrics, MetricsSkeleton, money, order, type Metric } from './Metrics'

let dispose: (() => void) | undefined
let host: HTMLDivElement

function mount(view: () => unknown) {
  host = document.createElement('div')
  document.body.append(host)
  dispose = render(view as never, host)
  return host
}

afterEach(() => {
  dispose?.()
  dispose = undefined
  document.body.replaceChildren()
})

const count = (selector: string) => host.querySelectorAll(selector).length
const dts = (code: string) => [...host.querySelectorAll('dt')].filter((dt) => dt.textContent === code).length

describe('Metrics', () => {
  it('gives every card every currency', () => {
    const inflow: Metric = { label: 'Поступления', kind: 'in', values: { UZS: money(100), USD: money(200) } }
    const outflow: Metric = { label: 'Списания', kind: 'out', values: { UZS: money(50), RUB: money(10) } }
    const net: Metric = { label: 'Сальдо', kind: 'net', values: { UZS: money(-300) } }
    mount(() => <Metrics scope={{ total: 2751 }} items={[inflow, outflow, net]} currencies={order('UZS', [], inflow, outflow, net)} />)

    expect(count('.iota-mx-card')).toBe(3)
    expect((host.querySelector('.iota-mx-grid') as HTMLElement).style.getPropertyValue('--mx-n')).toBe('3')
    expect(dts('USD')).toBe(3)
    expect(dts('RUB')).toBe(3)
    expect([...host.querySelectorAll('.iota-mx-v--none')].filter((node) => node.textContent === '—')).toHaveLength(4)
    expect(host.textContent).toContain('+1,00')
    expect(host.textContent).toContain('\u22120,50')
    expect(host.querySelector('.iota-mx-v--neg')).not.toBeNull()
    expect(host.querySelector('.iota-mx-scope-count')?.textContent).toBe('2\u00A0751 records')
    expect(host.querySelector('.iota-mx-note')).toBeNull()
    expect(host.querySelector('section')?.className).toBe('iota-mx iota-mx--table')
  })

  it('folds currencies past three and keeps them in the markup', () => {
    const metric: Metric = { label: 'Сумма', kind: 'plain', values: { UZS: money(1), USD: money(1), EUR: money(1), RUB: money(1), KZT: money(1) } }
    mount(() => <Metrics scope={{ total: 1 }} items={[metric]} currencies={order('UZS', [], metric)} />)

    const folded = [...host.querySelectorAll<HTMLElement>('[data-folded]')]
    expect(folded.map((row) => row.querySelector('dt')?.textContent)).toEqual(['RUB', 'USD'])
    expect(folded.every((row) => row.style.display === 'none')).toBe(true)
    const toggle = host.querySelector<HTMLButtonElement>('.iota-mx-note button')!
    expect(toggle.textContent).toBe('2 more currencies')
    expect(toggle.getAttribute('aria-expanded')).toBe('false')
    toggle.click()
    expect(folded.every((row) => row.style.display === '')).toBe(true)
    expect(toggle.textContent).toBe('Fewer currencies')
    expect(toggle.getAttribute('aria-expanded')).toBe('true')
  })

  it('shows a narrowed and empty selection with its reset', () => {
    let resets = 0
    mount(() => (
      <Metrics
        scope={{ total: 2751, matched: 12, filters: 2, period: 'Ноябрь 2026' }}
        items={[{ label: 'Сумма', kind: 'plain', values: {} }]}
        currencies={[]}
        state="empty"
        emptyNote="Нет операций"
        onReset={() => { resets += 1 }}
      />
    ))
    expect(host.textContent).toContain('Ноябрь 2026')
    expect(host.textContent).toContain('2 filters')
    expect(host.textContent).toContain('Found 12 of 2\u00A0751')
    expect(host.querySelector('.iota-mx-note')?.textContent).toBe('Нет операций')
    expect(host.querySelector('.iota-mx-card--empty')).not.toBeNull()
    const reset = [...host.querySelectorAll('button')].find((button) => button.textContent === 'Show all')!
    reset.click()
    expect(resets).toBe(1)
  })

  it('takes localized labels with plural counts and a scope noun', () => {
    mount(() => (
      <Metrics
        scope={{ total: 21, countLabel: (text, n) => `${text} ${n % 10 === 1 ? 'долг' : 'долга'}` }}
        items={[]}
        currencies={[]}
        labels={{ title: 'Итоги по выборке' }}
      />
    ))
    expect(host.textContent).toContain('Итоги по выборке')
    expect(host.querySelector('.iota-mx-scope-count')?.textContent).toBe('21 долг')
    expect(host.querySelector('.iota-mx-grid')).toBeNull()
  })

  it('uses the English fallbacks without labels', () => {
    mount(() => <Metrics scope={{ total: 2 }} items={[]} currencies={[]} onReset={() => undefined} />)
    expect(host.textContent).toContain('Totals for selection')
    expect(host.textContent).toContain('Show all')
  })

  it('renders a count metric from its single value', () => {
    mount(() => <Metrics scope={{ total: 1 }} items={[{ label: 'Ops', kind: 'count', values: { '': { minor: 2751, decimals: 0 } } }]} currencies={['UZS', 'USD']} />)
    expect(host.querySelector('.iota-mx-v--big')?.textContent).toBe('2\u00A0751')
    expect(host.querySelector('dl')).toBeNull()
  })
})

describe('MetricsSkeleton', () => {
  it('keeps the box of the band', () => {
    mount(() => <MetricsSkeleton items={3} currencies={2} />)
    expect(count('.iota-mx-card')).toBe(3)
    expect(count('.iota-mx-sk--row')).toBe(3)
    expect(host.querySelector('section')?.getAttribute('aria-busy')).toBe('true')
    expect(host.querySelector('.iota-mx-note')).toBeNull()
  })

  it('reserves the currency toggle', () => {
    mount(() => <MetricsSkeleton items={1} currencies={4} />)
    expect(host.querySelector('.iota-mx-note')).not.toBeNull()
    expect(count('.iota-mx-sk--row')).toBe(2)
  })
})

describe('KpiTile', () => {
  it('shows a compact money value with the full one in its title', () => {
    mount(() => (
      <KpiTiles columns={2}>
        <KpiTile label="Списания" v={money(1929625692367n)} cur="UZS" kind="out" locale="ru" delta="за 2026 год" />
        <KpiTile label="Полисы" v={12480} />
        <KpiTile label="Премия" v={money(1929625692367n)} cur="UZS" locale="ru" digits={1} />
      </KpiTiles>
    ))
    expect((host.firstElementChild as HTMLElement).style.getPropertyValue('--mx-n')).toBe('2')
    const [money_, plain] = [...host.querySelectorAll<HTMLElement>('.iota-mx-v--big')]
    expect(money_!.textContent).toBe('\u221219,30 млрд\u00A0UZS')
    expect(money_!.querySelector('small')?.textContent).toBe('млрд\u00A0UZS')
    expect(money_!.title).toBe('\u221219\u00A0296\u00A0256\u00A0923,67 UZS')
    expect(host.querySelector('.iota-mx-d')?.textContent).toBe('за 2026 год')
    expect(plain!.textContent).toBe('12\u00A0480')
    expect(plain!.hasAttribute('title')).toBe(false)
    expect(host.querySelectorAll('.iota-mx-v--big')[2]!.textContent).toBe('19,3 млрд\u00A0UZS')
  })

  it('marks only a negative balance', () => {
    mount(() => (
      <KpiTiles>
        <KpiTile label="Net" v={money(-500)} cur="USD" kind="net" />
        <KpiTile label="Out" v={money(500)} cur="USD" kind="out" />
      </KpiTiles>
    ))
    expect(count('.iota-mx-v--neg')).toBe(1)
  })
})

describe('KpiTiles style', () => {
  it('keeps string and object styles next to the column count', () => {
    mount(() => <KpiTiles columns={2} style="padding: 2rem"><span /></KpiTiles>)
    let tiles = host.querySelector('.iota-mx-tiles') as HTMLElement
    expect(tiles.style.getPropertyValue('--mx-n')).toBe('2')
    expect(tiles.style.padding).toBe('2rem')
    dispose?.()
    document.body.replaceChildren()
    mount(() => <KpiTiles style={{ padding: '1rem' }}><span /></KpiTiles>)
    tiles = host.querySelector('.iota-mx-tiles') as HTMLElement
    expect(tiles.style.getPropertyValue('--mx-n')).toBe('4')
    expect(tiles.style.padding).toBe('1rem')
  })
})

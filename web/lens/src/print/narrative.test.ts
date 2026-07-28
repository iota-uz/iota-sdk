import { describe, expect, it } from 'vitest'
import type { Frame, Panel } from '../contract'
import { headlineReadings, narrativeFact } from './narrative'

function panelWith(overrides: Partial<Panel>): Panel {
  return {
    id: 'panel',
    kind: 'donut',
    title: 'Panel',
    semantics: 'partition',
    frame: 'panel:root',
    encoding: { label: 'label', value: 'value' },
    format: { value: { kind: 'number', minorUnits: false, precision: 0 } },
    actions: [],
    ...overrides,
  }
}

function frameWith(rows: Array<Array<unknown>>, extra: Array<string> = []): Frame {
  return {
    columns: [
      { name: 'label', type: 'string' },
      { name: 'value', type: 'number' },
      ...extra.map((name) => ({ name, type: 'bool' as const })),
    ],
    rows,
  }
}

describe('narrativeFact', () => {
  it('names the leading share and the weight of the three largest', () => {
    const fact = narrativeFact(
      panelWith({}),
      frameWith([['Motor', 60], ['Property', 20], ['Travel', 15], ['Cargo', 5]]),
      'en',
    )

    expect(fact).toMatchObject({
      labelKey: 'print.factPartition',
      vars: { label: 'Motor', share: '60%', top: '95%', rest: 1 },
    })
  })

  it('states the remainder instead of a top-three weight for a two-slice split', () => {
    const fact = narrativeFact(panelWith({}), frameWith([['Earned', 75], ['Unearned', 25]]), 'en')

    expect(fact).toMatchObject({ labelKey: 'print.factPair', vars: { label: 'Earned', share: '75%', rest: 'Unearned' } })
  })

  it('weighs a share inside its own ring when a frame holds two partitions', () => {
    // Two rings of one donut: recognition splits the premium, payment splits
    // the same total again. Summing across them would weigh each reading
    // against twice its own whole.
    const fact = narrativeFact(
      panelWith({ encoding: { label: 'label', value: 'value', series: 'ring' } }),
      {
        columns: [
          { name: 'label', type: 'string' },
          { name: 'value', type: 'number' },
          { name: 'ring', type: 'string' },
        ],
        rows: [
          ['Earned', 78, 'recognition'],
          ['Unearned', 22, 'recognition'],
          ['Collected', 99, 'payment'],
          ['Receivable', 1, 'payment'],
        ],
      },
      'en',
    )

    expect(fact).toMatchObject({
      labelKey: 'print.factPair',
      vars: { label: 'Collected', share: '99%', rest: 'Receivable' },
    })
  })

  it('reads a progress ring against its own maximum', () => {
    const fact = narrativeFact(
      panelWith({ kind: 'radial', radial: { mode: 'progress', max: 200 } }),
      frameWith([['Covered', 150]]),
      'en',
    )

    expect(fact).toMatchObject({ labelKey: 'print.factProgress', vars: { value: '150', max: '200', share: '75%' } })
  })

  it('walks a bridge from opening to result and names the largest deduction', () => {
    const fact = narrativeFact(
      panelWith({ kind: 'cascade', semantics: 'reconciliation', encoding: { label: 'label', value: 'value', final: 'final' } }),
      frameWith(
        [['Earned', 100, false], ['Acquisition', -30, false], ['Claims', -45, false], ['Result', 25, true]],
        ['final'],
      ),
    'en',
    )

    expect(fact).toMatchObject({
      labelKey: 'print.factBridge',
      vars: { start: '100', end: '25', label: 'Claims', amount: '-45', share: '45%' },
    })
  })

  it('reports only the ends when a bridge has no deduction', () => {
    const fact = narrativeFact(
      panelWith({ kind: 'cascade', semantics: 'reconciliation' }),
      frameWith([['Opening', 100], ['Closing', 140]]),
      'en',
    )

    expect(fact).toMatchObject({ labelKey: 'print.factBridgeEnds', vars: { start: '100', end: '140' } })
  })

  it('reads a series from first to last and marks its peak', () => {
    const fact = narrativeFact(
      panelWith({ kind: 'line', semantics: 'series' }),
      frameWith([['2023', 100], ['2024', 180], ['2025', 120]]),
      'en',
    )

    expect(fact).toMatchObject({
      labelKey: 'print.factSeries',
      vars: { first: '100', last: '120', change: '+20%', peak: '180', peakLabel: '2024' },
    })
  })

  it('does not read a ranked bar chart as a progression', () => {
    const fact = narrativeFact(
      panelWith({ kind: 'bar', semantics: 'series' }),
      frameWith([['Motor', 180], ['Property', 90], ['Cargo', 20]]),
      'en',
    )

    expect(fact).toMatchObject({ labelKey: 'print.factTable', vars: { rows: 3, label: 'Motor' } })
  })

  it('counts table rows and names the largest', () => {
    const fact = narrativeFact(
      panelWith({ kind: 'table', semantics: 'evidence' }),
      frameWith([['Motor', 4], ['Property', 9]]),
      'en',
    )

    expect(fact).toMatchObject({ labelKey: 'print.factTable', vars: { rows: 2, label: 'Property', value: '9' } })
  })

  it('says nothing about an empty frame or a single share', () => {
    expect(narrativeFact(panelWith({}), frameWith([]), 'en')).toBeUndefined()
    expect(narrativeFact(panelWith({}), frameWith([['Motor', 60]]), 'en')).toBeUndefined()
    expect(narrativeFact(panelWith({}), undefined, 'en')).toBeUndefined()
  })

  it('ignores negative and zero slices when weighing shares', () => {
    const fact = narrativeFact(
      panelWith({}),
      frameWith([['Motor', 60], ['Adjustment', -10], ['Property', 40], ['Empty', 0]]),
      'en',
    )

    expect(fact).toMatchObject({ labelKey: 'print.factPair', vars: { label: 'Motor', share: '60%' } })
  })
})

describe('headlineReadings', () => {
  const headlinePanel = (title: string, total?: number): Panel => ({
    id: title,
    kind: 'donut',
    title,
    semantics: 'partition',
    frame: 'f',
    encoding: { label: 'label', value: 'value' },
    format: { value: { kind: 'number', minorUnits: false } },
    actions: [],
    ...(total === undefined ? {} : { total }),
  })

  it('takes a figure’s single value, or the total it is measured against', () => {
    const single: Frame = {
      columns: [{ name: 'label', type: 'string' }, { name: 'value', type: 'number' }],
      rows: [['Убыточность', 2.8]],
    }
    const many: Frame = {
      columns: [{ name: 'label', type: 'string' }, { name: 'value', type: 'number' }],
      rows: [['A', 1], ['B', 2]],
    }

    const readings = headlineReadings([
      { id: 'one', panel: headlinePanel('Убыточность'), frame: single },
      { id: 'two', panel: headlinePanel('Премия', 92), frame: many },
    ], 'ru')

    expect(readings.map(({ label, value }) => `${label} ${value}`)).toEqual(['Убыточность 2,8', 'Премия 92'])
  })

  it('says nothing when a chapter has only one reading to state', () => {
    const frame: Frame = {
      columns: [{ name: 'label', type: 'string' }, { name: 'value', type: 'number' }],
      rows: [['Убыточность', 2.8]],
    }

    expect(headlineReadings([{ id: 'one', panel: headlinePanel('Убыточность'), frame }], 'ru')).toEqual([])
  })
})

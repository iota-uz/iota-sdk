import { describe, expect, it } from 'vitest'
import type { AutomationGraph, EditorCatalog } from '@iota-uz/sdk/automation-editor'
import { catalog as editorCatalog } from '../../../../../web/automation-editor/fixture/catalog'
import { jevNode, routeGraph } from './fixtures/automations/backtests'
import {
  active, backtestErrorText, classifiersOf, eventSummary, filterMismatches, nodeTitles, percent, progress, readBacktestsView, writeBacktestsView,
  type ReportResult,
} from './backtestsModel'
import { translator } from './model'

const t = translator((k) => ({
  'Automations.Graphs.Backtests.Errors.LimitDatasets': 'At most 20 datasets',
  'Automations.Graphs.Backtests.Errors.HumanRequired': 'Only a person',
  'Automations.Graphs.Errors.Generic': 'Could not {action}.',
}[k] ?? k))
const catalog = { ...editorCatalog, nodes: [...editorCatalog.nodes, jevNode] } as unknown as EditorCatalog
const graph = routeGraph(3) as unknown as AutomationGraph

describe('classifiers of a graph', () => {
  it('lists the decision node with the outcomes of its choices, first', () => {
    const cs = classifiersOf(graph, catalog, ['intent'], 'en')
    expect(cs[0]).toMatchObject({ id: 'intent', decision: true, outcomes: ['sales', 'support', 'refund', 'low_confidence', 'failed'] })
    // The sends have one outcome besides failed: nothing to label.
    expect(cs.map((c) => c.id)).toEqual(['intent'])
  })
  it('names nodes whose title and subtitle collide by their ids', () => {
    const twin = { ...graph, nodes: [...graph.nodes, { id: 'to_sales_2', type: 'core.send_email', params: { to: 'sales@acme.test', subject: 'Lead', body: 'x' } }] } as AutomationGraph
    const titles = nodeTitles(twin, catalog, 'en')
    const names = ['to_sales', 'to_sales_2'].map((id) => titles.get(id))
    expect(new Set(names).size).toBe(2)
    expect(names[1]).toContain('(to_sales_2)')
  })
  it('tells two nodes of one name apart (C3)', () => {
    const titles = nodeTitles(graph, catalog, 'en')
    expect(titles.get('to_sales')).not.toEqual(titles.get('to_support'))
    expect(titles.get('to_sales')).toContain('Send email')
  })
})

describe('the section in the URL (D1)', () => {
  it('reads and writes the open backtest and its filters, keeping other parameters', () => {
    const v = readBacktestsView('?backtest=b1&node=intent&mismatch=1&x=1')
    expect(v).toEqual({ dataset: null, backtest: 'b1', compare: null, node: 'intent', mismatch: true })
    expect(writeBacktestsView('https://app.test/automations/a/backtests?x=1&node=old', { ...v, node: null, mismatch: false, compare: 'b0' }))
      .toBe('/automations/a/backtests?x=1&backtest=b1&compare=b0')
  })
})

describe('progress and numbers', () => {
  it('measures the progress and tells a job in progress', () => {
    expect(progress({ done: 3, total: 6, status: 'running' })).toBe(0.5)
    expect(progress({ done: 0, total: 0, status: 'queued' })).toBe(0)
    expect(progress({ done: 5, total: 6, status: 'done' })).toBe(1)
    expect(active('queued')).toBe(true)
    expect(active('cancelled')).toBe(false)
  })
  it('writes rates as percents of the language, an undefined one as a dash', () => {
    expect(percent(0.5, 'en')).toBe('50%')
    expect(percent(null, 'en')).toBe('—')
    expect(percent(1 / 3, 'ru')).toMatch(/33,3\s?%/)
  })
})

describe('refusals in words (C1, C2)', () => {
  it('maps the codes of the backtests and falls back to the generic phrase', () => {
    expect(backtestErrorText(t, { code: 'limit_datasets', message: 'x' }, 'create')).toBe('At most 20 datasets')
    expect(backtestErrorText(t, { code: 'HUMAN_REQUIRED', message: 'x' }, 'run')).toBe('Only a person')
    expect(backtestErrorText(t, { code: 'weird', message: 'x' }, 'run it')).toBe('Could not run it.')
  })
})

describe('events and mismatches', () => {
  it('summarizes an event by its title, else its text fields', () => {
    expect(eventSummary({ title: 'Asks for a refund', event: {} })).toBe('Asks for a refund')
    expect(eventSummary({ title: '', event: { kind: 'telegram.message.v1', text: 'Hello', from: 'ann', by_self: false } })).toBe('Hello · ann')
  })
  it('filters the mismatches of a report by classifier', () => {
    const r = { mismatches: [{ node: 'intent' }, { node: 'tone' }] } as unknown as ReportResult
    expect(filterMismatches(r, 'tone')).toHaveLength(1)
    expect(filterMismatches(r, null)).toHaveLength(2)
  })
})

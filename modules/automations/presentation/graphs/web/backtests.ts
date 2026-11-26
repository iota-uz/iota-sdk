import { describeItem, nodeOutcomes, type AutomationGraph, type EditorCatalog } from '@iota-uz/sdk/automation-editor'
import type { RPC } from './Automations.solid.generated'
import { editorLocale, errorCode, errorText, intlTag, type Translator } from './model'

export type BacktestsResult = RPC['automations.backtests']['result']
export type DatasetView = BacktestsResult['datasets'][number]
export type BacktestView = BacktestsResult['backtests'][number]
export type ImportView = DatasetView['imports'][number]
export type SourceView = BacktestsResult['sources'][number]
export type ExamplesResult = RPC['automations.dataset_examples']['result']
export type ExampleView = ExamplesResult['items'][number]
export type ReportResult = RPC['automations.backtest_report']['result']
export type ClassifierView = ReportResult['classifiers'][number]
export type ResultsResult = RPC['automations.backtest_results']['result']
export type ResultView = ResultsResult['items'][number]
export type CompareResult = RPC['automations.backtest_compare']['result']
export type EstimateResult = RPC['automations.backtest_estimate']['result']
export type JobStatus = BacktestView['status']

/** A node of the graph that a label may name: its title by meaning (C3) and the outcomes it may take. */
export interface Classifier {
  id: string
  title: string
  outcomes: string[]
  /** The node's answer is a decision of a model (a classifier, an LLM). */
  decision: boolean
}

/**
 * The names of a graph's nodes as the canvas gives them (C3); two nodes of one name are told apart by
 * their subtitle (an email's subject), else by their id.
 */
export function nodeTitles(graph: AutomationGraph, catalog: EditorCatalog, lang: string): Map<string, string> {
  const names = graph.nodes.map((n) => ({ id: n.id, ...describeItem(graph, catalog, n.id, editorLocale(lang)) }))
  const count = new Map<string, number>()
  for (const n of names) count.set(n.title, (count.get(n.title) ?? 0) + 1)
  const out = new Map<string, string>()
  for (const n of names) {
    const title = n.title || n.id
    out.set(n.id, (count.get(n.title) ?? 0) > 1 ? `${title} · ${n.subtitle && n.subtitle !== n.id ? n.subtitle : n.id}` : title)
  }
  // A subtitle that does not tell them apart either: the final names that still collide take the id.
  const final = new Map<string, number>()
  for (const v of out.values()) final.set(v, (final.get(v) ?? 0) + 1)
  for (const [id, v] of out) if ((final.get(v) ?? 0) > 1) out.set(id, `${v} (${id})`)
  return out
}

/**
 * The nodes an author labels: the decision nodes (as the server names them) and every node with more
 * than one outcome besides failed, decision nodes first.
 */
export function classifiersOf(graph: AutomationGraph, catalog: EditorCatalog, decisionNodes: string[], lang: string): Classifier[] {
  const titles = nodeTitles(graph, catalog, lang)
  const out: Classifier[] = []
  for (const node of graph.nodes) {
    const spec = catalog.nodes.find((n) => n.type === node.type)
    const outcomes = nodeOutcomes(node, spec)
    const meaningful = outcomes.filter((o) => o !== 'failed')
    const decision = decisionNodes.includes(node.id)
    if (!decision && meaningful.length < 2) continue
    out.push({ id: node.id, title: titles.get(node.id) ?? node.id, outcomes, decision })
  }
  return out.sort((a, b) => Number(b.decision) - Number(a.decision) || a.title.localeCompare(b.title))
}

/** The title of a node by its id, else the id itself (a node of another version). */
export function nodeTitle(titles: Map<string, string> | undefined, id: string): string {
  return titles?.get(id) ?? id
}

/** A rate as a percent of the language, «—» when undefined. */
export function percent(v: number | null | undefined, lang: string): string {
  if (v === null || v === undefined || !Number.isFinite(v)) return '—'
  return new Intl.NumberFormat(intlTag(lang), { style: 'percent', maximumFractionDigits: 1 }).format(v)
}

/** An amount of US dollars; small amounts keep their cents' fractions. */
export function usd(v: number, lang: string): string {
  return new Intl.NumberFormat(intlTag(lang), { style: 'currency', currency: 'USD', maximumFractionDigits: v > 0 && v < 1 ? 4 : 2 }).format(v)
}

export function integer(v: number, lang: string): string {
  return new Intl.NumberFormat(intlTag(lang)).format(v)
}

/** A job still in progress: the screen polls it. */
export function active(status: JobStatus): boolean {
  return status === 'queued' || status === 'running'
}

/** The progress of a backtest from 0 to 1. */
export function progress(b: Pick<BacktestView, 'done' | 'total' | 'status'>): number {
  if (b.status === 'done') return 1
  if (b.total <= 0) return 0
  return Math.min(1, b.done / b.total)
}

/** What the section shows, kept in the URL (D1): the open dataset or backtest and the report's filters. */
export interface BacktestsView {
  dataset: string | null
  backtest: string | null
  compare: string | null
  node: string | null
  mismatch: boolean
}

export function readBacktestsView(search: string): BacktestsView {
  const p = new URLSearchParams(search)
  return {
    dataset: p.get('dataset'),
    backtest: p.get('backtest'),
    compare: p.get('compare'),
    node: p.get('node'),
    mismatch: p.get('mismatch') === '1',
  }
}

export function writeBacktestsView(href: string, v: BacktestsView): string {
  const url = new URL(href)
  for (const k of ['dataset', 'backtest', 'compare', 'node', 'mismatch']) url.searchParams.delete(k)
  if (v.dataset) url.searchParams.set('dataset', v.dataset)
  if (v.backtest) url.searchParams.set('backtest', v.backtest)
  if (v.compare) url.searchParams.set('compare', v.compare)
  if (v.node) url.searchParams.set('node', v.node)
  if (v.mismatch) url.searchParams.set('mismatch', '1')
  return url.pathname + url.search
}

/** The path of the section of a graph. */
export function backtestsPath(base: string, id: string): string {
  return `${base}/${id}/backtests`
}

/** A refusal of the backtests' RPC in words (C1, C2). */
export function backtestErrorText(t: Translator, error: unknown, action: string): string {
  switch (errorCode(error)) {
    case 'backtests_unavailable': return t('Backtests.Errors.Unavailable')
    case 'limit_datasets': return t('Backtests.Errors.LimitDatasets')
    case 'limit_examples': return t('Backtests.Errors.LimitExamples')
    case 'limit_active': return t('Backtests.Errors.LimitActive')
    case 'empty_dataset': return t('Backtests.Errors.EmptyDataset')
    case 'unsupported_source': return t('Backtests.Errors.UnsupportedSource')
    case 'backtest_finished': return t('Backtests.Errors.Finished')
    case 'estimate_stale': return t('Backtests.Errors.EstimateStale')
    case 'HUMAN_REQUIRED': return t('Backtests.Errors.HumanRequired')
    case 'forbidden': return t('Backtests.Errors.Forbidden')
  }
  return errorText(t, error, action)
}

/** Why an import failed, in words: the server's reason is for the log only (C1). */
export function importErrorText(t: Translator, error: string | null | undefined): string {
  const e = (error ?? '').toLowerCase()
  if (e.includes('right') || e.includes('forbidden') || e.includes('required')) return t('Backtests.Import.FailedRights')
  if (e.includes('connector') || e.includes('unsupported')) return t('Backtests.Import.FailedSource')
  return t('Backtests.Import.Failed')
}

/** Why a backtest failed, in words. */
export function backtestFailureText(t: Translator, error: string | null | undefined): string {
  const e = (error ?? '').toLowerCase()
  if (e.includes('validate')) return t('Backtests.Run.FailedInvalid')
  if (e.includes('forbidden') || e.includes('required') || e.includes('not available')) return t('Backtests.Run.FailedRights')
  return t('Backtests.Run.Failed')
}

/** A short summary of an example's event: its title, else the first text fields of its payload. */
export function eventSummary(ex: Pick<ExampleView, 'title' | 'event'>): string {
  if (ex.title) return ex.title
  const parts: string[] = []
  const walk = (v: unknown, depth: number) => {
    if (parts.length >= 3 || depth > 2) return
    if (typeof v === 'string' && v.trim()) parts.push(v.trim())
    else if (typeof v === 'number' || typeof v === 'boolean') parts.push(String(v))
    else if (v && typeof v === 'object' && !Array.isArray(v)) {
      for (const [k, x] of Object.entries(v)) if (k !== 'kind' && k !== 'by_self' && k !== 'connectionId') walk(x, depth + 1)
    }
  }
  walk(ex.event, 0)
  const s = parts.join(' · ')
  return s.length > 140 ? `${s.slice(0, 139)}…` : s
}

/** Classifier rows of a report, for the metrics table: one per outcome, the classifier first. */
export function outcomeRows(c: ClassifierView): { outcome: string; precision: number | null; recall: number | null; support: number; tp: number; fp: number; fn: number }[] {
  return c.outcomes.map((o) => ({ outcome: o.outcome, precision: o.precision, recall: o.recall, support: o.support, tp: o.tp, fp: o.fp, fn: o.fn }))
}

/** The mismatches of a report under a classifier filter (all when node is null). */
export function filterMismatches(r: ReportResult, node: string | null): ReportResult['mismatches'] {
  return node ? r.mismatches.filter((m) => m.node === node) : r.mismatches
}

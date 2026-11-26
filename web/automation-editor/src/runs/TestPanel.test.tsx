import { fireEvent, render, waitFor } from '@solidjs/testing-library'
import { describe, expect, it, vi } from 'vitest'
import { catalog } from '../../fixture/catalog'
import { sample } from '../../fixture/graphs'
import type { EditorApi, RunData, RunsQuery, RunSummary } from '../types'
import { TestPanel } from './TestPanel'

const at = '2026-11-20T09:00:00Z'
const runOf = (id: string, reached: boolean, hidden = false): RunData => ({
  run_id: id, version: 3, state: 'done', subject: null, trigger: 'user_created', error: null, created_at: at, updated_at: at,
  tokens: [], waits: [],
  steps: reached ? [{ node: 'send_email', token_id: 't', attempt: 1, input: {}, outcome: 'sent', output: {}, logs: [], error: null, started_at: at, ended_at: at, ...(hidden ? { hidden: true } : {}) }] : [],
  event: { kind: 'core.user.created.v1', user: { id: Number(id.slice(1)), email: `${id}@acme.test` } },
})

describe('TestPanel «From the last run»', () => {
  it('asks the server for runs that reached the step and follows the cursor past ones it may not read', async () => {
    // Page one: five runs whose step at the node is hidden from the author; page two: one it may read.
    const pages: Record<string, { items: RunData[]; next: string | null }> = {
      first: { items: ['r1', 'r2', 'r3', 'r4', 'r5'].map((id) => runOf(id, true, true)), next: 'page-two' },
      'page-two': { items: [runOf('r7', true)], next: null },
    }
    const byId = new Map(Object.values(pages).flatMap((p) => p.items).map((r) => [r.run_id, r]))
    const runs = vi.fn(async (q: RunsQuery) => {
      const page = pages[q.cursor ?? 'first']!
      const items: RunSummary[] = page.items.map(({ tokens: _t, steps: _s, waits: _w, event: _e, ...rest }) => rest)
      return { items, next_cursor: page.next }
    })
    const api: EditorApi = {
      validate: vi.fn(), dryRun: vi.fn(), testNode: vi.fn(async () => ({ outcome: 'sent' })),
      runs, run: vi.fn(async (id: string) => byId.get(id)!),
    }
    const { getByTestId, container } = render(() => (
      <TestPanel graph={sample} catalog={catalog} api={api} locale="en" node="send_email" automationId="a-1" cache={new Map()} onOverlay={() => undefined} onClose={() => undefined} />
    ))
    fireEvent.click(getByTestId('test-example'))
    await waitFor(() => expect(container.querySelector<HTMLInputElement>('input[type="number"]')?.value).toBe('7'))
    expect(runs).toHaveBeenCalledTimes(2)
    expect(runs.mock.calls[0]![0]).toMatchObject({ automation_id: 'a-1', node: 'send_email' })
    expect(runs.mock.calls[1]![0]).toMatchObject({ node: 'send_email', cursor: 'page-two' })
  })

  it('reads the newest matching run with the step of its node, even when more steps came after it than a detail holds', async () => {
    // The newest run reached the step, then took over a thousand more steps: its plain detail (the latest
    // steps) does not hold the step; the read that names the node does. The older run must not win.
    const newer = runOf('r9', true)
    const older = runOf('r3', true)
    const runs = vi.fn(async () => ({ items: [newer, older].map(({ tokens: _t, steps: _s, waits: _w, event: _e, ...rest }) => rest as RunSummary), next_cursor: null }))
    const run = vi.fn(async (id: string, opts?: { nodes?: string[] }) => {
      const r = id === 'r9' ? newer : older
      if (id === 'r9' && !opts?.nodes?.includes('send_email')) return { ...r, steps: [] } // capped
      return r
    })
    const api: EditorApi = { validate: vi.fn(), dryRun: vi.fn(), testNode: vi.fn(async () => ({ outcome: 'sent' })), runs, run }
    const { getByTestId, container } = render(() => (
      <TestPanel graph={sample} catalog={catalog} api={api} locale="en" node="send_email" automationId="a-1" cache={new Map()} onOverlay={() => undefined} onClose={() => undefined} />
    ))
    fireEvent.click(getByTestId('test-example'))
    await waitFor(() => expect(container.querySelector<HTMLInputElement>('input[type="number"]')?.value).toBe('9'))
    expect(run).toHaveBeenCalledTimes(1)
    expect(run.mock.calls[0]![1]?.nodes).toEqual(expect.arrayContaining(['send_email', 'has_email']))
  })

  it('a later visit the author may not read does not hide the run: its readable visit is the example', async () => {
    // The newest run visited the node with readable data, then again after a field the author may not
    // read: its read with the node gives both visits (the later one hidden). It wins over the older run.
    const newer: RunData = { ...runOf('r20', true), steps: [...runOf('r20', true).steps, { ...runOf('r20', true).steps[0]!, attempt: 2, hidden: true, output: null, started_at: '2026-11-20T09:05:00Z' }] }
    const older = runOf('r10', true)
    const runs = vi.fn(async () => ({ items: [newer, older].map(({ tokens: _t, steps: _s, waits: _w, event: _e, ...rest }) => rest as RunSummary), next_cursor: null }))
    const run = vi.fn(async (id: string) => (id === 'r20' ? newer : older))
    const api: EditorApi = { validate: vi.fn(), dryRun: vi.fn(), testNode: vi.fn(async () => ({ outcome: 'sent' })), runs, run }
    const { getByTestId, container } = render(() => (
      <TestPanel graph={sample} catalog={catalog} api={api} locale="en" node="send_email" automationId="a-1" cache={new Map()} onOverlay={() => undefined} onClose={() => undefined} />
    ))
    fireEvent.click(getByTestId('test-example'))
    await waitFor(() => expect(container.querySelector<HTMLInputElement>('input[type="number"]')?.value).toBe('20'))
  })

  it('reads the outputs of steps far before the node in further batches', async () => {
    // A chain of 60 sends before the tested one: the first read names 50 nodes, the rest come in a second.
    const chain = Array.from({ length: 60 }, (_, i) => `s${i}`)
    const graph = {
      ...sample,
      nodes: [...sample.nodes, ...chain.map((id) => ({ id, type: 'core.send_email', params: { to: 'a', subject: 'b', body: 'c' } }))],
      edges: [
        { from: { node: 'user_created', outcome: 'fired' }, to: 's0' },
        ...chain.slice(1).map((id, i) => ({ from: { node: chain[i]!, outcome: 'sent' }, to: id })),
        { from: { node: 's59', outcome: 'sent' }, to: 'send_email' },
      ],
    }
    const step = (node: string, minute: number) => ({ node, token_id: 't', attempt: 1, input: {}, outcome: 'sent', output: { message_id: `m-${node}` }, logs: [], error: null, started_at: `2026-11-20T09:${String(minute).padStart(2, '0')}:00Z`, ended_at: at })
    const full: RunData = { ...runOf('r1', true), steps: [...chain.map((id, i) => step(id, i % 60)), step('send_email', 59)] }
    const run = vi.fn(async (_id: string, opts?: { nodes?: string[] }) => ({ ...full, steps: full.steps.filter((s) => opts?.nodes?.includes(s.node)) }))
    const runs = vi.fn(async () => ({ items: [full].map(({ tokens: _t, steps: _s, waits: _w, event: _e, ...rest }) => rest as RunSummary), next_cursor: null }))
    const api: EditorApi = { validate: vi.fn(), dryRun: vi.fn(), testNode: vi.fn(async () => ({ outcome: 'sent' })), runs, run }
    const { getByTestId, container } = render(() => (
      <TestPanel graph={graph} catalog={catalog} api={api} locale="en" node="send_email" automationId="a-1" cache={new Map()} onOverlay={() => undefined} onClose={() => undefined} />
    ))
    fireEvent.click(getByTestId('test-example'))
    await waitFor(() => expect(run).toHaveBeenCalledTimes(2))
    expect(run.mock.calls[0]![1]!.nodes).toHaveLength(50)
    expect(run.mock.calls[1]![1]!.nodes).toContain('s0')
    const field = (path: string) => [...container.querySelectorAll<HTMLInputElement | HTMLTextAreaElement>(`[data-param="${path}"] input, [data-param="${path}"] textarea`)].map((i) => i.value)
    await waitFor(() => expect(field('nodes.s0.output.message_id')).toContain('m-s0'))
    expect(field('nodes.s59.output.message_id')).toContain('m-s59')
  })
})

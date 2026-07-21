import { act, render, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { ChartAdapter, ChartEvents, ChartInput, ChartInstance } from '../charts/adapter'

const charts = vi.hoisted(() => ({
  getChartAdapter: vi.fn<() => Promise<ChartAdapter>>(),
}))

vi.mock('../charts', () => ({ getChartAdapter: charts.getChartAdapter }))

import { ChartHost } from './ChartHost'

const input = (rows: Array<Array<unknown>>): ChartInput => ({
  kind: 'bar',
  frame: {
    columns: [{ name: 'id', type: 'string' }, { name: 'value', type: 'number' }],
    rows,
  },
  encoding: { id: 'id', value: 'value' },
  format: (_field, value) => String(value),
  theme: { palette: {}, series: {} },
})

function deferred<T>() {
  let settle: ((value: T) => void) | undefined
  const promise = new Promise<T>((resolve) => {
    settle = resolve
  })
  return {
    promise,
    resolve(value: T) {
      settle?.(value)
    },
  }
}

afterEach(() => {
  charts.getChartAdapter.mockReset()
  vi.restoreAllMocks()
})

describe('ChartHost', () => {
  it('mounts, updates, forwards current events, and disposes an injected adapter', async () => {
    let select: ((key: string) => void) | undefined
    const update = vi.fn<(next: ChartInput) => void>()
    const dispose = vi.fn()
    const mount = vi.fn<(el: HTMLElement, initial: ChartInput, events: ChartEvents) => ChartInstance>((_el, _input, events) => {
      select = (key) => events.onSelect(key)
      return { update, dispose }
    })
    const adapter: ChartAdapter = { mount }
    const firstSelect = vi.fn()
    const secondSelect = vi.fn()
    const view = render(<ChartHost input={input([['a', 1]])} adapter={adapter} onSelect={firstSelect} />)
    await waitFor(() => expect(mount).toHaveBeenCalledTimes(1))

    view.rerender(<ChartHost input={input([['b', 2]])} adapter={adapter} onSelect={secondSelect} />)
    expect(update.mock.calls.at(-1)?.[0].frame.rows).toEqual([['b', 2]])
    select?.('b')
    expect(firstSelect).not.toHaveBeenCalled()
    // The anchor is optional context for overlay placement; the key is the contract.
    expect(secondSelect).toHaveBeenCalledWith('b', undefined)

    view.unmount()
    expect(dispose).toHaveBeenCalledTimes(1)
  })

  it('aborts a lazy mount when unmounted before the adapter resolves', async () => {
    const pending = deferred<ChartAdapter>()
    const dispose = vi.fn()
    const mount = vi.fn<ChartAdapter['mount']>(() => ({ update: vi.fn(), dispose }))
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    charts.getChartAdapter.mockReturnValue(pending.promise)
    const view = render(<ChartHost input={input([['a', 1]])} />)
    await waitFor(() => expect(charts.getChartAdapter).toHaveBeenCalledTimes(1))

    view.unmount()
    await act(async () => {
      pending.resolve({ mount })
      await pending.promise
    })

    expect(mount).not.toHaveBeenCalled()
    expect(dispose).not.toHaveBeenCalled()
    expect(consoleError).not.toHaveBeenCalled()
    consoleError.mockRestore()
  })

  it('mounts an in-flight adapter with the latest input', async () => {
    const pending = deferred<ChartAdapter>()
    const mount = vi.fn<ChartAdapter['mount']>(() => ({ update: vi.fn(), dispose: vi.fn() }))
    charts.getChartAdapter.mockReturnValue(pending.promise)
    const view = render(<ChartHost input={input([['old', 1]])} />)
    await waitFor(() => expect(charts.getChartAdapter).toHaveBeenCalledTimes(1))

    view.rerender(<ChartHost input={input([['latest', 2]])} />)
    await act(async () => {
      pending.resolve({ mount })
      await pending.promise
    })

    expect(mount).toHaveBeenCalledTimes(1)
    expect(mount.mock.calls[0]?.[1].frame.rows).toEqual([['latest', 2]])
  })

  it('loads and mounts the default chart adapter', async () => {
    const mount = vi.fn<ChartAdapter['mount']>(() => ({ update: vi.fn(), dispose: vi.fn() }))
    charts.getChartAdapter.mockResolvedValue({ mount })

    render(<ChartHost input={input([['a', 1]])} />)

    await waitFor(() => expect(mount).toHaveBeenCalledTimes(1))
    expect(charts.getChartAdapter).toHaveBeenCalledTimes(1)
  })
})

import { render, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { ChartAdapter, ChartEvents, ChartInput, ChartInstance } from '../charts/adapter'
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
    expect(secondSelect).toHaveBeenCalledWith('b')

    view.unmount()
    expect(dispose).toHaveBeenCalledTimes(1)
  })
})

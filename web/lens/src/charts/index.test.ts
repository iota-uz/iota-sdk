import { describe, expect, it, vi } from 'vitest'
import type { ChartAdapter } from './adapter'
import { resolveChartAdapter } from './index'

describe('chart adapter loading', () => {
  it('installs optional kind modules before exposing the adapter for mount', async () => {
    let finishPreparing: (() => void) | undefined
    const prepareEChartsKind = vi.fn(() => new Promise<void>((resolve) => { finishPreparing = resolve }))
    const echartsAdapter = { mount: vi.fn() } as unknown as ChartAdapter
    const resolved = resolveChartAdapter('map', () => Promise.resolve({ echartsAdapter, prepareEChartsKind }))
    let exposed = false
    void resolved.then(() => { exposed = true })

    await vi.waitFor(() => expect(prepareEChartsKind).toHaveBeenCalledWith('map'))
    expect(exposed).toBe(false)

    finishPreparing?.()
    await expect(resolved).resolves.toBe(echartsAdapter)
    expect(exposed).toBe(true)
  })
})

import type { DashboardDocument } from '../contract'
import { PanelClient } from './panel'

interface PrefetchDocumentPanelsOptions {
  csrf?: string
  fetcher?: typeof fetch
  signal?: AbortSignal
}

/**
 * Carry useful child data all the way into the browser cache. Warming just the
 * first content row gives the user immediate figures while preserving a
 * bounded one-surface budget; lower rows remain progressive after open.
 *
 * This module is loaded only after a speculative document has arrived. It must
 * stay outside the critical root bundle: child warm-up cannot tax first paint.
 */
export async function prefetchDocumentFirstRow(
  document: DashboardDocument,
  options: PrefetchDocumentPanelsOptions,
): Promise<DashboardDocument> {
  const endpoint = document.endpoints.panel
  const firstRow = document.layout.rows.find(({ panels }) => panels.length > 0)
  if (!endpoint || !firstRow || options.signal?.aborted) return document
  const ids = new Set(firstRow.panels.map(({ panelId }) => panelId))
  const candidates = document.panels.filter(({ id, deferred }) => deferred === true && ids.has(id))
  if (candidates.length === 0) return document

  const client = new PanelClient(endpoint, { csrf: options.csrf, fetcher: options.fetcher, prefetch: true })
  try {
    const results = await client.loadBatch(candidates.map(({ id }) => ({
      snapshotId: document.snapshotId,
      panelId: id,
    })), { signal: options.signal })
    const frames = { ...document.frames }
    const hydrated = new Set<string>()
    for (const panel of candidates) {
      const result = results[panel.id]
      if (!result?.frames || result.error) continue
      const loaded = result.frames[panel.frame] ?? Object.values(result.frames)[0]
      if (!loaded) continue
      frames[panel.frame] = loaded
      hydrated.add(panel.id)
    }
    if (hydrated.size === 0) return document
    return {
      ...document,
      frames,
      panels: document.panels.map((panel) => hydrated.has(panel.id) ? { ...panel, deferred: false } : panel),
    }
  } finally {
    client.dispose()
  }
}

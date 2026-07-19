import { parseDocument, type DashboardDocument } from '../contract'

export interface DocumentRequestOptions {
  csrf?: string
  signal?: AbortSignal
  fetcher?: typeof fetch
}

export async function fetchDocument(src: string, options: DocumentRequestOptions = {}): Promise<DashboardDocument> {
  const response = await (options.fetcher ?? fetch)(src, {
    credentials: 'same-origin',
    headers: options.csrf ? { 'X-CSRF-Token': options.csrf } : undefined,
    signal: options.signal,
  })
  if (!response.ok) throw new Error(`document request failed with ${response.status}`)
  return parseDocument(await response.json())
}

export class ExportError extends Error {
  constructor(message: string, readonly status: number) {
    super(message)
    this.name = 'ExportError'
  }
}

export class ExportSnapshotGoneError extends ExportError {
  constructor(message = 'snapshot is unknown or expired') {
    super(message, 410)
    this.name = 'ExportSnapshotGoneError'
  }
}

export interface ExportWorkbookRequest {
  endpoint: string
  snapshotId: string
  panelId?: string
  csrf?: string
  fetcher?: typeof fetch
}

export interface ExportWorkbook {
  blob: Blob
  filename: string
}

function responseFilename(header: string | null): string | undefined {
  if (!header) return undefined
  const encoded = /filename\*=UTF-8''([^;]+)/i.exec(header)?.[1]
  if (encoded) {
    try {
      return decodeURIComponent(encoded)
    } catch {
      // Fall through to the legacy filename parameter.
    }
  }
  return /filename="([^"]+)"/i.exec(header)?.[1]
}

async function responseMessage(response: Response): Promise<string> {
  try {
    const payload: unknown = await response.json()
    if (payload && typeof payload === 'object' && 'message' in payload && typeof payload.message === 'string') {
      return payload.message
    }
  } catch {
    // The status remains enough context when the response is not JSON.
  }
  return `export request failed with ${response.status}`
}

export async function exportWorkbook(request: ExportWorkbookRequest): Promise<ExportWorkbook> {
  const url = new URL(request.endpoint, globalThis.location?.href ?? 'http://localhost')
  url.searchParams.set('snapshot', request.snapshotId)
  if (request.panelId) url.searchParams.set('panel', request.panelId)
  else url.searchParams.delete('panel')

  const response = await (request.fetcher ?? fetch)(url, {
    method: 'GET',
    credentials: 'same-origin',
    headers: request.csrf ? { 'X-CSRF-Token': request.csrf } : undefined,
  })
  if (!response.ok) {
    const message = await responseMessage(response)
    if (response.status === 410) throw new ExportSnapshotGoneError(message)
    throw new ExportError(message, response.status)
  }
  const filename = responseFilename(response.headers.get('Content-Disposition'))
  if (!filename) throw new ExportError('Export response did not include a filename', response.status)
  return { blob: await response.blob(), filename }
}

export function downloadWorkbook(workbook: ExportWorkbook): void {
  const url = URL.createObjectURL(workbook.blob)
  const link = document.createElement('a')
  link.href = url
  link.download = workbook.filename
  link.hidden = true
  document.body.append(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}

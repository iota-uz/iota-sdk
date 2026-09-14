import { createSignal, For, onCleanup, onMount, Show, splitProps, type JSX } from 'solid-js'
import { Spinner } from '../display/Spinner'
import { classes } from '../internal/classes'

export type ExportFormat = 'excel' | 'csv' | 'json' | 'txt'
export type ExportButtonSize = 'normal' | 'md' | 'sm' | 'xs'

export interface ExportLabels {
  title: string
  preparing: string
  failed: string
  formats: Record<ExportFormat, string>
}

export interface ExportRequest {
  format: ExportFormat
  url: string
  params: URLSearchParams
  signal: AbortSignal
}

export interface ExportFile {
  blob: Blob
  filename?: string
}

export interface ExportDropdownProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'onError'> {
  formats: readonly ExportFormat[]
  exportURL?: string
  paramsFormID?: string
  download?: boolean
  label?: JSX.Element
  size?: ExportButtonSize
  open?: boolean
  defaultOpen?: boolean
  exporting?: boolean
  onOpenChange?: (open: boolean) => void
  onExport?: (request: ExportRequest) => Promise<Response | Blob | ExportFile | void> | Response | Blob | ExportFile | void
  onExportingChange?: (exporting: boolean) => void
  onError?: (error: unknown) => void
  optionProps?: JSX.ButtonHTMLAttributes<HTMLButtonElement>
  labels?: Partial<Omit<ExportLabels, 'formats'>> & { formats?: Partial<Record<ExportFormat, string>> }
}

const defaultLabels: ExportLabels = {
  title: 'Export', preparing: 'Preparing export…', failed: 'Export failed. Please try again.',
  formats: { excel: 'Export to Excel', csv: 'Export to CSV', json: 'Export to JSON', txt: 'Export to TXT' },
}

function DownloadIcon(props: { size: number }) {
  return <svg aria-hidden="true" width={props.size} height={props.size} viewBox="0 0 256 256"><path d="M128 24v136m-48-48 48 48 48-48M40 192v24h176v-24" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16"/></svg>
}

function FileIcon(props: { size: number }) {
  return <svg aria-hidden="true" width={props.size} height={props.size} viewBox="0 0 256 256"><path d="M48 24h104l56 56v152H48Z" fill="none" stroke="currentColor" stroke-linejoin="round" stroke-width="16"/><path d="M152 24v56h56" fill="none" stroke="currentColor" stroke-linejoin="round" stroke-width="16"/></svg>
}

export function collectExportParams(formID?: string, locationSearch = typeof window === 'undefined' ? '' : window.location.search): URLSearchParams {
  const params = new URLSearchParams(locationSearch)
  if (formID) {
    const form = document.getElementById(formID)
    if (!(form instanceof HTMLFormElement)) throw new Error(`ExportDropdown: ParamsFormID element is missing or not a form: ${formID}`)
    const formParams = new URLSearchParams(new FormData(form) as unknown as URLSearchParams)
    const keys = new Set<string>()
    formParams.forEach((_value, key) => keys.add(key))
    keys.forEach((key) => params.delete(key))
    formParams.forEach((value, key) => params.append(key, value))
  }
  for (const key of ['page', 'limit', 'per_page']) params.delete(key)
  return params
}

export function buildExportURL(exportURL: string, format: ExportFormat, params: URLSearchParams): string {
  const next = new URLSearchParams(params)
  next.set('format', format)
  return `${exportURL}${exportURL.includes('?') ? '&' : '?'}${next.toString()}`
}

export function saveExportFile(file: ExportFile): void {
  const url = URL.createObjectURL(file.blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = file.filename ?? 'export'
  document.body.append(anchor)
  anchor.click()
  anchor.remove()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}

export function ExportDropdown(props: ExportDropdownProps) {
  const [local, native] = splitProps(props, ['formats', 'exportURL', 'paramsFormID', 'download', 'label', 'size', 'open', 'defaultOpen', 'exporting', 'onOpenChange', 'onExport', 'onExportingChange', 'onError', 'optionProps', 'labels', 'class'])
  const [internalOpen, setInternalOpen] = createSignal(local.defaultOpen ?? false)
  const [internalExporting, setInternalExporting] = createSignal(false)
  let request: AbortController | undefined
  let alive = true
  const open = () => local.open ?? internalOpen()
  const exporting = () => local.exporting ?? internalExporting()
  const labels = () => ({ ...defaultLabels, ...local.labels, formats: { ...defaultLabels.formats, ...local.labels?.formats } })
  const setOpen = (value: boolean) => { if (local.open === undefined) setInternalOpen(value); local.onOpenChange?.(value) }
  const setExporting = (value: boolean) => { if (local.exporting === undefined) setInternalExporting(value); local.onExportingChange?.(value) }
  const run = async (format: ExportFormat) => {
    if (exporting()) return
    request?.abort()
    const controller = new AbortController()
    request = controller
    try {
      const params = collectExportParams(local.paramsFormID)
      const url = local.exportURL ? buildExportURL(local.exportURL, format, params) : ''
      setOpen(false)
      setExporting(true)
      let result = await local.onExport?.({ format, url, params, signal: controller.signal })
      if (!local.onExport && local.exportURL) result = await fetch(url, { method: local.download ? 'GET' : 'POST', credentials: 'same-origin', signal: controller.signal })
      if (!alive || controller.signal.aborted || request !== controller) return
      if (result instanceof Response) {
        if (!result.ok) throw new Error(`Export request failed: ${result.status}`)
        if (local.download) {
          const blob = await result.blob()
          const disposition = result.headers.get('content-disposition') ?? ''
          const filename = new URLSearchParams(disposition.replace(/;\s*/g, '&')).get('filename') ?? `export.${format === 'excel' ? 'xlsx' : format}`
          saveExportFile({ blob, filename })
        }
      } else if (result instanceof Blob) saveExportFile({ blob: result, filename: `export.${format === 'excel' ? 'xlsx' : format}` })
      else if (result && 'blob' in result) saveExportFile(result)
    } catch (error) { if (alive && !controller.signal.aborted && request === controller) local.onError?.(error) }
    finally { if (alive && !controller.signal.aborted && request === controller) setExporting(false) }
  }
  onMount(() => {
    const escape = (event: KeyboardEvent) => { if (event.key === 'Escape' && open()) setOpen(false) }
    document.addEventListener('keydown', escape)
    onCleanup(() => document.removeEventListener('keydown', escape))
  })
  onCleanup(() => { alive = false; request?.abort() })
  return <div {...native} class={classes('relative', local.class)} data-export-url={local.exportURL} data-params-form={local.paramsFormID}>
    <details class="relative z-10 peer" name="export-dropdown" open={open()} onToggle={(event) => {
      setOpen(event.currentTarget.open)
      if (local.open !== undefined) queueMicrotask(() => { event.currentTarget.open = open() })
    }}>
      <summary class={classes('list-none cursor-pointer shrink-0 btn btn-secondary btn-with-icon flex items-center gap-2', `btn-${local.size ?? 'normal'}`)} aria-busy={exporting()}>
        <Show when={!exporting()} fallback={<Spinner role="presentation" spinnerClass="w-[18px] h-[18px]" />}><span class="flex items-center"><DownloadIcon size={18} /></span></Show>
        {local.label ?? labels().title}<span class="ml-1"><span class="sr-only">Toggle</span>⌄</span>
      </summary>
      <ul class="flex flex-col gap-1 mt-1 absolute bg-surface-300 right-0 text-sm rounded-md w-44 overflow-hidden shadow-sm border border-secondary p-1">
        <For each={local.formats}>{(format) => <li><button {...local.optionProps} type="button" class={classes('flex items-center gap-2 w-full text-left p-2 duration-200 hover:bg-surface-400 rounded-md disabled:opacity-50 disabled:cursor-not-allowed', local.optionProps?.class)} disabled={exporting() || local.optionProps?.disabled} onClick={() => void run(format)}><FileIcon size={16} />{labels().formats[format]}</button></li>}</For>
      </ul>
    </details>
    <Show when={open()}><button type="button" aria-label="Close export menu" class="fixed w-full h-full left-0 top-0" onClick={() => setOpen(false)} /></Show>
    <Show when={exporting()}><div class="fixed bottom-4 right-4 z-50 flex items-center gap-3 bg-surface-300 border border-secondary rounded-lg shadow-lg px-4 py-3 text-sm" role="status"><Spinner role="presentation" spinnerClass="w-5 h-5" /><span>{labels().preparing}</span></div></Show>
  </div>
}

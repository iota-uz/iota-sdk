import { createEffect, createSignal, For, onCleanup, onMount, Show, splitProps, type JSX } from 'solid-js'
import { Card } from '../display/Card'
import { Progress } from '../display/Progress'
import { UploadDropzone, type UploadItem } from '../forms-advanced/Upload'
import { Button } from '../forms/Button'
import { classes } from '../internal/classes'

export interface ImportColumn { header: string; description?: string; required?: boolean }
export type ImportErrorsMap = Record<string, string>
export type ImportRunStatus = 'queued' | 'running' | 'done' | 'failed' | 'cancelled'
export interface ImportCount { label: string; value: number }
export interface ImportResult { dryRun?: boolean; counts?: readonly ImportCount[]; warnings?: readonly string[]; detail?: JSX.Element }
export interface ImportRunState { id?: string; phase?: string; done: number; total: number; status: ImportRunStatus; result?: ImportResult; error?: string }

export interface ImportLabels {
  upload: string
  uploadPlaceholder: string
  downloadTemplate: string
  exampleBelow: string
  validationError: string
  submit: string
  cancel: string
  running: string
  processingFailed: string
  warnings: string
  dryRunNotice: string
  confirm: string
  retry: string
  cancelRun: string
  cancelled: string
  fileRequired: string
}

export const defaultImportLabels: ImportLabels = {
  upload: 'Choose file', uploadPlaceholder: 'Upload an import file', downloadTemplate: 'Download template', exampleBelow: 'Example:',
  validationError: 'Validation error', submit: 'Submit', cancel: 'Cancel', running: 'Running…', processingFailed: 'Import failed',
  warnings: 'Warnings', dryRunNotice: 'This is a preview — no data has been changed yet. Review the results above, then confirm to apply the import.',
  confirm: 'Confirm', retry: 'Retry', cancelRun: 'Cancel', cancelled: 'Import cancelled', fileRequired: 'Select a file to import',
}

export interface ImportErrorsProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'title'> {
  errors: ImportErrorsMap
  title?: JSX.Element
  exclude?: readonly string[]
}

export function ImportErrors(props: ImportErrorsProps) {
  const [local, native] = splitProps(props, ['errors', 'title', 'exclude', 'class'])
  const entries = () => Object.entries(local.errors).filter(([key, value]) => value && !(local.exclude ?? ['FileID']).includes(key))
  return <Show when={entries().length}><div {...native} class={classes('bg-red-100 text-red-700 py-3 px-4 text-sm font-medium rounded-md mb-4', local.class)} role="alert"><div class="flex items-center space-x-2"><span class="text-red-600" aria-hidden="true">⚠</span><span class="font-semibold">{local.title ?? defaultImportLabels.validationError}</span></div><div class="mt-2 space-y-1"><For each={entries()}>{([, message]) => <div class="flex items-start space-x-2 text-sm"><span class="text-red-500 mt-0.5">•</span><span>{message}</span></div>}</For></div></div></Show>
}

export interface ExampleTableProps extends JSX.HTMLAttributes<HTMLDivElement> { columns: readonly ImportColumn[]; rows: readonly (readonly string[])[] }
function spreadsheetColumn(index: number): string {
  let result = ''
  for (let value = index + 1; value > 0; value = Math.floor((value - 1) / 26)) result = String.fromCharCode(65 + (value - 1) % 26) + result
  return result
}
export function ExampleTable(props: ExampleTableProps) {
  const [local, native] = splitProps(props, ['columns', 'rows', 'class'])
  return <div {...native} class={classes('overflow-x-auto', local.class)}><table class="table-auto border-collapse border border-default w-full text-sm"><thead><tr><th class="border border-default bg-gray-100"></th><For each={local.columns}>{(_, index) => <th class="border border-default px-4 py-2 bg-gray-100 text-center font-semibold">{spreadsheetColumn(index())}</th>}</For></tr><tr class="bg-gray-200 text-left"><th class="border border-default px-4 py-2 text-center bg-gray-100">1</th><For each={local.columns}>{(column) => <th class="border border-default px-4 py-2">{column.header}</th>}</For></tr></thead><tbody><For each={local.rows}>{(row, rowIndex) => <tr><td class="border border-default px-4 py-2 text-center bg-gray-100 font-semibold">{rowIndex() + 2}</td><For each={row}>{(cell) => <td class="border border-default px-4 py-2">{cell}</td>}</For></tr>}</For></tbody></table></div>
}

export interface ImportPageConfig {
  title: JSX.Element
  description?: JSX.Element
  columns?: readonly ImportColumn[]
  exampleRows?: readonly (readonly string[])[]
  acceptedFileTypes?: string
  templateDownloadURL?: string
  submitLabel?: JSX.Element
  submitHint?: JSX.Element
  targetID?: string
}

export interface ImportSubmitRequest { file: UploadItem; formData: FormData }
export type ImportUploadAdapter = (file: File, signal: AbortSignal) => Promise<UploadItem>
export type ImportSubmitAdapter = (request: ImportSubmitRequest, signal: AbortSignal) => Promise<ImportRunState>

export interface ImportFormProps extends Omit<JSX.FormHTMLAttributes<HTMLFormElement>, 'onSubmit'> {
  config: ImportPageConfig
  errors?: ImportErrorsMap
  upload: ImportUploadAdapter
  submit: ImportSubmitAdapter
  extraOptions?: JSX.Element
  labels?: Partial<ImportLabels>
  onCancel?: () => void
  onSubmit?: (state: ImportRunState) => void
  onError?: (error: unknown) => void
}

export function ImportForm(props: ImportFormProps) {
  const [local, native] = splitProps(props, ['config', 'errors', 'upload', 'submit', 'extraOptions', 'labels', 'onCancel', 'onSubmit', 'onError', 'class', 'ref'])
  const [items, setItems] = createSignal<readonly UploadItem[]>([])
  const [clientErrors, setClientErrors] = createSignal<ImportErrorsMap>({})
  const [submitting, setSubmitting] = createSignal(false)
  let request: AbortController | undefined
  let disposed = false
  const labels = () => ({ ...defaultImportLabels, ...local.labels })
  const errors = () => ({ ...(local.errors ?? {}), ...clientErrors() })
  const submit: JSX.EventHandler<HTMLFormElement, SubmitEvent> = async (event) => {
    event.preventDefault()
    const file = items()[0]
    if (!file) { setClientErrors({ FileID: labels().fileRequired }); return }
    request?.abort()
    const controller = new AbortController()
    request = controller
    setSubmitting(true)
    setClientErrors({})
    try {
      const next = await local.submit({ file, formData: new FormData(event.currentTarget) }, controller.signal)
      if (!disposed && !controller.signal.aborted && request === controller) local.onSubmit?.(next)
    }
    catch (error) { if (!controller.signal.aborted) { local.onError?.(error); setClientErrors({ Submit: error instanceof Error ? error.message : labels().processingFailed }) } }
    finally { if (!disposed && !controller.signal.aborted && request === controller) setSubmitting(false) }
  }
  onCleanup(() => { disposed = true; request?.abort() })
  return <form {...native} ref={(element) => { if (typeof local.ref === 'function') local.ref(element) }} id={native.id ?? 'import-form'} class={classes('h-full flex flex-col', local.class)} onSubmit={submit}>
    <div class="flex-1 overflow-y-auto"><div class="px-6 pt-6"><h1 class="text-2xl font-bold mb-4">{local.config.title}</h1></div><Card class="mx-6 mb-6"><div class="flex justify-between items-start mb-4"><p class="text-gray-700">{local.config.description}</p><Show when={local.config.templateDownloadURL}><Button href={local.config.templateDownloadURL!} variant="secondary" size="normal">▣ {labels().downloadTemplate}</Button></Show></div>
      <Show when={local.config.columns?.length}><ul class="list-disc list-inside text-gray-700 mb-4 space-y-2"><For each={local.config.columns}>{(column) => <li><span class="font-medium">{column.header}</span><Show when={column.description}><span class="text-gray-600">- {column.description}</span></Show><Show when={column.required}><span class="text-red-500">*</span></Show></li>}</For></ul></Show>
      <ImportErrors errors={errors()} title={labels().validationError} />
      <Show when={local.config.exampleRows?.length}><div class="mt-6"><p class="text-base mb-3">{labels().exampleBelow}</p><ExampleTable columns={local.config.columns ?? []} rows={local.config.exampleRows ?? []} /></div></Show>
      <div class="mt-6"><UploadDropzone label={labels().upload} placeholder={labels().uploadPlaceholder} error={errors().FileID} accept={local.config.acceptedFileTypes} name="FileID" form={native.id ?? 'import-form'} items={items()} onItemsChange={setItems} upload={local.upload} class="col-span-3" /><Show when={local.extraOptions}><div class="mt-6">{local.extraOptions}</div></Show></div>
    </Card></div>
    <div class="bg-white border-t border-subtle shadow-lg"><div class="px-6 py-4 flex justify-end items-center gap-4"><Show when={local.config.submitHint}><p class="text-sm text-gray-500 mr-auto max-w-xl">{local.config.submitHint}</p></Show><Button type="button" variant="secondary" size="md" onClick={() => local.onCancel?.()}>{labels().cancel}</Button><Button type="submit" size="md" loading={submitting()} disabled={submitting()}>⇧ {local.config.submitLabel ?? labels().submit}</Button></div></div>
  </form>
}

export interface ImportRunResultProps {
  state: ImportRunState
  confirm?: (state: ImportRunState, signal: AbortSignal) => Promise<ImportRunState>
  retry?: () => void
  labels?: Partial<ImportLabels>
  onStateChange?: (state: ImportRunState) => void
  onError?: (error: unknown) => void
}

export function ImportRunResult(props: ImportRunResultProps) {
  const labels = () => ({ ...defaultImportLabels, ...props.labels })
  const [pending, setPending] = createSignal(false)
  let request: AbortController | undefined
  let disposed = false
  const confirm = async () => {
    if (!props.confirm || pending()) return
    request?.abort()
    const controller = new AbortController(); request = controller; setPending(true)
    try {
      const next = await props.confirm(props.state, controller.signal)
      if (!disposed && !controller.signal.aborted && request === controller) props.onStateChange?.(next)
    } catch (error) {
      if (!disposed && !controller.signal.aborted && request === controller) props.onError?.(error)
    } finally {
      if (!disposed && !controller.signal.aborted && request === controller) setPending(false)
    }
  }
  onCleanup(() => { disposed = true; request?.abort() })
  return <Card class="mx-6 mb-6">
    <Show when={props.state.status === 'failed'}><div class="bg-red-100 text-red-700 py-3 px-4 text-sm font-medium rounded-md mb-4" role="alert"><div class="flex items-center space-x-2"><span class="text-red-600">⚠</span><span class="font-semibold">{labels().processingFailed}</span></div><Show when={props.state.error}><div class="mt-2 text-sm">{props.state.error}</div></Show></div></Show>
    <Show when={props.state.status === 'cancelled'}><div class="bg-yellow-50 text-yellow-800 py-3 px-4 text-sm rounded-md mb-4" role="status">{labels().cancelled}</div></Show>
    <Show when={props.state.result}>{(result) => <><Show when={result().counts?.length}><ul class="text-gray-700 mb-4 space-y-2"><For each={result().counts}>{(count) => <li class="flex justify-between border-b border-subtle py-1"><span>{count.label}</span><span class="font-medium">{count.value}</span></li>}</For></ul></Show><Show when={result().warnings?.length}><div class="bg-yellow-50 text-yellow-800 py-3 px-4 text-sm rounded-md mb-4"><div class="font-semibold mb-2">{labels().warnings}</div><ul class="space-y-1"><For each={result().warnings}>{(warning) => <li class="flex items-start space-x-2"><span class="mt-0.5">•</span><span>{warning}</span></li>}</For></ul></div></Show><Show when={result().detail}><div class="mb-4">{result().detail}</div></Show><Show when={result().dryRun && props.state.status !== 'failed' && props.confirm}><div class="bg-blue-50 text-blue-800 py-3 px-4 text-sm rounded-md mb-4">{labels().dryRunNotice}</div><div class="flex justify-end"><Button size="md" loading={pending()} onClick={() => void confirm()}>✓ {labels().confirm}</Button></div></Show></>}</Show>
    <Show when={(props.state.status === 'failed' || props.state.status === 'cancelled') && props.retry}><div class="flex justify-end"><Button size="md" variant="secondary" onClick={props.retry}>{labels().retry}</Button></div></Show>
  </Card>
}

export interface ImportRunProgressProps extends JSX.HTMLAttributes<HTMLDivElement> {
  state?: ImportRunState
  initialState: ImportRunState
  loadStatus?: (state: ImportRunState, signal: AbortSignal) => Promise<ImportRunState>
  pollInterval?: number
  cancel?: (state: ImportRunState, signal: AbortSignal) => Promise<ImportRunState>
  confirm?: ImportRunResultProps['confirm']
  retry?: () => void
  labels?: Partial<ImportLabels>
  onStateChange?: (state: ImportRunState) => void
  onError?: (error: unknown) => void
}

export function ImportRunProgress(props: ImportRunProgressProps) {
  const [local, native] = splitProps(props, ['state', 'initialState', 'loadStatus', 'pollInterval', 'cancel', 'confirm', 'retry', 'labels', 'onStateChange', 'onError', 'class'])
  const [internalState, setInternalState] = createSignal(local.initialState)
  const [cancelling, setCancelling] = createSignal(false)
  const state = () => local.state ?? internalState()
  const labels = () => ({ ...defaultImportLabels, ...local.labels })
  let request: AbortController | undefined
  let action: AbortController | undefined
  let disposed = false
  const terminal = () => ['done', 'failed', 'cancelled'].includes(state().status)
  const update = (next: ImportRunState) => { if (local.state === undefined) setInternalState(next); local.onStateChange?.(next) }
  const poll = async () => {
    if (!local.loadStatus || terminal()) return
    request?.abort(); const controller = new AbortController(); request = controller
    try {
      const next = await local.loadStatus(state(), controller.signal)
      if (!disposed && !controller.signal.aborted && request === controller) update(next)
    } catch (error) { if (!disposed && !controller.signal.aborted && request === controller) local.onError?.(error) }
  }
  let timer: ReturnType<typeof setInterval> | undefined
  onMount(() => { if (local.loadStatus) timer = setInterval(() => void poll(), local.pollInterval ?? 2000) })
  createEffect(() => { if (terminal()) clearInterval(timer) })
  onCleanup(() => { disposed = true; clearInterval(timer); request?.abort(); action?.abort() })
  const cancel = async () => {
    if (!local.cancel || cancelling()) return
    request?.abort()
    action?.abort()
    const controller = new AbortController(); action = controller; setCancelling(true)
    try {
      const next = await local.cancel(state(), controller.signal)
      if (!disposed && !controller.signal.aborted && action === controller) update(next)
    } catch (error) {
      if (!disposed && !controller.signal.aborted && action === controller) local.onError?.(error)
    } finally {
      if (!disposed && !controller.signal.aborted && action === controller) setCancelling(false)
    }
  }
  return <div {...native} id={native.id ?? 'import-run'} class={local.class}><Show when={terminal()} fallback={<Card class="mx-6 mb-6"><div class="space-y-3"><p class="text-sm font-medium text-gray-700">{state().phase || labels().running}</p><Progress value={state().done} target={state().total > 0 ? state().total : 1} /><Show when={local.cancel}><div class="flex justify-end"><Button type="button" variant="secondary" size="sm" loading={cancelling()} onClick={() => void cancel()}>{labels().cancelRun}</Button></div></Show></div></Card>}><ImportRunResult state={state()} confirm={local.confirm} retry={local.retry} labels={local.labels} onStateChange={update} onError={local.onError} /></Show></div>
}

export const RunProgress = ImportRunProgress
export const RunResult = ImportRunResult

export interface ImportPageProps extends Omit<ImportFormProps, 'onSubmit'> {
  runState?: ImportRunState
  loadStatus?: ImportRunProgressProps['loadStatus']
  cancelRun?: ImportRunProgressProps['cancel']
  confirmRun?: ImportRunProgressProps['confirm']
  retry?: () => void
  onRunStateChange?: (state: ImportRunState) => void
}

export function ImportPage(props: ImportPageProps) {
  const [workflow, formProps] = splitProps(props, ['runState', 'loadStatus', 'cancelRun', 'confirmRun', 'retry', 'onRunStateChange'])
  const [state, setState] = createSignal<ImportRunState | undefined>(workflow.runState)
  const current = () => workflow.runState ?? state()
  const update = (next: ImportRunState) => { if (workflow.runState === undefined) setState(next); workflow.onRunStateChange?.(next) }
  return <div id={formProps.config.targetID ?? 'import-page'} class="h-full"><Show when={current()} fallback={<ImportForm {...formProps} onSubmit={update} />}>
    {(run) => <ImportRunProgress initialState={run()} state={workflow.runState} loadStatus={workflow.loadStatus} cancel={workflow.cancelRun} confirm={workflow.confirmRun} retry={workflow.retry} labels={formProps.labels} onStateChange={update} onError={formProps.onError} />}
  </Show></div>
}

import { createSignal, createUniqueId, For, onCleanup, Show, splitProps, type JSX } from 'solid-js'
import { Button } from '../forms/Button'
import { classes } from '../internal/classes'
import { callHandler } from '../internal/events'
import { Spinner } from '../display/Spinner'

export interface UploadItem {
  id: string
  name?: string
  slug?: string
  url?: string
  mimeType?: string
  size?: string
}

export interface UploadLabels {
  genericError?: string
  networkError?: string
  typeRejected?: string
  sizeRejected?: string
  tooLargeStatus?: string
  unsupportedType?: string
  serverError?: string
  removeFile?: string
}

const labelDefaults: Required<UploadLabels> = {
  genericError: 'Upload failed',
  networkError: 'Network error — please retry',
  typeRejected: 'File "{name}" — unsupported format',
  sizeRejected: 'File "{name}" is too large ({size}MB > {limit}MB)',
  tooLargeStatus: 'File too large',
  unsupportedType: 'Unsupported file type',
  serverError: 'Server error — please retry later',
  removeFile: 'Remove file',
}

function labels(value?: UploadLabels): Required<UploadLabels> {
  return { ...labelDefaults, ...value }
}

export function acceptsFile(file: Pick<File, 'name' | 'type'>, accept: string): boolean {
  if (!accept.trim()) return true
  const name = file.name.toLowerCase()
  const type = file.type.toLowerCase()
  return accept.split(',').map((token) => token.trim().toLowerCase()).filter(Boolean).some((token) => {
    if (token.startsWith('.')) return name.endsWith(token)
    if (token.endsWith('/*')) return type.startsWith(token.slice(0, -1))
    return type === token
  })
}

export function validateUploadFiles(files: readonly File[], accept: string, maxSize: number, copy: UploadLabels = {}): string[] {
  const text = labels(copy)
  const errors: string[] = []
  for (const file of files) {
    if (maxSize > 0 && file.size > maxSize) {
      errors.push(text.sizeRejected
        .replace('{name}', file.name)
        .replace('{size}', (file.size / 1_048_576).toFixed(1))
        .replace('{limit}', (maxSize / 1_048_576).toFixed(1)))
      continue
    }
    if (!acceptsFile(file, accept)) errors.push(text.typeRejected.replace('{name}', file.name))
  }
  return errors
}

function displayName(item: UploadItem): string {
  if (item.name?.trim()) return item.name.trim()
  if (item.slug?.trim()) return item.slug.trim()
  if (item.url?.trim()) {
    const part = item.url.split('/').filter(Boolean).at(-1)
    if (part) return part
  }
  return item.id ? `File #${item.id}` : 'Uploaded file'
}

export interface UploadListProps extends JSX.HTMLAttributes<HTMLDivElement> {
  items: readonly UploadItem[]
  name?: string
  form?: string
  labels?: UploadLabels
  onRemove?: (item: UploadItem, index: number) => void
}

export function UploadList(props: UploadListProps) {
  const [local, native] = splitProps(props, ['class', 'items', 'name', 'form', 'labels', 'onRemove'])
  const copy = () => labels(local.labels)
  return (
    <div {...native} class={classes('w-full space-y-2', local.class)}>
      <For each={local.items}>{(item, index) => {
        const name = () => displayName(item)
        const meta = () => [item.mimeType?.trim(), item.size?.trim()].filter(Boolean).join(' • ')
        const image = () => Boolean(item.url && item.mimeType?.trim().toLowerCase().startsWith('image/'))
        return (
          <div class="rounded-md border border-subtle bg-surface-100 p-2" data-upload-id={item.id}>
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <Show when={item.url} fallback={<div class="truncate text-xs font-medium text-100" title={name()}>{name()}</div>}>
                  {(url) => <a href={url()} target="_blank" rel="noopener noreferrer" class="block truncate text-xs font-medium text-primary hover:underline" title={name()}>{name()}</a>}
                </Show>
                <div class="truncate text-[11px] text-text-300">{meta()}</div>
              </div>
              <button type="button" class="text-red-500 hover:text-red-700 text-xs" title={copy().removeFile} aria-label={copy().removeFile} onClick={() => local.onRemove?.(item, index())}>
                <svg aria-hidden="true" width="14" height="14" viewBox="0 0 256 256" xmlns="http://www.w3.org/2000/svg">
                  <line x1="200" y1="56" x2="56" y2="200" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" />
                  <line x1="200" y1="200" x2="56" y2="56" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" />
                </svg>
              </button>
            </div>
            <Show when={image()}>
              <a href={item.url} target="_blank" rel="noopener noreferrer"><img class="mt-2 h-16 w-16 rounded-md object-contain" src={item.url} alt={name()} /></a>
            </Show>
            <Show when={local.name}><input type="hidden" name={local.name} value={item.id} form={local.form} /></Show>
          </div>
        )
      }}</For>
    </div>
  )
}

export interface UploadDropzoneProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'onError'> {
  label: string
  placeholder?: string
  accept?: string
  multiple?: boolean
  maxSize?: number
  name?: string
  form?: string
  items?: readonly UploadItem[]
  defaultItems?: readonly UploadItem[]
  onItemsChange?: (items: readonly UploadItem[]) => void
  onFilesSelected?: (files: readonly File[]) => void
  upload?: (file: File, signal: AbortSignal) => Promise<UploadItem>
  labels?: UploadLabels
  error?: string
  inputProps?: Omit<JSX.InputHTMLAttributes<HTMLInputElement>, 'type' | 'accept' | 'multiple'>
}

export function UploadDropzone(props: UploadDropzoneProps) {
  const generatedID = createUniqueId()
  const [local, native] = splitProps(props, [
    'class', 'label', 'placeholder', 'accept', 'multiple', 'maxSize', 'name', 'form', 'items', 'defaultItems', 'onItemsChange',
    'onFilesSelected', 'upload', 'labels', 'error', 'inputProps',
  ])
  const [internalItems, setInternalItems] = createSignal<readonly UploadItem[]>(local.defaultItems ?? [])
  const [pending, setPending] = createSignal(false)
  const [clientError, setClientError] = createSignal('')
  const [dragging, setDragging] = createSignal(false)
  const id = () => local.inputProps?.id ?? generatedID
  const accept = () => local.accept ?? 'image/*'
  const items = () => local.items ?? internalItems()
  const copy = () => labels(local.labels)
  let input!: HTMLInputElement
  let controller: AbortController | undefined
  onCleanup(() => controller?.abort())

  const updateItems = (next: readonly UploadItem[]) => {
    if (local.items === undefined) setInternalItems(next)
    local.onItemsChange?.(next)
  }
  const process = async (files: readonly File[]) => {
    const errors = validateUploadFiles(files, accept(), local.maxSize ?? 0, local.labels)
    if (errors.length) {
      setClientError(errors.join('; '))
      input.value = ''
      return
    }
    setClientError('')
    local.onFilesSelected?.(files)
    if (!local.upload || files.length === 0) return
    controller?.abort()
    const current = new AbortController()
    controller = current
    setPending(true)
    try {
      const uploaded = await Promise.all(files.map((file) => local.upload!(file, current.signal)))
      if (current.signal.aborted || controller !== current) return
      updateItems([...items(), ...uploaded])
    } catch (cause) {
      if (!current.signal.aborted) setClientError(cause instanceof Error && cause.message ? cause.message : copy().networkError)
    } finally {
      if (!current.signal.aborted && controller === current) setPending(false)
      if (input?.isConnected) input.value = ''
    }
  }
  const onInputChange: JSX.EventHandler<HTMLInputElement, Event> = (event) => {
    const files = [...(event.currentTarget.files ?? [])]
    void process(local.multiple ? files : files.slice(0, 1))
    callHandler(local.inputProps?.onChange, event)
  }
  const onDrop: JSX.EventHandler<HTMLDivElement, DragEvent> = (event) => {
    event.preventDefault()
    setDragging(false)
    const files = [...(event.dataTransfer?.files ?? [])]
    void process(local.multiple ? files : files.slice(0, 1))
  }
  const remove = (item: UploadItem, index: number) => updateItems(items().filter((candidate, candidateIndex) => candidateIndex !== index || candidate !== item))

  return (
    <div
      {...native}
      class={classes('relative border border-default border-dashed rounded-md p-4 flex flex-col items-center transition-colors', dragging() && 'border-brand', local.class)}
      onDragEnter={(event) => { event.preventDefault(); setDragging(true) }}
      onDragOver={(event) => event.preventDefault()}
      onDragLeave={(event) => { if (event.currentTarget === event.target) setDragging(false) }}
      onDrop={onDrop}
    >
      <div class={classes('htmx-indicator pointer-events-none absolute inset-0 z-10 flex items-center justify-center rounded-md bg-surface-100/80', !pending() && 'hidden')} aria-hidden="true">
        <Spinner spinnerClass="w-6 h-6" />
      </div>
      <div class="flex flex-col items-center">
        <input
          {...local.inputProps}
          ref={(element) => {
            input = element
            if (typeof local.inputProps?.ref === 'function') local.inputProps.ref(element)
          }}
          id={id()}
          type="file"
          class={classes('sr-only', local.inputProps?.class)}
          aria-hidden="true"
          tabindex="-1"
          accept={accept()}
          multiple={local.multiple}
          name={local.inputProps?.name ?? 'file'}
          onChange={onInputChange}
        />
        <UploadList items={items()} name={local.name} form={local.form} labels={local.labels} onRemove={remove} />
        <div class="flex gap-1 items-center mt-4 mb-1.5">
          <Button type="button" variant="primary-outline" size="xs" aria-label={local.label} title={local.label} onClick={() => input.click()} disabled={pending()}>{local.label}</Button>
        </div>
        <Show when={local.placeholder}><p class="text-xs">{local.placeholder}</p></Show>
        <p class={classes('text-red-500 text-xs mt-2', !clientError() && 'hidden')} role="alert" aria-live="polite">{clientError()}</p>
        <Show when={local.error}><p class="text-red-500 text-xs mt-2" role="alert">{local.error}</p></Show>
      </div>
    </div>
  )
}

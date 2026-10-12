import { createEffect, createMemo, createSignal, For, onCleanup, Show, type JSX } from 'solid-js'
import type { Column, FieldFormat, Frame, LevelSource, TableColumn } from '../contract'
import { QueryRequestSchema, QueryResponseSchema } from '../contract'
import { CaretRight } from '../icons'
import { queryPathForNavigation, useDashboard, useFormat, useTranslate } from '../runtime'

/**
 * The collapsed «source data» section of a focus canvas: the audit rows behind
 * the level's figure, folded away until asked for. The table itself mounts on
 * first expand; a source frame the document did not inline is asked for from
 * the query endpoint at that moment, so a level that is never audited costs
 * nothing.
 */
export interface SourceDataDisclosureProps {
  source: LevelSource
  style?: JSX.CSSProperties
}

interface SourceColumn {
  field: string
  label: string
  align?: TableColumn['align']
  index: number
  type: Column['type']
}

function resolveColumns(source: LevelSource, frame: Frame): Array<SourceColumn> {
  const declared = source.columns?.length
    ? source.columns.map(({ field, label, align }) => ({ field, label, align }))
    : frame.columns.map(({ name }) => ({ field: name, label: name, align: undefined }))
  return declared.flatMap((column) => {
    const index = frame.columns.findIndex(({ name }) => name === column.field)
    if (index < 0) return []
    return [{ ...column, index, type: frame.columns[index]!.type }]
  })
}

function inferredFormat(type: Column['type']): FieldFormat | undefined {
  if (type === 'number') return { kind: 'number', minorUnits: false }
  if (type === 'time') return { kind: 'date', minorUnits: false }
  return undefined
}

function SourceCell(props: { column: SourceColumn; format?: FieldFormat; value: unknown }): JSX.Element {
  const display = useFormat(props.format ?? inferredFormat(props.column.type))
  const text = display(props.value)
  if (props.column.type === 'time' && text !== '—') {
    return <time dateTime={typeof props.value === 'string' ? props.value : undefined}>{text}</time>
  }
  return <>{text}</>
}

export function SourceDataDisclosure(props: SourceDataDisclosureProps): JSX.Element {
  const { document, navigation } = useDashboard()
  const translate = useTranslate()
  const [open, setOpen] = createSignal(false)
  const [fetched, setFetched] = createSignal<Frame>()
  const [loading, setLoading] = createSignal(false)
  const [failed, setFailed] = createSignal(false)
  let requested = false
  const frame = createMemo(() => document.frames[props.source.frame] ?? fetched())
  const endpoint = () => document.endpoints.query
  const label = () => props.source.label?.trim() || translate('focus.sourceData', 'Source data')
  const columns = createMemo(() => (frame() ? resolveColumns(props.source, frame()!) : []))

  // Lazy fetch, once, on first expand: the document usually inlines the source
  // frame beside its level, so this path only runs for a deep link whose
  // snapshot response left it out.
  createEffect(() => {
    if (!open() || frame() || !endpoint() || requested) return
    requested = true
    const controller = new AbortController()
    setLoading(true)
    const request = QueryRequestSchema.parse({
      snapshotId: document.snapshotId,
      path: queryPathForNavigation(document, navigation.path),
      ...(navigation.perspectiveId ? { perspective: navigation.perspectiveId } : {}),
    })
    void fetch(endpoint()!, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(request),
      signal: controller.signal,
    })
      .then(async (response) => {
        if (!response.ok) throw new Error(`source query failed with ${response.status}`)
        const payload = QueryResponseSchema.parse(await response.json())
        const resolved = payload.frames[props.source.frame]
        if (!resolved) throw new Error('source frame missing from query response')
        setFetched(resolved)
      })
      .catch(() => {
        if (!controller.signal.aborted) setFailed(true)
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false)
      })
    onCleanup(() => {
      if (controller.signal.aborted) return
      controller.abort()
      requested = false
      setLoading(false)
    })
  })

  return (
    <div class={`lens-focus-source${open() ? ' lens-focus-source-open' : ''}`} style={props.style}>
      <button
        aria-expanded={open()}
        class="lens-focus-source-toggle"
        onClick={() => setOpen((current) => !current)}
        type="button"
      >
        <CaretRight className="lens-focus-source-caret" size={12} />
        <span class="lens-focus-source-label">{label()}</span>
        <Show when={frame()}>
          {(value) => (
            <span class="lens-focus-source-count">
              {translate('focus.sourceRows', '{n} rows', { n: value().rows.length })}
            </span>
          )}
        </Show>
      </button>
      <Show when={open()}>
        <div class="lens-focus-source-body">
          <Show
            when={frame()}
            fallback={(
              <p class="lens-focus-source-state" role={failed() ? 'alert' : 'status'}>
                {loading()
                  ? translate('focus.sourceLoading', 'Loading source data…')
                  : translate('focus.sourceUnavailable', 'Source data is unavailable.')}
              </p>
            )}
          >
            {(value) => (
              <div class="lens-focus-source-scroll">
                <table class="lens-table lens-focus-source-table">
                  <thead>
                    <tr>
                      <For each={columns()}>
                        {(column) => (
                          <th
                            class={column.align === 'right' ? 'lens-table-col-right' : undefined}
                            scope="col"
                          >
                            <span class="lens-table-heading-static">{column.label}</span>
                          </th>
                        )}
                      </For>
                    </tr>
                  </thead>
                  <tbody>
                    <For each={value().rows}>
                      {(row) => (
                        <tr>
                          <For each={columns()}>
                            {(column) => (
                              <td class={column.align === 'right' ? 'lens-table-col-right' : undefined}>
                                <SourceCell column={column} format={props.source.format?.[column.field]} value={row[column.index]} />
                              </td>
                            )}
                          </For>
                        </tr>
                      )}
                    </For>
                    <Show when={value().rows.length === 0}>
                      <tr>
                        <td class="lens-table-empty" colSpan={Math.max(1, columns().length)}>
                          {translate('panel.empty', 'No data')}
                        </td>
                      </tr>
                    </Show>
                  </tbody>
                </table>
              </div>
            )}
          </Show>
        </div>
      </Show>
    </div>
  )
}

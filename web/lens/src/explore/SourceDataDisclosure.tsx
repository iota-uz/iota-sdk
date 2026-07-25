import { useEffect, useMemo, useRef, useState, type CSSProperties } from 'react'
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
  style?: CSSProperties
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

function SourceCell({ column, format, value }: { column: SourceColumn; format?: FieldFormat; value: unknown }) {
  const display = useFormat(format ?? inferredFormat(column.type))
  const text = display(value)
  if (column.type === 'time' && text !== '—') {
    return <time dateTime={typeof value === 'string' ? value : undefined}>{text}</time>
  }
  return <>{text}</>
}

export function SourceDataDisclosure({ source, style }: SourceDataDisclosureProps) {
  const { document, navigation } = useDashboard()
  const translate = useTranslate()
  const [open, setOpen] = useState(false)
  const [fetched, setFetched] = useState<Frame>()
  const [loading, setLoading] = useState(false)
  const [failed, setFailed] = useState(false)
  const requested = useRef(false)
  const frame = document.frames[source.frame] ?? fetched
  const endpoint = document.endpoints.query
  const label = source.label?.trim() || translate('focus.sourceData', 'Source data')

  // Lazy fetch, once, on first expand: the document usually inlines the source
  // frame beside its level, so this path only runs for a deep link whose
  // snapshot response left it out.
  useEffect(() => {
    if (!open || frame || !endpoint || requested.current) return
    requested.current = true
    const controller = new AbortController()
    setLoading(true)
    const request = QueryRequestSchema.parse({
      snapshotId: document.snapshotId,
      path: queryPathForNavigation(document, navigation.path),
      ...(navigation.perspectiveId ? { perspective: navigation.perspectiveId } : {}),
    })
    void fetch(endpoint, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(request),
      signal: controller.signal,
    })
      .then(async (response) => {
        if (!response.ok) throw new Error(`source query failed with ${response.status}`)
        const payload = QueryResponseSchema.parse(await response.json())
        const resolved = payload.frames[source.frame]
        if (!resolved) throw new Error('source frame missing from query response')
        setFetched(resolved)
      })
      .catch(() => {
        if (!controller.signal.aborted) setFailed(true)
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false)
      })
    return () => controller.abort()
  }, [document, endpoint, frame, navigation.path, navigation.perspectiveId, open, source.frame])

  const columns = useMemo(() => (frame ? resolveColumns(source, frame) : []), [frame, source])

  return (
    <div className={`lens-focus-source${open ? ' lens-focus-source-open' : ''}`} style={style}>
      <button
        aria-expanded={open}
        className="lens-focus-source-toggle"
        onClick={() => setOpen((current) => !current)}
        type="button"
      >
        <CaretRight className="lens-focus-source-caret" size={12} />
        <span className="lens-focus-source-label">{label}</span>
        {frame && (
          <span className="lens-focus-source-count">
            {translate('focus.sourceRows', '{n} rows', { n: frame.rows.length })}
          </span>
        )}
      </button>
      {open && (
        <div className="lens-focus-source-body">
          {frame ? (
            <div className="lens-focus-source-scroll">
              <table className="lens-table lens-focus-source-table">
                <thead>
                  <tr>
                    {columns.map((column) => (
                      <th
                        className={column.align === 'right' ? 'lens-table-col-right' : undefined}
                        key={column.field}
                        scope="col"
                      >
                        <span className="lens-table-heading-static">{column.label}</span>
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {frame.rows.map((row, rowIndex) => (
                    <tr key={rowIndex}>
                      {columns.map((column) => (
                        <td
                          className={column.align === 'right' ? 'lens-table-col-right' : undefined}
                          key={column.field}
                        >
                          <SourceCell column={column} format={source.format?.[column.field]} value={row[column.index]} />
                        </td>
                      ))}
                    </tr>
                  ))}
                  {frame.rows.length === 0 && (
                    <tr>
                      <td className="lens-table-empty" colSpan={Math.max(1, columns.length)}>
                        {translate('panel.empty', 'No data')}
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          ) : (
            <p className="lens-focus-source-state" role={failed ? 'alert' : 'status'}>
              {loading
                ? translate('focus.sourceLoading', 'Loading source data…')
                : translate('focus.sourceUnavailable', 'Source data is unavailable.')}
            </p>
          )}
        </div>
      )}
    </div>
  )
}

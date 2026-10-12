import { createMemo, createSignal } from 'solid-js'
import { CompositionSchema, DefinitionSchema } from './contract/schemas'
import type { Composition, Definition, Placement, SourceDescriptor } from './contract/types'

export type ReportDefinition = Definition
export type ReportComposition = Composition
export type ReportPlacement = Placement
export type ReportSourceDescriptor = SourceDescriptor
export type SaveIntent = 'draft' | 'publish'

export interface ReportDraftHost<T> {
  save(value: T, options: { expectedRevision: string; intent: SaveIntent }): Promise<T>
}

function copy<T>(value: T): T {
  return structuredClone(value)
}

export function parseReportDefinition(value: unknown): Definition {
  const definition = DefinitionSchema.parse(value)
  if (!definition.id || !definition.revision || !definition.title.trim() || definition.source.version < 1) {
    throw new Error('Invalid report definition')
  }
  return definition
}

export function parseReportComposition(value: unknown): Composition {
  const composition = CompositionSchema.parse(value)
  const ids = new Set<string>()
  if (!composition.id || !composition.revision || !composition.title.trim()) throw new Error('Invalid report composition')
  for (const row of composition.rows) {
    if (!row.length) throw new Error('Empty report row')
    let span = 0
    for (const placement of row) {
      if (!placement.id || !placement.reportId || ids.has(placement.id)) throw new Error('Invalid report placement')
      ids.add(placement.id)
      const width = placement.span ?? 12
      if (!Number.isInteger(width) || width < 1 || width > 12) throw new Error('Invalid report width')
      span += width
    }
    if (span > 12) throw new Error('Report row exceeds twelve columns')
  }
  return composition
}

export function copyReportDefinition(value: Definition, id: string, revision: string): Definition {
  if (id === value.id) throw new Error('A report copy needs a new identity')
  return parseReportDefinition({ ...copy(value), id, revision })
}

export function duplicateReportPlacement(value: Placement, id: string): Placement {
  if (!id || id === value.id) throw new Error('A placement copy needs a new identity')
  return { ...copy(value), id }
}

/** An explicit host acknowledgement is the only operation that advances the saved revision. */
export function createReportDraft<T extends Definition | Composition>(initial: T, host: ReportDraftHost<T>) {
  const validate = (value: unknown): T => ('rows' in initial ? parseReportComposition(value) : parseReportDefinition(value)) as T
  const [saved, setSaved] = createSignal(copy(validate(initial)))
  const [draft, setDraft] = createSignal(copy(initial))
  const [saving, setSaving] = createSignal(false)
  const [error, setError] = createSignal<unknown>()
  const dirty = createMemo(() => JSON.stringify(saved()) !== JSON.stringify(draft()))

  return {
    value: () => copy(draft()),
    saved: () => copy(saved()),
    dirty,
    saving,
    error,
    update(change: (current: T) => T) {
      setDraft(() => copy(('rows' in initial ? CompositionSchema : DefinitionSchema).parse(change(copy(draft()))) as T))
      setError(undefined)
    },
    cancel() {
      if (saving()) throw new Error('Cannot cancel an in-flight save')
      setDraft(() => copy(saved()))
      setError(undefined)
    },
    async save(intent: SaveIntent = 'draft') {
      if (saving()) throw new Error('A save is already in flight')
      const submitted = copy(draft())
      const submittedJSON = JSON.stringify(submitted)
      setSaving(true)
      setError(undefined)
      try {
        const acknowledged = copy(validate(await host.save(copy(validate(submitted)), { expectedRevision: saved().revision, intent })))
        if (acknowledged.id !== submitted.id || acknowledged.revision === saved().revision) {
          throw new Error('Invalid host save acknowledgement')
        }
        setSaved(() => acknowledged)
        if (JSON.stringify(draft()) === submittedJSON) setDraft(() => copy(acknowledged))
        else setDraft((current) => ({ ...current, revision: acknowledged.revision }))
        return copy(acknowledged)
      } catch (cause) {
        setError(cause)
        throw cause
      } finally {
        setSaving(false)
      }
    },
  }
}

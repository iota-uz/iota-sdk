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

/**
 * Thrown by a host save when expectedRevision is no longer the stored one.
 * latest, when supplied, is the stored value the reader can review or adopt;
 * the draft keeps every local edit until the reader decides.
 */
export class ReportRevisionConflictError<T = unknown> extends Error {
  readonly code = 'REPORT_REVISION_CONFLICT'

  constructor(readonly latest?: T, message = 'The report was saved by someone else') {
    super(message)
    this.name = 'ReportRevisionConflictError'
  }
}

/** How a draft adopts a newer stored value: keep the local edits on top of it, or discard them. */
export type ReportRebaseMode = 'keep' | 'discard'

function copy<T>(value: T): T {
  return structuredClone(value)
}

/** JSON with object keys sorted, so equal values compare equal whatever order a host wrote them in. */
function canonical(value: unknown): string {
  return JSON.stringify(value, (_key, item: unknown) => (item && typeof item === 'object' && !Array.isArray(item)
    ? Object.fromEntries(Object.entries(item as Record<string, unknown>).sort(([left], [right]) => (left < right ? -1 : left > right ? 1 : 0)))
    : item))
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
  const [draft, setDraft] = createSignal(copy(saved()))
  const [saving, setSaving] = createSignal(false)
  const [error, setError] = createSignal<unknown>()
  const [sequence, setSequence] = createSignal(0)
  const dirty = createMemo(() => canonical(saved()) !== canonical(draft()))
  const touch = () => setSequence((value) => value + 1)

  return {
    value: () => copy(draft()),
    saved: () => copy(saved()),
    dirty,
    saving,
    error,
    /** Counts local changes (edit, cancel, rebase); an external proposal is stale once it moves. */
    sequence,
    /** The revision conflict of the last save, or undefined. */
    conflict: (): ReportRevisionConflictError<T> | undefined => {
      const value = error()
      return value instanceof ReportRevisionConflictError ? value as ReportRevisionConflictError<T> : undefined
    },
    update(change: (current: T) => T) {
      setDraft(() => copy(('rows' in initial ? CompositionSchema : DefinitionSchema).parse(change(copy(draft()))) as T))
      setError(undefined)
      touch()
    },
    cancel() {
      if (saving()) throw new Error('Cannot cancel an in-flight save')
      setDraft(() => copy(saved()))
      setError(undefined)
      touch()
    },
    /**
     * Adopts a newer stored value as the saved base, for example after a
     * revision conflict. keep leaves the local edits in place to be saved
     * against it; discard replaces them with it.
     */
    rebase(latest: T, mode: ReportRebaseMode) {
      if (saving()) throw new Error('Cannot rebase during an in-flight save')
      const base = copy(validate(latest))
      if (base.id !== saved().id) throw new Error('A draft can only rebase onto the same report')
      setSaved(() => base)
      setDraft((current) => (mode === 'discard' ? copy(base) : { ...current, revision: base.revision }))
      setError(undefined)
      touch()
    },
    async save(intent: SaveIntent = 'draft') {
      if (saving()) throw new Error('A save is already in flight')
      const submitted = copy(draft())
      const submittedJSON = canonical(submitted)
      setSaving(true)
      setError(undefined)
      try {
        const acknowledged = copy(validate(await host.save(copy(validate(submitted)), { expectedRevision: saved().revision, intent })))
        if (acknowledged.id !== submitted.id || acknowledged.revision === saved().revision) {
          throw new Error('Invalid host save acknowledgement')
        }
        setSaved(() => acknowledged)
        if (canonical(draft()) === submittedJSON) setDraft(() => copy(acknowledged))
        else setDraft((current) => ({ ...current, revision: acknowledged.revision }))
        touch()
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

export type ReportDraft<T extends Definition | Composition> = ReturnType<typeof createReportDraft<T>>

/**
 * Identifies the draft state an external proposal was prepared against. A
 * proposal whose base differs from the current token is stale.
 */
export function reportDraftToken(draft: Pick<ReportDraft<Composition>, 'saved' | 'sequence'>): string {
  return `${draft.saved().revision}:${draft.sequence()}`
}

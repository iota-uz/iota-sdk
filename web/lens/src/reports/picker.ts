import { createMemo, createSignal } from 'solid-js'
import type { DashboardDocument, EffectiveContext } from '../contract/types'
import { CatalogPageSchema, DestinationSchema } from './contract/schemas'
import type { CatalogEntry, CatalogPage, CatalogQuery, CatalogScope, Composition, Destination, Placement } from './contract/types'
import { placeLibraryReport } from './placement'

export type ReportCatalogEntry = CatalogEntry
export type ReportCatalogPage = CatalogPage
export type ReportCatalogQuery = CatalogQuery
export type ReportCatalogScope = CatalogScope
export type ReportDestination = Destination

/**
 * The host's report library. search lists only reports the reader may read;
 * preview returns the document of one entry. Both receive an AbortSignal that
 * fires when the reader moves on.
 */
export interface ReportCatalogHost {
  search(query: CatalogQuery, options: { signal: AbortSignal }): Promise<CatalogPage>
  preview?(entry: CatalogEntry, options: { signal: AbortSignal }): Promise<DashboardDocument>
}

/** One chosen report. duplicate marks a deliberate second card of a report the target already places. */
export interface ReportPick {
  entry: CatalogEntry
  duplicate: boolean
}

/**
 * What the reader confirmed. The id is stable across retries of the same
 * choice, so the caller can make adding idempotent; picks keep click order.
 */
export interface ReportPickerConfirmation {
  id: string
  target: string
  picks: ReportPick[]
}

export function parseCatalogPage(value: unknown): CatalogPage {
  const page = CatalogPageSchema.parse(value)
  const ids = new Set<string>()
  for (const entry of page.entries) {
    if (!entry.id || !entry.revision || !entry.title.trim() || ids.has(entry.id)) throw new Error('Invalid report catalog page')
    ids.add(entry.id)
  }
  return page
}

export function parseDestinations(value: unknown): Destination[] {
  return DestinationSchema.array().parse(value)
}

let confirmationSequence = 0

function confirmationId(): string {
  const random = globalThis.crypto?.randomUUID?.()
  confirmationSequence += 1
  return random ? `pick-${random}` : `pick-${Date.now().toString(36)}-${confirmationSequence}`
}

/**
 * The picker's own selection: ordered by the reader's clicks and discarded with
 * the picker. A report the target already places cannot be chosen again by a
 * plain click; addAnother is the explicit second-card choice.
 */
export function createReportPickerSelection(placed: (entry: CatalogEntry) => number) {
  const [picks, setPicks] = createSignal<ReportPick[]>([])
  let pending: { key: string; id: string } | undefined
  const key = (value: ReportPick[]) => JSON.stringify(value.map((pick) => [pick.entry.id, pick.duplicate]))
  const index = (id: string) => picks().findIndex((pick) => pick.entry.id === id)
  const without = (id: string) => picks().filter((pick) => pick.entry.id !== id)

  return {
    picks: () => picks().map((pick) => ({ ...pick })),
    count: createMemo(() => picks().length),
    selected: (id: string) => index(id) >= 0,
    duplicate: (id: string) => picks()[index(id)]?.duplicate ?? false,
    toggle(entry: CatalogEntry) {
      if (placed(entry) > 0) return
      setPicks(index(entry.id) >= 0 ? without(entry.id) : [...picks(), { entry, duplicate: false }])
    },
    addAnother(entry: CatalogEntry) {
      if (placed(entry) === 0) return
      setPicks(index(entry.id) >= 0 ? without(entry.id) : [...picks(), { entry, duplicate: true }])
    },
    clear() { setPicks([]) },
    /** The confirmation of the current choice; the same choice keeps its id until it succeeds. */
    confirmation(target: string): ReportPickerConfirmation {
      const current = picks()
      const currentKey = `${target}:${key(current)}`
      if (pending?.key !== currentKey) pending = { key: currentKey, id: confirmationId() }
      return { id: pending.id, target, picks: current.map((pick) => ({ entry: { ...pick.entry }, duplicate: pick.duplicate })) }
    },
    /** Forget the confirmation id after the caller acknowledged it. */
    settle() { pending = undefined },
  }
}

/**
 * Appends a confirmed choice to the end of a composition, one card per row.
 * Library placements inherit the composition's filters. Applying the same
 * confirmation twice adds nothing, and a plain pick of an already placed
 * report is skipped rather than duplicated.
 */
export function appendReportPicks(
  composition: Composition,
  confirmation: ReportPickerConfirmation,
  options: { span?: number } = {},
): { composition: Composition; added: Placement[] } {
  const next = structuredClone(composition)
  const placements = next.rows.flat()
  const ids = new Set(placements.map((placement) => placement.id))
  const reports = new Set(placements.map((placement) => placement.reportId))
  const added: Placement[] = []
  confirmation.picks.forEach((pick, position) => {
    const id = `${confirmation.id}-${position + 1}`
    if (ids.has(id) || (!pick.duplicate && reports.has(pick.entry.id))) return
    const placement = placeLibraryReport(id, pick.entry.id)
    if (options.span !== undefined && options.span !== 12) placement.span = options.span
    next.rows.push([placement])
    ids.add(id)
    reports.add(pick.entry.id)
    added.push(structuredClone(placement))
  })
  return { composition: next, added }
}

/** Where an opened analysis is added: an existing composition or a new one the host creates. */
export type ReportDestinationChoice = { kind: 'existing'; destination: Destination } | { kind: 'new' }

/**
 * The confirmed destination with the slice shown when the reader chose it.
 * The host places the report with placeOpenedReport, so the card keeps that
 * period locally; nothing is published by the dialog.
 */
export interface ReportDestinationConfirmation {
  id: string
  reportId: string
  choice: ReportDestinationChoice
  context?: EffectiveContext
}

export interface ReportDestinationHost {
  destinations(query: { reportId: string; search: string }, options: { signal: AbortSignal }): Promise<Destination[]>
}

export function destinationConfirmationId(): string {
  return confirmationId()
}

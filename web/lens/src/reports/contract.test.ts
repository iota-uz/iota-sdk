import { createRoot } from 'solid-js'
import { describe, expect, it } from 'vitest'
import fixture from '../../fixtures/report-contract.json'
import { copyReportDefinition, createReportDraft, duplicateReportPlacement, parseReportComposition, parseReportDefinition } from './draft'
import type { ReportComposition, ReportDefinition } from './draft'
import { compareReportVersion } from './version'

// Two host-owned persistence adapters. The SDK only parses what they hand back.
interface StoringHost {
  put(kind: string, id: string, value: unknown): void
  get(kind: string, id: string): unknown
}

function objectHost(): StoringHost {
  const records = new Map<string, unknown>()
  return {
    put: (kind, id, value) => { records.set(`${kind}/${id}`, structuredClone(value)) },
    get: (kind, id) => structuredClone(records.get(`${kind}/${id}`)),
  }
}

function textHost(): StoringHost {
  const records: Record<string, string> = {}
  return {
    put: (kind, id, value) => { records[`${kind}:${id}`] = JSON.stringify(value) },
    get: (kind, id): unknown => JSON.parse(records[`${kind}:${id}`] ?? 'null') as unknown,
  }
}

const restoreDefinition = (host: StoringHost, id: string) => parseReportDefinition(host.get('definition', id))
const restoreComposition = (host: StoringHost, id: string) => parseReportComposition(host.get('composition', id))

// False green: a single in-memory object placed once passes identity checks without storage.
// The same fixture drives pkg/lens/report/acceptance_test.go.
describe('reusable report contract across host adapters', () => {
  const restored: Record<string, ReportComposition> = {}

  for (const [name, open] of Object.entries({ object: objectHost, text: textHost })) {
    it(`${name} adapter restores the stored definitions and placements`, () => {
      const host = open()
      for (const raw of fixture.definitions) {
        const definition = parseReportDefinition(raw)
        host.put('definition', definition.id, definition)
      }
      const composition = parseReportComposition(fixture.composition)
      host.put('composition', composition.id, composition)

      const back = restoreComposition(host, 'board')
      expect(back).toEqual(composition)
      restored[name] = back

      const [a, b] = back.rows[0] ?? []
      if (!a || !b) throw new Error('fixture lost its shared placements')
      expect(a.reportId).toBe(b.reportId)
      expect(a.title).toBe('June revenue')
      expect(b.title).toBeUndefined()
      expect(a.parameters).toEqual({ period: 6 })
      expect(restoreDefinition(host, 'revenue').parameters).toEqual({ period: 10 })

      const duplicate = duplicateReportPlacement(a, 'a-copy')
      expect(duplicate.reportId).toBe('revenue')
      expect(duplicate.title).toBe('June revenue')

      const original = restoreDefinition(host, 'revenue')
      const independent = copyReportDefinition(original, 'revenue-copy', 'c1')
      host.put('definition', independent.id, independent)
      host.put('definition', original.id, { ...original, title: 'Net revenue', revision: 'r2' })
      expect(restoreDefinition(host, 'revenue-copy').title).toBe('Revenue')
      expect(restoreDefinition(host, 'revenue').title).toBe('Net revenue')
      expect(restoreComposition(host, 'board').rows[0]?.[0]?.title).toBe('June revenue')

      expect(restoreComposition(host, 'board').rows[1]).toEqual([{ id: 'overview', reportId: 'sales-overview', span: 12 }])
    })
  }

  it('both adapters restore identical compositions', () => {
    expect(Object.keys(restored).sort()).toEqual(['object', 'text'])
    expect(restored.object).toEqual(restored.text)
  })

  it('rejects an execution snapshot in place of a permanent source', () => {
    expect(() => parseReportComposition({ ...fixture.composition, snapshotId: 'expired' })).toThrow()
    expect(() => parseReportDefinition({ ...fixture.definitions[0], snapshotId: 'expired' })).toThrow()
  })
})

describe('shared draft fixture', () => {
  it('editing local state neither saves nor publishes until the host acknowledges', async () => {
    const saved = parseReportDefinition(fixture.draft.saved)
    const edited = parseReportDefinition(fixture.draft.edited)
    const writes: Array<{ value: ReportDefinition; expectedRevision?: string }> = []
    let dispose = () => {}
    const draft = createRoot((cleanup) => {
      dispose = cleanup
      return createReportDraft(saved, { save(value, options) { writes.push({ value, expectedRevision: options.expectedRevision }); return Promise.resolve({ ...value, revision: 'r2' }) } })
    })
    try {
      draft.update(() => edited)
      expect(draft.dirty()).toBe(true)
      expect(draft.saved()).toEqual(saved)
      expect(writes).toHaveLength(0)
      await draft.save()
      expect(writes).toEqual([{ value: edited, expectedRevision: 'r1' }])
      expect(draft.saved()).toEqual({ ...edited, revision: 'r2' })
    } finally { dispose() }
  })
})

describe('shared version fixture', () => {
  for (const version of fixture.versions) {
    it(`${version.name} is ${version.status}`, () => {
      expect(compareReportVersion(version.opened, version.latest)).toBe(version.status)
    })
  }
})

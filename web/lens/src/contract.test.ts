import { readFileSync, readdirSync } from 'node:fs'
import path from 'node:path'
import { describe, expect, it } from 'vitest'
import {
  ContractVersionMismatchError,
  DashboardDocumentSchema,
  parseDocument,
} from './contract'

const goldenDirectory = path.resolve(process.cwd(), '../../pkg/lens/document/testdata')
const goldenFixtures = readdirSync(goldenDirectory)
  .filter((name) => name.endsWith('.json'))
  .sort()
  .map((name) => ({
    name,
    document: JSON.parse(readFileSync(path.join(goldenDirectory, name), 'utf8')) as unknown,
  }))

describe.each(goldenFixtures)('Go golden fixture $name', ({ document }) => {
  it('parses and round-trips at the JSON level', () => {
    expect(parseDocument(document)).toEqual(document)
  })
})

describe('invalid Lens documents', () => {
  const validDocument = () => {
    const goldenFixture = goldenFixtures.find((fixture) => fixture.name === 'small.json')
    if (!goldenFixture) throw new Error('expected small.json Go golden fixture')
    return structuredClone(goldenFixture.document) as Record<string, unknown>
  }

  it('rejects a structurally valid document with an unknown panel kind', () => {
    const document = validDocument()
    const panels = document['panels'] as Array<Record<string, unknown>>
    panels[0]!['kind'] = 'bogus'
    const result = DashboardDocumentSchema.safeParse(document)
    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues.some((issue) => issue.path.join('.') === 'panels.0.kind')).toBe(
        true,
      )
    }
  })

  it('rejects a deep wrong-typed frame rows value', () => {
    const document = validDocument()
    const frames = document['frames'] as Record<string, Record<string, unknown>>
    const firstFrame = Object.values(frames)[0]
    if (!firstFrame) throw new Error('expected at least one frame in small.json')
    firstFrame['rows'] = 'notarray'
    const result = DashboardDocumentSchema.safeParse(document)
    expect(result.success).toBe(false)
    if (!result.success) {
      expect(
        result.error.issues.some((issue) => issue.path.join('.').endsWith('.rows')),
      ).toBe(true)
    }
  })

  it('rejects unknown top-level keys via strict schemas', () => {
    const document = validDocument()
    document['unexpected'] = true
    expect(DashboardDocumentSchema.safeParse(document).success).toBe(false)
  })

  it('reports an incompatible major version with a typed error', () => {
    const goldenFixture = goldenFixtures[0]
    if (!goldenFixture) throw new Error('expected at least one Go golden fixture')
    const document = parseDocument(goldenFixture.document)

    expect(() => parseDocument({ ...document, version: '2.0.0' })).toThrow(
      ContractVersionMismatchError,
    )
  })
})

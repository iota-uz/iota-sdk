import { readFileSync, readdirSync } from 'node:fs'
import path from 'node:path'
import { describe, expect, it } from 'vitest'
import invalidFixture from './test/fixtures/invalid-document.json'
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
  it('rejects an intentionally invalid fixture', () => {
    expect(DashboardDocumentSchema.safeParse(invalidFixture).success).toBe(false)
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

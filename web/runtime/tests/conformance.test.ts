// Host conformance, TS side: runs the shared cases (tests/conformance/cases.json) against the in-memory test host.
// pkg/extruntime/e2e/conformance_test.go runs the same cases and the same package against the Go host and the
// PostgreSQL data store, so a semantic drift between the two fails CI on one side.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import type { Ref } from '../src/schema.ts'
import { createTestHost, fixture, overlayFixture } from '../src/testing.ts'
import * as c from './conformance/package/src/contracts.ts'
import * as s from './conformance/package/src/server.ts'
import * as core from './conformance/kit-core.ts'

type J = Record<string, unknown>
interface Step { via: 'read' | 'write' | 'flow'; step: J; expect: { ok?: unknown; error?: string } }
interface Seed { label?: string; entity?: string; overlay?: string; base?: unknown; values: J }
interface Case { name: string; user: { permissions: string[] }; seed: Seed[]; steps: Step[] }

const doc = JSON.parse(readFileSync(new URL('./conformance/cases.json', import.meta.url), 'utf8')) as { cases: Case[] }
const entities = { item: c.Item, child: c.Child } as const
const isRecord = (v: unknown): v is J => typeof v === 'object' && v !== null && !Array.isArray(v)

/** Replaces {"$ref": label} with the seeded row's ref. */
function bind(v: unknown, refs: Map<string, Ref>): unknown {
  if (Array.isArray(v)) return v.map((x) => bind(x, refs))
  if (!isRecord(v)) return v
  if (typeof v.$ref === 'string') {
    const r = refs.get(v.$ref)
    if (r === undefined) throw new Error(`unknown seed label ${v.$ref}`)
    return { entity: r.entity, key: r.key }
  }
  return Object.fromEntries(Object.entries(v).map(([k, x]) => [k, bind(x, refs)]))
}

/** Replaces refs of seeded rows with {"$ref": label}. */
function label(v: unknown, refs: Map<string, Ref>): unknown {
  if (Array.isArray(v)) return v.map((x) => label(x, refs))
  if (!isRecord(v)) return v
  if (typeof v.entity === 'string' && typeof v.key === 'string' && Object.keys(v).length === 2) {
    for (const [l, r] of refs) if (r.entity === v.entity && r.key === v.key) return { $ref: l }
    return { $ref: '?' }
  }
  return Object.fromEntries(Object.entries(v).map(([k, x]) => [k, label(x, refs)]))
}

describe('host conformance (shared cases)', () => {
  for (const k of doc.cases) {
    it(k.name, async () => {
      const refs = new Map<string, Ref>()
      const fixtures: object[] = []
      for (const seed of k.seed) {
        const values = bind(seed.values, refs) as never
        if (seed.overlay !== undefined) {
          fixtures.push(overlayFixture(c.ItemExtra, bind(seed.base, refs) as never, values))
          continue
        }
        const f = fixture(entities[seed.entity as keyof typeof entities] as typeof c.Item, values)
        fixtures.push(f)
        if (seed.label !== undefined) refs.set(seed.label, f.ref as Ref)
      }
      const host = createTestHost({
        fixtures,
        entities: [c.Item, c.Child, c.ItemExtra, core.User],
        implementations: [s.touch, s.ordinary],
        user: { permissions: k.user.permissions },
      })
      for (const [i, st] of k.steps.entries()) {
        const input = { step: bind(st.step, refs) as never }
        let actual: { ok?: unknown; error?: string }
        try {
          const out =
            st.via === 'read'
              ? await s.conformanceRead.handler(host.query(), input)
              : st.via === 'write'
                ? await host.transaction((tx) => s.conformanceWrite.handler(tx, input))
                : await s.conformanceFlow.handler(host.command(), input)
          actual = { ok: label(out.result, refs) }
        } catch (err) {
          actual = { error: isRecord(err) && typeof err.code === 'string' ? err.code : '<error>' }
        }
        const want = st.expect.error === '*' && actual.error !== undefined ? { error: actual.error } : st.expect
        expect(actual, `step ${i}: ${JSON.stringify(st.step)}`).toEqual(want)
      }
    })
  }
})

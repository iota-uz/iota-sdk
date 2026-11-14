import { implement } from "@iota-uz/sdk/runtime/server"
import type { CommandContext, QueryContext, Tx } from "@iota-uz/sdk/runtime/server"
import * as core from "@iota-kit/core"
import * as c from "./contracts"

type J = Record<string, unknown>
type Reader = Pick<QueryContext, "get" | "find" | "count">

const entities = { item: c.Item, child: c.Child, user: core.User } as const
const overlays = { itemExtra: c.ItemExtra } as const

const entityOf = (name: unknown) => {
  const e = (entities as Record<string, (typeof entities)[keyof typeof entities]>)[String(name)]
  if (e === undefined) throw new Error(`conformance: unknown entity ${String(name)}`)
  return e
}
const overlayOf = (name: unknown) => {
  const o = (overlays as Record<string, (typeof overlays)[keyof typeof overlays]>)[String(name)]
  if (o === undefined) throw new Error(`conformance: unknown overlay ${String(name)}`)
  return o
}

/** The selected fields of a row; a field the row does not carry reads "<absent>". */
const project = (row: unknown, select: unknown): unknown => {
  if (row === null || row === undefined) return null
  const r = row as J
  const fields = Array.isArray(select) ? (select as string[]) : []
  return Object.fromEntries(fields.map((f) => [f, f in r ? (r[f] === undefined ? "<absent>" : r[f]) : "<absent>"]))
}

async function read(q: Reader, s: J): Promise<unknown> {
  switch (s.op) {
    case "find": {
      const rows = await q.find(entityOf(s.entity) as never, {
        ...(s.where === undefined ? {} : { where: s.where }),
        ...(s.orderBy === undefined ? {} : { orderBy: s.orderBy }),
        ...(s.limit === undefined ? {} : { limit: s.limit }),
        ...(s.offset === undefined ? {} : { offset: s.offset }),
      } as never)
      return (rows as unknown[]).map((r) => project(r, s.select))
    }
    case "count":
      return q.count(entityOf(s.entity) as never, (s.where ?? {}) as never)
    case "get":
      return project(await q.get(entityOf(s.entity) as never, s.key as never), s.select)
    case "getOverlay":
      return project(await q.get(overlayOf(s.overlay) as never, s.base as never), s.select)
    default:
      throw new Error(`conformance: unknown read op ${String(s.op)}`)
  }
}

async function write(tx: Tx, s: J): Promise<unknown> {
  switch (s.op) {
    case "insert":
      return project(await tx.insert(entityOf(s.entity) as never, s.values as never), s.select)
    case "update":
      return project(await tx.update(entityOf(s.entity) as never, s.key as never, s.patch as never), s.select)
    case "delete":
      await tx.delete(entityOf(s.entity) as never, s.key as never)
      return null
    case "put":
      return project(await tx.put(overlayOf(s.overlay) as never, s.base as never, s.values as never), s.select)
    case "call":
      return tx.call((s.target === "ordinary" ? c.Ordinary : c.Touch) as never, (s.input ?? {}) as never)
    case "then-fail": {
      await write(tx, s.first as J)
      throw new Error("conformance: abandoned")
    }
    default:
      return read(tx, s)
  }
}

async function flow(ctx: CommandContext, s: J): Promise<unknown> {
  switch (s.op) {
    case "nested-transaction":
      return ctx.transaction(async () => ctx.transaction(async () => true))
    case "call-in-transaction":
      return ctx.transaction(async () => ctx.call(c.Ordinary, {}))
    case "call":
      return ctx.call(c.Ordinary, {})
    default:
      return ctx.transaction((tx) => write(tx, s))
  }
}

export const conformanceWrite = implement(c.Write, async (tx, input) => ({ result: (await write(tx, input.step as J)) as never }))
export const conformanceRead = implement(c.Read, async (q, input) => ({ result: (await read(q, input.step as J)) as never }))
export const conformanceFlow = implement(c.Flow, async (ctx, input) => ({ result: (await flow(ctx, input.step as J)) as never }))
export const touch = implement(c.Touch, async (tx, input) => {
  await tx.insert(c.Child, { label: `touched ${input.name}` })
  return { ok: true }
})
export const ordinary = implement(c.Ordinary, async () => ({ ok: true }))

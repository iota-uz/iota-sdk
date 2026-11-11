import { implement } from "@iota-uz/sdk/runtime/server"
import * as c from "./contracts"

export const recordHit = implement(c.RecordHit, async (ctx, input) => {
  const total = await ctx.transaction(async (tx) => {
    await tx.insert(c.Hit, { label: input.label.trim() })
    return tx.count(c.Hit, {})
  })
  return { total }
})

export const listHits = implement(c.ListHits, async (q) => {
  const rows = await q.find(c.Hit, { orderBy: [{ field: "createdAt" }] })
  return rows.map((r) => ({ label: r.label }))
})

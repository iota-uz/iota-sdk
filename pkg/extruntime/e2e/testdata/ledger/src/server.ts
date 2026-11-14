import { validation } from "@iota-uz/sdk/runtime"
import { implement, writeRule } from "@iota-uz/sdk/runtime/server"
import * as c from "./contracts"

export const append = implement(c.Append, async (ctx, input) => ({ entry: (await ctx.transaction((tx) => tx.insert(c.Entry, { memo: input.memo }))).ref }))

export const remove = implement(c.Remove, async (ctx, input) => {
  await ctx.transaction((tx) => tx.delete(c.Entry, input.entry))
  return {}
})

// Entries are never deleted: every delete of this entity is refused by the rule.
export const appendOnly = writeRule("e2e.ledger.appendOnly", c.Entry, async (_tx, change) => {
  if (change.op === "delete") throw validation(c.NoDeletes.at([], {}))
})

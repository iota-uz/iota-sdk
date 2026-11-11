import { implement } from "@iota-uz/sdk/runtime/server"
import * as c from "./contracts"

export const setProfile = implement(c.SetProfile, async (ctx, input) => {
  const row = await ctx.transaction(async (tx) =>
    tx.put(c.ClientExtra, input.client, {
      ...(input.tier === undefined ? {} : { tier: input.tier }),
      ...(input.score === undefined ? {} : { score: input.score }),
      ...(input.internalNote === undefined ? {} : { internalNote: input.internalNote }),
    }),
  )
  return { tier: row.tier, score: row.score }
})

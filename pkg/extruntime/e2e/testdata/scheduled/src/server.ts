import { schedule } from "@iota-uz/sdk/runtime/server"
import * as c from "./contracts"

export const nightly = schedule("e2e.scheduled.nightly", "* * * * *", async (ctx) => {
  await ctx.transaction(async (tx) => {
    await tx.insert(c.Tick, { slot: ctx.slot, runAs: tx.user.kind + ":" + tx.user.userId })
  })
})

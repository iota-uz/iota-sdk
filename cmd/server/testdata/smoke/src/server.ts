import { implement } from "@iota-uz/sdk/runtime/server"
import * as c from "./contracts"

export const greet = implement(c.Greet, async (ctx, input) => {
  const name = input.name.trim()
  const visits = await ctx.transaction(async (tx) => {
    await tx.insert(c.Visit, { name })
    return tx.count(c.Visit, { name })
  })
  return { greeting: `Hello, ${name}!`, visits }
})

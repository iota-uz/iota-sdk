import { decimal } from "@iota-uz/sdk/runtime"
import { implement } from "@iota-uz/sdk/runtime/server"
import * as c from "./contracts"

async function fetchPremium(upstream: string, number: string): Promise<string> {
  const response = await fetch(`${upstream}/quote/${encodeURIComponent(number)}`)
  if (!response.ok) throw new Error(`upstream answered ${response.status}`)
  return ((await response.json()) as { premium: string }).premium
}

export const quote = implement(c.Quote, async (ctx, input) => {
  const premium = decimal(await fetchPremium(input.upstream, input.number))
  const row = await ctx.transaction(async (tx) => tx.insert(c.Policy, { number: input.number, premium }))
  return { number: row.number, premium: row.premium }
})

export const twoStep = implement(c.TwoStep, async (ctx, input) => {
  await ctx.transaction(async (tx) => tx.insert(c.Policy, { number: input.number, premium: decimal("0.00"), stage: "step-1" }))
  await fetchPremium(input.upstream, input.number) // fails after the first transaction committed
  return { ok: true }
})

export const bulkCreate = implement(c.BulkCreate, async (tx, input) => {
  let created = 0
  for (const number of input.numbers) {
    if (number.startsWith("BAD")) throw c.BadNumber.of({ number })
    await tx.insert(c.Policy, { number, premium: decimal("1.00") })
    created++
  }
  return { created }
})

export const countPolicies = implement(c.CountPolicies, async (q) => ({ count: await q.count(c.Policy) }))

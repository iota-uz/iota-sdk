import { implement } from "@iota-uz/sdk/runtime/server"
import * as c from "./contracts"

export const applyDiscount = implement(c.ApplyDiscount, async (tx, input) => {
  const order = await tx.get(c.Order, input.order)
  if (order === null) throw new Error("order not found")
  const cents = Math.floor((order.cents * (100 - input.percent)) / 100)
  await tx.update(c.Order, input.order, { cents })
  return { cents }
})

export const createOrder = implement(c.CreateOrder, async (ctx, input) => {
  const row = await ctx.transaction(async (tx) => tx.insert(c.Order, { number: input.number, cents: input.cents }))
  return { order: row.ref }
})

export const checkout = implement(c.Checkout, async (ctx, input) =>
  ctx.transaction(async (tx) => {
    const discounted = await tx.call(c.ApplyDiscount, { order: input.order, percent: input.percent ?? 0 })
    await tx.update(c.Order, input.order, { status: "paid" })
    return { cents: discounted.cents, status: "paid" }
  }),
)

export const readOrder = implement(c.ReadOrder, async (q, input) => {
  const order = await q.get(c.Order, input.order)
  return order === null ? null : { cents: order.cents, status: order.status }
})

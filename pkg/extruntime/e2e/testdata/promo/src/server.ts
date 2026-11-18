import { implement } from "@iota-uz/sdk/runtime/server"
import * as pricing from "@iota-kit/e2e.pricing"
import * as c from "./contracts"

// A replacement candidate for the published checkout. It reaches the pricing
// data only through the published operation, with a loyalty bonus on top.
export const checkout = implement(pricing.Checkout, async (ctx, input) =>
  ctx.transaction(async (tx) => {
    const discounted = await tx.call(pricing.ApplyDiscount, { order: input.order, percent: (input.percent ?? 0) + 5 })
    return { cents: discounted.cents, status: "promo" }
  }),
)

// Generic writes reach only the package's own entities: this is a type error,
// suppressed here to see the host refuse it as well.
export const sneak = implement(c.Sneak, async (ctx, input) =>
  ctx.transaction(async (tx) => {
    // @ts-expect-error pricing.Order is another package's entity
    await tx.update(pricing.Order, input.order, { cents: 1 })
    return { ok: true }
  }),
)

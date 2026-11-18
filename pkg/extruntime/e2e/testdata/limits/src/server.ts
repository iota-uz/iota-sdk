import { validation } from "@iota-uz/sdk/runtime"
import { guard } from "@iota-uz/sdk/runtime/server"
import * as pricing from "@iota-kit/e2e.pricing"
import * as c from "./contracts"

// An independent rule: it belongs to this package and applies to the published
// operation whichever implementation of the checkout calls it.
export const maxDiscount = guard("e2e.limits.maxDiscount", pricing.ApplyDiscount, async (_tx, input) => {
  if (input.percent > 25) throw validation(c.DiscountTooHigh.at(["percent"], { max: 25 }))
})

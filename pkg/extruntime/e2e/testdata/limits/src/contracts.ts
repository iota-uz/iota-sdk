import { issue, t } from "@iota-uz/sdk/runtime"

export const DiscountTooHigh = issue("e2e.limits.discountTooHigh", t.object({ max: t.int() }))

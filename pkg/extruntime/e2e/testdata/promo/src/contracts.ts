import { command, t } from "@iota-uz/sdk/runtime"
import * as pricing from "@iota-kit/e2e.pricing"

export const Sneak = command("e2e.promo.sneak", {
  input: t.object({ order: t.ref(pricing.Order) }),
  output: t.object({ ok: t.bool() }),
})

import { command, t } from "@iota-uz/sdk/runtime"

export const PriceWithVat = command("acme.pricing.priceWithVat", {
  input: t.object({ amount: t.int(), percent: t.int() }),
  output: t.object({ total: t.int() }),
})

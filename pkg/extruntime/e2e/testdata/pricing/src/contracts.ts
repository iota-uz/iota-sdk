import { command, entity, field, operation, query, t } from "@iota-uz/sdk/runtime"

export const Order = entity("e2e.pricing.order", {
  number: field.text().unique(),
  cents: field.int(),
  status: field.enum(["open", "paid"]).default("open"),
})

// A published building block: other packages may call it, and guard it.
export const ApplyDiscount = operation("e2e.pricing.applyDiscount", {
  input: t.object({ order: t.ref(Order), percent: t.int() }),
  output: t.object({ cents: t.int() }),
})

export const CreateOrder = command("e2e.pricing.createOrder", {
  input: t.object({ number: t.string(), cents: t.int() }),
  output: t.object({ order: t.ref(Order) }),
})

// The published command other packages may replace.
export const Checkout = command("e2e.pricing.checkout", {
  input: t.object({ order: t.ref(Order), percent: t.int().optional() }),
  output: t.object({ cents: t.int(), status: t.string() }),
})

export const ReadOrder = query("e2e.pricing.readOrder", {
  input: t.object({ order: t.ref(Order) }),
  output: t.object({ cents: t.int(), status: t.string() }).nullable(),
})

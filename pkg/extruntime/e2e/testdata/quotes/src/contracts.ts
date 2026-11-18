import { command, domainError, entity, field, query, t } from "@iota-uz/sdk/runtime"

export const Policy = entity("e2e.quotes.policy", {
  number: field.text().unique(),
  premium: field.decimal({ precision: 18, scale: 2 }),
  stage: field.text().default("quoted"),
})

export const BadNumber = domainError("e2e.quotes.badNumber", t.object({ number: t.string() }))

// Ordinary command: the external quote is fetched outside any transaction.
export const Quote = command("e2e.quotes.quote", {
  input: t.object({ number: t.string(), upstream: t.string() }),
  output: t.object({ number: t.string(), premium: t.decimal({ precision: 18, scale: 2 }) }),
})

// Ordinary command whose first transaction commits before a later step fails.
export const TwoStep = command("e2e.quotes.twoStep", {
  input: t.object({ number: t.string(), upstream: t.string() }),
  output: t.object({ ok: t.bool() }),
})

// Atomic command: the whole handler is one transaction.
export const BulkCreate = command("e2e.quotes.bulkCreate", {
  atomic: true,
  input: t.object({ numbers: t.array(t.string()) }),
  output: t.object({ created: t.int() }),
  errors: [BadNumber],
})

export const CountPolicies = query("e2e.quotes.countPolicies", {
  input: t.object({}),
  output: t.object({ count: t.int() }),
})

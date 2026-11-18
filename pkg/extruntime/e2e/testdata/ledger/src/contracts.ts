// A ledger whose entries are append-only: a write rule refuses to delete one.
import { command, entity, field, issue, t } from "@iota-uz/sdk/runtime"

export const Entry = entity("e2e.ledger.entry", { memo: field.text({ maxLength: 80 }) }, { access: { read: [{ rows: "all" }] } })

export const NoDeletes = issue("e2e.ledger.noDeletes", t.object({}))

export const Append = command("e2e.ledger.append", {
  input: t.object({ memo: t.string() }),
  output: t.object({ entry: t.ref(Entry) }),
})

export const Remove = command("e2e.ledger.remove", {
  input: t.object({ entry: t.ref(Entry) }),
  output: t.object({}),
})

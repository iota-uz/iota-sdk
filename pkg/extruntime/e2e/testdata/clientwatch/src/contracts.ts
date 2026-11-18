import { command, entity, field, issue, query, t } from "@iota-uz/sdk/runtime"
import * as crm from "@iota-kit/crm"

// One row per crm.client.updated.v1 delivery the subscription handled.
export const Seen = entity("e2e.clientwatch.seen", {
  clientId: field.text(),
  firstName: field.text(),
  changed: field.text(),
  eventId: field.text().unique(),
})

export const Abandoned = issue("e2e.clientwatch.abandoned", t.object({}))

// Renames a client through the platform's operation, then fails: the whole
// transaction, the client write and its event roll back.
export const RenameThenFail = command("e2e.clientwatch.renameThenFail", {
  input: t.object({ client: t.ref(crm.Client), firstName: t.string() }),
  output: t.object({ ok: t.bool() }),
})

export const SeenRows = query("e2e.clientwatch.seenRows", {
  input: t.object({}),
  output: t.array(t.object({ clientId: t.string(), firstName: t.string(), changed: t.string() })),
})

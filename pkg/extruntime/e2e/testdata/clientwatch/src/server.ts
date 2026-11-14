import { validation } from "@iota-uz/sdk/runtime"
import { implement, subscribe } from "@iota-uz/sdk/runtime/server"
import * as crm from "@iota-kit/crm"
import * as c from "./contracts"

// The CRM module publishes crm.client.updated.v1 in the transaction of the
// client write; the subscription sees it only after that transaction commits.
export const onClientUpdated = subscribe("e2e.clientwatch.onClientUpdated", crm.Updated, async (tx, event) => {
  await tx.insert(c.Seen, {
    clientId: event.payload.clientId,
    firstName: event.payload.firstName,
    changed: event.payload.changed.join(","),
    eventId: event.id,
  })
})

export const renameThenFail = implement(c.RenameThenFail, async (ctx, input) =>
  ctx.transaction(async (tx) => {
    await tx.call(crm.UpsertClient, { client: input.client, firstName: input.firstName })
    throw validation(c.Abandoned.at([], {}))
  }),
)

export const seenRows = implement(c.SeenRows, async (q) => {
  const rows = await q.find(c.Seen, { orderBy: [{ field: "createdAt" }] })
  return rows.map((r) => ({ clientId: r.clientId, firstName: r.firstName, changed: r.changed }))
})

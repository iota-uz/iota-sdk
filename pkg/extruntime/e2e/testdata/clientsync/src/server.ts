import { decode, ref, t } from "@iota-uz/sdk/runtime"
import { endpoint, subscribe } from "@iota-uz/sdk/runtime/server"
import * as crm from "@iota-kit/crm"

// A subscription (a background root: no HTTP request, no session) completes
// the client the form created, through crm.operations.upsertClient.
export const onClientCreated = subscribe("e2e.clientsync.onClientCreated", crm.Created, async (tx, event) => {
  await tx.call(crm.UpsertClient, { client: ref(crm.Client, event.payload.clientId), comments: "synced by subscription" })
})

// A webhook (a public endpoint acting as its trigger owner) creates a client.
const Lead = t.object({ firstName: t.string(), lastName: t.string(), phone: t.string() })
export const ingest = endpoint("e2e.clientsync.ingest", { method: "POST", path: "/ingest", public: true }, async (ctx) => {
  const lead = decode(Lead, ctx.request.body)
  const out = await ctx.transaction((tx) => tx.call(crm.UpsertClient, { ...lead, comments: "from webhook" }))
  return { status: 201, body: { client: out.client.key } }
})

import { implement, subscribe } from "@iota-uz/sdk/runtime/server"
import * as acme from "@iota-kit/acme.insurance"
import * as c from "./contracts"

// A foreign event drives a write to an own entity.
export const onRenewed = subscribe("partner.loyalty.onRenewed", acme.PolicyRenewed, async (tx, e) => {
  const flag = await tx.insert(c.Flag, { policy: e.subject, note: null })
  await tx.emit(c.FlagRaised, { subject: { entity: c.Flag.id, key: flag.key }, payload: { reason: "renewed" } })
})

// A replacement candidate for a foreign atomic command: it receives Tx.
export const activate = implement(acme.ActivatePolicy, async (tx, input) => {
  const policy = await tx.get(acme.Policy, input.policy)
  return { status: policy?.status ?? "missing" }
})

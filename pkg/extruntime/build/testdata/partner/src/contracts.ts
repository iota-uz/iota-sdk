import { entity, event, field, t } from "@iota-uz/sdk/runtime"
import * as acme from "@iota-kit/acme.insurance"

export const Flag = entity("partner.loyalty.flag", {
  policy: field.ref(acme.Policy),
  note: field.text().optional(),
})

export const FlagRaised = event("partner.loyalty.flagRaised.v1", {
  subject: Flag,
  payload: t.object({ reason: t.string() }),
})

// A generic renewal campaign: policies about to expire get an SMS; replies
// are collected until the reply window closes.
import { command, entity, field, notice, query, t, workflow } from "@iota-uz/sdk/runtime"

export const Policy = entity("e2e.renewals.policy", {
  number: field.text().unique(),
  holder: field.text(),
  phone: field.text(),
  expiresOn: field.date().index(),
  status: field.enum(["active", "lapsed"]).default("active"),
})

// One row per policy the campaign addressed.
export const Target = entity("e2e.renewals.target", {
  campaign: field.text().index(),
  number: field.text(),
  phone: field.text(),
  outcome: field.enum(["pending", "sent", "replied", "noReply"]).default("pending"),
  reply: field.text().optional(),
})

export const Reminder = notice("e2e.renewals.reminder.v1", {
  subject: Policy,
  payload: t.object({ number: t.string(), holder: t.string(), expiresOn: t.date() }),
})

export const Campaign = workflow("e2e.renewals.campaign", {
  input: t.object({ from: t.date(), to: t.date(), replyWindowMs: t.int() }),
  output: t.object({ outcomes: t.array(t.object({ number: t.string(), outcome: t.string() })) }),
  signals: { reply: t.object({ number: t.string(), text: t.string() }) },
})

export const AddPolicy = command("e2e.renewals.addPolicy", {
  input: t.object({ number: t.string(), holder: t.string(), phone: t.string(), expiresOn: t.date(), lapsed: t.bool().optional() }),
  output: t.object({ policy: t.ref(Policy) }),
})

export const StartCampaign = command("e2e.renewals.startCampaign", {
  input: t.object({ from: t.date(), to: t.date(), replyWindowMs: t.int() }),
  output: t.object({ instance: t.string() }),
})

export const Targets = query("e2e.renewals.targets", {
  input: t.object({ campaign: t.string() }),
  output: t.array(t.object({ number: t.string(), outcome: t.string(), reply: t.string().nullable() })),
})

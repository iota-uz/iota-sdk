import { command, entity, field, permission, query, t, workflow } from "@iota-uz/sdk/runtime"

export const Request = entity("e2e.approvals.request", {
  title: field.text(),
  status: field.enum(["pending", "approved", "rejected", "expired"]).default("pending"),
})

export const Review = workflow("e2e.approvals.review", {
  input: t.object({ request: t.ref(Request), approver: t.string(), waitSeconds: t.int(), cooldownSeconds: t.int() }),
  output: t.object({ outcome: t.string() }),
})

export const Submit = command("e2e.approvals.submit", {
  input: t.object({ title: t.string(), approver: t.string(), waitSeconds: t.int(), cooldownSeconds: t.int() }),
  output: t.object({ request: t.ref(Request), instance: t.string() }),
})

export const StatusOf = query("e2e.approvals.statusOf", {
  input: t.object({ request: t.ref(Request) }),
  output: t.object({ status: t.string() }).nullable(),
})

// A committee: any holder of the permission decides.
export const Committee = permission("e2e.approvals.committee", { label: { en: "Approvals committee" } })

export const CommitteeReview = workflow("e2e.approvals.committeeReview", {
  input: t.object({ request: t.ref(Request) }),
  output: t.object({ outcome: t.string(), by: t.string().optional() }),
})

export const SubmitToCommittee = command("e2e.approvals.submitToCommittee", {
  input: t.object({ title: t.string() }),
  output: t.object({ request: t.ref(Request), instance: t.string() }),
})

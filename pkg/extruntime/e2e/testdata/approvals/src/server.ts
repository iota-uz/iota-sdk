import { ref, t } from "@iota-uz/sdk/runtime"
import { implement } from "@iota-uz/sdk/runtime/server"
import * as core from "@iota-kit/core"
import * as c from "./contracts"

export const submit = implement(c.Submit, async (ctx, input) =>
  ctx.transaction(async (tx) => {
    const row = await tx.insert(c.Request, { title: input.title })
    const started = await tx.start(
      c.Review,
      { request: row.ref, approver: input.approver, waitSeconds: input.waitSeconds, cooldownSeconds: input.cooldownSeconds },
      { key: `review:${row.ref.key}` },
    )
    return { request: row.ref, instance: started.instance }
  }),
)

export const review = implement(c.Review, async (wf, input) => {
  const verdict = await wf.task("approve", {
    subject: input.request,
    assignee: ref(core.User, input.approver),
    decision: t.object({ approved: t.bool(), note: t.string().optional() }),
    timeout: `${input.waitSeconds}s`,
  })
  if (verdict.outcome === "timeout") {
    await wf.transaction("expire", async (tx) => {
      await tx.update(c.Request, input.request, { status: "expired" })
      return null
    })
    return { outcome: "expired" }
  }
  // A timer between the decision and its effect.
  await wf.sleep("cooldown", `${input.cooldownSeconds}s`)
  const status = verdict.decision.approved ? "approved" : "rejected"
  await wf.transaction("apply", async (tx) => {
    await tx.update(c.Request, input.request, { status })
    return null
  })
  return { outcome: status }
})

export const statusOf = implement(c.StatusOf, async (q, input) => {
  const row = await q.get(c.Request, input.request)
  return row === null ? null : { status: row.status }
})

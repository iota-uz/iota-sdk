import { implement } from "@iota-uz/sdk/runtime/server"
import * as c from "./contracts"

const TAG = "__TAG__"

export const start = implement(c.Start, async (ctx, input) =>
  ctx.transaction(async (tx) => {
    const started = await tx.start(c.Job, { label: input.label }, { key: `job:${input.label}` })
    return { instance: started.instance, startedBy: TAG }
  }),
)

export const job = implement(c.Job, async (wf, input) => {
  await wf.waitFor("go", c.Job.signals.go)
  await wf.transaction("log", async (tx) => {
    await tx.insert(c.Log, { label: input.label, tag: TAG /* EXTRA_WRITE */ })
    return null
  })
  return { tag: TAG, label: input.label }
})

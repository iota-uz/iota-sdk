import { implement } from "@iota-uz/sdk/runtime/server"
import * as pricing from "@iota-kit/e2e.pricing"
import * as c from "./contracts"

export const start = implement(c.Start, async (ctx, input) =>
  ctx.transaction(async (tx) => {
    const started = await tx.start(c.Run, input, { key: `run:${input.first.key}` })
    return { instance: started.instance }
  }),
)

// The workflow calls another package's published command; each call selects
// the implementation bound at the moment it runs.
export const run = implement(c.Run, async (wf, input) => {
  await wf.waitFor("go1", c.Run.signals.go1)
  const first = await wf.call("checkout", pricing.Checkout, { order: input.first, percent: input.percent })
  await wf.waitFor("go2", c.Run.signals.go2)
  const second = await wf.call("checkout", pricing.Checkout, { order: input.second, percent: input.percent })
  return { first: `${first.status}:${first.cents}`, second: `${second.status}:${second.cents}` }
})

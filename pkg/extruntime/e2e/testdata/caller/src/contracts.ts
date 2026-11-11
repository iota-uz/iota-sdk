import { command, t, workflow } from "@iota-uz/sdk/runtime"
import * as pricing from "@iota-kit/e2e.pricing"

export const Run = workflow("e2e.caller.run", {
  input: t.object({ first: t.ref(pricing.Order), second: t.ref(pricing.Order), percent: t.int() }),
  output: t.object({ first: t.string(), second: t.string() }),
  signals: { go1: t.object({}), go2: t.object({}) },
})

export const Start = command("e2e.caller.start", {
  input: t.object({ first: t.ref(pricing.Order), second: t.ref(pricing.Order), percent: t.int() }),
  output: t.object({ instance: t.string() }),
})

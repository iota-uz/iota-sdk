import { command, entity, field, t, workflow } from "@iota-uz/sdk/runtime"

export const Log = entity("e2e.versioned.log", {
  label: field.text(),
  tag: field.text(),
  // EXTRA_FIELD
})

export const Job = workflow("e2e.versioned.job", {
  input: t.object({ label: t.string() }),
  output: t.object({ tag: t.string(), label: t.string() }),
  signals: { go: t.object({ note: t.string().optional() }) },
})

export const Start = command("e2e.versioned.start", {
  input: t.object({ label: t.string() }),
  output: t.object({ instance: t.string(), startedBy: t.string() }),
})

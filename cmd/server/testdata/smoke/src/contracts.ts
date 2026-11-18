import { command, entity, field, t } from "@iota-uz/sdk/runtime"

export const Visit = entity("smoke.greeter.visit", {
  name: field.text({ maxLength: 80 }),
})

export const Greet = command("smoke.greeter.greet", {
  input: t.object({ name: t.string() }),
  output: t.object({ greeting: t.string(), visits: t.int() }),
})

import { entity, field } from "@iota-uz/sdk/runtime"

// One row per slot the schedule ran.
export const Tick = entity("e2e.scheduled.tick", {
  slot: field.text().unique(),
  runAs: field.text(),
})

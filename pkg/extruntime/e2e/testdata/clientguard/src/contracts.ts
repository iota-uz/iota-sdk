import { issue, t } from "@iota-uz/sdk/runtime"

export const NameBlocked = issue("e2e.clientguard.nameBlocked", t.object({ name: t.string() }))

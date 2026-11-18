import { validation } from "@iota-uz/sdk/runtime"
import { guard } from "@iota-uz/sdk/runtime/server"
import * as crm from "@iota-kit/crm"
import * as c from "./contracts"

// An independent rule on the platform's client write: it applies to every
// caller of the operation, whichever implementation of the form command runs.
export const noBlockedNames = guard("e2e.clientguard.noBlockedNames", crm.UpsertClient, async (_tx, input) => {
  if (input.firstName === "Blocked") throw validation(c.NameBlocked.at(["firstName"], { name: input.firstName }))
})

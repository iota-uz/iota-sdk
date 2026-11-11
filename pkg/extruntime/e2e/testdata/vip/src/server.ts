import { implement } from "@iota-uz/sdk/runtime/server"
import * as crm from "@iota-kit/crm"
import * as c from "./contracts"

// A replacement candidate for the platform's client form command. Its own
// algorithm (last names in capitals) writes through the published operation,
// so the platform's client invariants and the guards of other packages hold.
export const saveClient = implement(crm.SaveClient, async (ctx, input) =>
  ctx.transaction(async (tx) =>
    tx.call(crm.UpsertClient, {
      ...input,
      ...(input.lastName === undefined ? {} : { lastName: input.lastName.toUpperCase() }),
    }),
  ),
)

// Generic writes to a native entity: a type error, suppressed here to see the
// host refuse it as well.
export const sneak = implement(c.Sneak, async (ctx, input) =>
  ctx.transaction(async (tx) => {
    // @ts-expect-error crm.Client is a native entity; it changes through crm.UpsertClient
    await tx.update(crm.Client, input.client, { firstName: "Hacked" })
    return { ok: true }
  }),
)

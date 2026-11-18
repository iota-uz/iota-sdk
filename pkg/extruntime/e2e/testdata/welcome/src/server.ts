import { implement, recipient, subscribe, templates } from "@iota-uz/sdk/runtime/server"
import * as core from "@iota-kit/core"
import * as c from "./contracts"

// Every new account gets an onboarding workflow, started in the subscription's own transaction.
export const onUserCreated = subscribe("e2e.welcome.onUserCreated", core.Created, async (tx, event) => {
  await tx.start(c.Onboard, {
    accountId: event.payload.userId,
    name: `${event.payload.firstName} ${event.payload.lastName}`,
    accountEmail: event.payload.email,
  })
})

export const onboard = implement(c.Onboard, async (wf, input) => {
  // The welcome goes to the account's sponsor: an external contact record.
  const contact = await wf.transaction("register", async (tx) => {
    const row = await tx.insert(c.Contact, {
      name: `Sponsor of ${input.name}`,
      phone: `+99890${input.accountId.padStart(7, "0")}`,
      email: `sponsor-${input.accountId}@partner.example`,
      accountId: input.accountId,
    })
    await tx.notify(c.Welcome, {
      subject: row.ref,
      payload: { name: input.name },
      to: [recipient.record(row.ref)],
      channels: ["sms", "email"],
    })
    return { ref: row.ref }
  })
  return { contact: contact.ref }
})

export const welcomeTemplates = templates(c.Welcome, {
  sms: ({ payload }) => `Welcome, ${payload.name}!`,
  email: ({ payload }) => ({ subject: `Welcome ${payload.name}`, html: `<p>Welcome, ${payload.name}</p>`, text: `Welcome, ${payload.name}` }),
})

export const contactDetails = implement(c.ContactDetails, async (q, record) => {
  const contact = await q.get(c.Contact, record)
  if (contact === null) return {}
  return {
    ...(contact.phone === null ? {} : { sms: [contact.phone] }),
    ...(contact.email === null ? {} : { email: [contact.email] }),
  }
})

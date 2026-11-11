import { contactResolver, entity, field, notice, t, workflow } from "@iota-uz/sdk/runtime"

// An outside party: not an ERP user, reachable through the contact data kept here.
export const Contact = entity("e2e.welcome.contact", {
  name: field.text(),
  phone: field.text().optional(),
  email: field.text().optional(),
  accountId: field.text().unique(),
})

export const Welcome = notice("e2e.welcome.welcome.v1", {
  subject: Contact,
  payload: t.object({ name: t.string() }),
})

export const ContactDetails = contactResolver("e2e.welcome.contactDetails", Contact)

export const Onboard = workflow("e2e.welcome.onboard", {
  input: t.object({ accountId: t.string(), name: t.string(), accountEmail: t.string() }),
  output: t.object({ contact: t.ref(Contact) }),
})

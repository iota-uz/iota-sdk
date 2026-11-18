import { command, contactResolver, deliveryPolicy, entity, field, notice, t } from "@iota-uz/sdk/runtime"
import * as core from "@iota-kit/core"

// An outside customer with its own devices and its own consent: not an ERP user.
export const Customer = entity("e2e.alerts.customer", {
  name: field.text(),
  phone: field.text(),
  email: field.text(),
  phoneDevice: field.text(),
  tabletDevice: field.text(),
  emailConsent: field.bool().default(true),
})

export const Statement = notice("e2e.alerts.statement.v1", {
  subject: Customer,
  payload: t.object({ amount: t.string() }),
})

export const CustomerContacts = contactResolver("e2e.alerts.customerContacts", Customer)

// Consent is the package's business: the SDK only asks, per notice, channel and target.
export const CustomerConsent = deliveryPolicy("e2e.alerts.customerConsent", Customer)

export const SendStatement = command("e2e.alerts.sendStatement", {
  atomic: true,
  input: t.object({ customer: t.ref(Customer), user: t.ref(core.User), amount: t.string() }),
  output: t.object({ sent: t.bool() }),
})

export const AddCustomer = command("e2e.alerts.addCustomer", {
  atomic: true,
  input: t.object({ name: t.string(), emailConsent: t.bool() }),
  output: t.object({ customer: t.ref(Customer) }),
})

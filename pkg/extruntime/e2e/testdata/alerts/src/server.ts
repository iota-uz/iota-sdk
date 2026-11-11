import { implement, recipient, templates } from "@iota-uz/sdk/runtime/server"
import * as c from "./contracts"

export const addCustomer = implement(c.AddCustomer, async (tx, input) => {
  const slug = input.name.toLowerCase()
  const row = await tx.insert(c.Customer, {
    name: input.name,
    phone: `+998-${slug}`,
    email: `${slug}@customer.example`,
    phoneDevice: `${slug}-phone`,
    tabletDevice: `${slug}-tablet`,
    emailConsent: input.emailConsent,
  })
  return { customer: row.ref }
})

// One statement for the external customer (sms, email and its two push devices) and one for an ERP user (inbox and push).
export const sendStatement = implement(c.SendStatement, async (tx, input) => {
  await tx.notify(c.Statement, {
    subject: input.customer,
    payload: { amount: input.amount },
    to: [recipient.record(input.customer)],
    channels: ["sms", "email", "push"],
  })
  await tx.notify(c.Statement, {
    subject: input.customer,
    payload: { amount: input.amount },
    to: [recipient.user(input.user)],
    channels: ["inApp", "push"],
  })
  return { sent: true }
})

export const statementTemplates = templates(c.Statement, {
  sms: ({ payload }) => `Statement: ${payload.amount}`,
  email: ({ payload }) => ({ subject: `Statement ${payload.amount}`, html: `<p>${payload.amount}</p>`, text: payload.amount }),
  push: ({ payload }) => ({ title: "Statement", body: payload.amount, data: { amount: payload.amount } }),
  inApp: ({ payload }) => ({ title: "Statement", body: payload.amount }),
})

export const customerContacts = implement(c.CustomerContacts, async (q, record) => {
  const customer = await q.get(c.Customer, record)
  if (customer === null) return {}
  return { sms: [customer.phone], email: [customer.email], push: [customer.phoneDevice, customer.tabletDevice] }
})

export const customerConsent = implement(c.CustomerConsent, async (q, request) => {
  const customer = await q.get(c.Customer, request.recipient.ref)
  if (customer === null) return { allow: false, reason: "unknown_customer" }
  if (request.channel === "email" && !customer.emailConsent) return { allow: false, reason: "email_opt_out" }
  return { allow: true }
})

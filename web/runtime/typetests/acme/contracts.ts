// Pure contracts graph of the example package "acme.insurance".
import {
  $, column, command, contactResolver, deliveryPolicy, domainError, entity, event, field, issue, notice, operation, overlay,
  permission, query, route, t, workflow,
} from '../../src/index.ts'
import * as kit from '../kit.ts'

export const ReadPolicies = permission('acme.insurance.readPolicies', { label: { en: 'Read policies', ru: 'Просмотр полисов' } })

export const Customer = entity('acme.insurance.customer', {
  name: field.text(),
  phone: field.text().optional(),
  owner: field.ref(kit.User).optional(),
})

export const Policy = entity(
  'acme.insurance.policy',
  {
    number: field.text().unique(),
    holder: field.ref(Customer),
    premium: field.decimal({ precision: 18, scale: 2 }),
    expiresOn: field.date().index(),
    status: field.enum(['draft', 'active', 'expired']).default('draft'),
    notes: field.text().optional(),
    extra: field.json<{ tags: string[] }>().optional(),
  },
  { access: { read: [{ when: ReadPolicies, rows: 'all' }, { rows: { 'holder.owner': $.user } }] } },
)

export const ClientExtra = overlay('acme.insurance.clientExtra', kit.Client, {
  loyaltyTier: field.enum(['bronze', 'silver', 'gold']).optional(),
  points: field.int().default(0),
})

export const ClientsTierColumn = column(kit.Clients, {
  id: 'acme.insurance.clientsTier',
  field: ClientExtra.fields.loyaltyTier,
  label: { en: 'Tier', ru: 'Уровень', uz: 'Daraja' },
  filterable: true,
  sortable: true,
  exportable: true,
})

export const NotActive = domainError('acme.insurance.notActive', t.object({ status: t.string() }))
export const TooLong = issue('acme.insurance.tooLong', t.object({ max: t.int() }))

export const ExtendExpiry = operation('acme.insurance.extendExpiry', {
  input: t.object({ policy: t.ref(Policy), months: t.int() }),
  output: t.object({ expiresOn: t.date() }),
  errors: [NotActive],
})
export const RenewPolicy = command('acme.insurance.renewPolicy', {
  input: t.object({ policy: t.ref(Policy) }),
  output: t.object({ expiresOn: t.date(), quote: t.decimal({ precision: 18, scale: 2 }) }),
  errors: [NotActive],
  permission: ReadPolicies,
})
export const ActivatePolicy = command('acme.insurance.activatePolicy', {
  atomic: true,
  input: t.object({ policy: t.ref(Policy) }),
  output: t.object({ status: t.string() }),
})
export const Expiring = query('acme.insurance.expiring', {
  input: t.object({ before: t.date() }),
  output: t.array(t.object({ policy: t.ref(Policy), number: t.string() })),
})

export const PolicyRenewed = event('acme.insurance.policyRenewed.v1', {
  subject: Policy,
  payload: t.object({ expiresOn: t.date(), premium: t.decimal({ precision: 18, scale: 2 }), version: t.int64() }),
})
export const RenewalReminder = notice('acme.insurance.renewalReminder.v1', {
  subject: Policy,
  payload: t.object({ number: t.string(), expiresOn: t.date() }),
})
export const RenewalFollowUp = workflow('acme.insurance.renewalFollowUp', {
  input: t.object({ policy: t.ref(Policy) }),
  output: t.object({ renewed: t.bool() }),
  signals: { decision: t.object({ approved: t.bool() }) },
})

export const PolicyRoute = route('acme.insurance.policy', { path: '/insurance/policies/:id', params: t.object({ id: t.uuid() }) })

export const CustomerContacts = contactResolver('acme.insurance.customerContacts', Customer)
export const CustomerConsent = deliveryPolicy('acme.insurance.customerConsent', Customer)

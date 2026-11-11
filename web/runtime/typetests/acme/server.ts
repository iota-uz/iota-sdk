// Server graph of "acme.insurance": implementations, guards, subscriptions, templates, schedule, endpoint.
import { decimal, decode, int64, isoDate, ref, t, validation, type ISODate } from '../../src/index.ts'
import {
  endpoint, guard, implement, precheck, recipient, schedule, subscribe, templates, writeRule,
} from '../../src/server.ts'
import * as kit from '../kit.ts'
import * as c from './contracts.ts'

function addMonths(date: ISODate, months: number): ISODate {
  const d = new Date(`${date}T00:00:00Z`)
  d.setUTCMonth(d.getUTCMonth() + months)
  return isoDate(d)
}

// --- new module: entity → query → command
export const expiring = implement(c.Expiring, async (q, input) => {
  const rows = await q.find(c.Policy, {
    where: { expiresOn: { lt: input.before }, status: 'active', 'holder.owner': { isNull: false } },
    orderBy: [{ field: 'expiresOn' }],
    limit: 100,
  })
  return rows.map((r) => ({ policy: ref(c.Policy, r.key), number: r.number }))
})

export const extendExpiry = implement(c.ExtendExpiry, async (tx, input) => {
  const policy = await tx.get(c.Policy, input.policy)
  if (policy === null) throw validation(c.TooLong.at(['policy'], { max: 0 }))
  if (policy.status !== 'active') throw c.NotActive.of({ status: policy.status })
  const expiresOn = addMonths(policy.expiresOn, input.months)
  await tx.update(c.Policy, input.policy, { expiresOn })
  await tx.emit(c.PolicyRenewed, {
    subject: input.policy,
    payload: { expiresOn, premium: policy.premium, version: int64(1n) },
  })
  return { expiresOn }
})

// --- ordinary command: external call → transaction → response
export const renew = implement(c.RenewPolicy, async (ctx, input) => {
  const policy = await ctx.query.get(c.Policy, input.policy)
  if (policy === null) throw c.NotActive.of({ status: 'missing' })
  const response = await fetch(`https://quotes.example.uz/${policy.number}`)
  const quote = decimal(((await response.json()) as { premium: string }).premium)
  const renewed = await ctx.transaction(async (tx) => tx.call(c.ExtendExpiry, { policy: input.policy, months: 12 }))
  return { expiresOn: renewed.expiresOn, quote }
})

export const activate = implement(c.ActivatePolicy, async (tx, input) => {
  await tx.update(c.Policy, input.policy, { status: 'active' })
  await tx.start(c.RenewalFollowUp, { policy: input.policy }, { key: `activate:${input.policy.key}` })
  return { status: 'active' }
})

// --- event → subscription → workflow start
export const onRenewed = subscribe('acme.insurance.onRenewed', c.PolicyRenewed, async (tx, e) => {
  await tx.start(c.RenewalFollowUp, { policy: e.subject })
})

// --- workflow with task / sleep / call / notify
export const renewalFollowUp = implement(c.RenewalFollowUp, async (wf, input) => {
  const policy = await wf.transaction('load', (tx) => tx.get(c.Policy, input.policy))
  if (policy === null) return { renewed: false }
  await wf.sleep('cooldown', '1d')
  const verdict = await wf.task('approve', {
    subject: input.policy,
    assignee: wf.user,
    decision: t.object({ approved: t.bool() }),
    timeout: '3d',
  })
  if (verdict.outcome === 'timeout' || !verdict.decision.approved) return { renewed: false }
  const renewed = await wf.call('renew', c.RenewPolicy, { policy: input.policy })
  const sms = await wf.step('sms', { output: t.object({ id: t.string() }) }, async ({ idempotencyKey }) => ({ id: idempotencyKey }))
  await wf.step('log', async () => {
    return { sent: sms.id, at: renewed.expiresOn }
  })
  const signal = await wf.waitFor('decision', c.RenewalFollowUp.signals.decision, { timeout: '1d' })
  const at = await wf.now('finished')
  await wf.transaction('remind', async (tx) => {
    await tx.notify(c.RenewalReminder, {
      subject: input.policy,
      payload: { number: policy.number, expiresOn: renewed.expiresOn },
      to: [recipient.user(ref(kit.User, '42')), recipient.record(policy.holder), recipient.address({ sms: '+998901234567' })],
      channels: ['sms', 'email'],
    })
    return null
  })
  return { renewed: signal.outcome === 'received' ? signal.payload.approved : at !== '' }
})

// --- notices
export const reminderTemplates = templates(c.RenewalReminder, {
  sms: ({ payload }) => `Policy ${payload.number} expires on ${payload.expiresOn}`,
  email: ({ payload, locale }) => ({ subject: `Renewal ${payload.number}`, html: `<p>${locale}</p>`, text: payload.number }),
  inApp: ({ payload }) => ({ title: payload.number, body: payload.expiresOn }),
  push: ({ payload }) => ({ title: payload.number, body: payload.expiresOn, data: { policy: payload.number } }),
})

export const customerContacts = implement(c.CustomerContacts, async (q, record) => {
  const customer = await q.get(c.Customer, record)
  return customer?.phone ? { sms: [customer.phone] } : {}
})
export const customerConsent = implement(c.CustomerConsent, async (_q, request) => ({
  allow: request.channel !== 'sms' || request.target.startsWith('+998'),
}))

// --- overlay of a native entity, replacement of a native command, independent guards
export const rejectIncoming = implement(kit.RejectIncoming, async (tx, input) => {
  await tx.call(kit.MarkClientContacted, { client: input.client })
  const extra = await tx.get(c.ClientExtra, input.client)
  await tx.put(c.ClientExtra, input.client, { loyaltyTier: extra?.loyaltyTier === 'gold' ? 'silver' : null })
  return { rejected: true }
})
export const sendInvoice = implement(kit.SendInvoice, async (ctx, input) => {
  const client = await ctx.query.get(kit.Client, input.client)
  return { number: client?.name ?? 'unknown' }
})
export const rejectGuard = guard('acme.insurance.rejectGuard', kit.RejectIncoming, async (_tx, input) => {
  if (input.reason === '') throw validation(c.TooLong.at(['reason'], { max: 1 }))
})
export const extendGuard = guard('acme.insurance.extendGuard', c.ExtendExpiry, async (_tx, input) => {
  if (input.months > 36) throw validation(c.TooLong.at(['months'], { max: 36 }))
})
export const renewPrecheck = precheck('acme.insurance.renewPrecheck', c.RenewPolicy, async (q, input) => {
  const policy = await q.get(c.Policy, input.policy)
  if (policy?.status === 'expired') throw c.NotActive.of({ status: policy.status })
})
export const notesRule = writeRule('acme.insurance.notesRule', c.Policy, async (_tx, { proposed, changed }) => {
  if (proposed !== null && changed.includes('notes') && (proposed.notes?.length ?? 0) > 10) {
    throw validation(c.TooLong.at(['notes'], { max: 10 }))
  }
})

// --- schedule and endpoint
export const nightly = schedule('acme.insurance.nightly', { cron: '0 3 * * *', timezone: 'Asia/Tashkent' }, async (ctx) => {
  const due = await ctx.query.count(c.Policy, { status: 'draft' })
  ctx.log.info('draft policies', { due })
})

const WebhookBody = t.object({ policy: t.ref(c.Policy) })
export const paymentWebhook = endpoint(
  'acme.insurance.paymentWebhook',
  { method: 'POST', path: '/insurance/webhook', public: true },
  async (ctx) => {
    const body = decode(WebhookBody, ctx.request.body)
    await ctx.call(c.ActivatePolicy, { policy: body.policy })
    return { status: 204 }
  },
)

import { decode, ref, t } from "@iota-uz/sdk/runtime"
import { endpoint, implement, recipient, templates } from "@iota-uz/sdk/runtime/server"
import * as c from "./contracts"

export const addPolicy = implement(c.AddPolicy, async (ctx, input) => {
  const row = await ctx.transaction(async (tx) =>
    tx.insert(c.Policy, {
      number: input.number, holder: input.holder, phone: input.phone, expiresOn: input.expiresOn,
      status: input.lapsed === true ? "lapsed" : "active",
    }),
  )
  return { policy: row.ref }
})

export const startCampaign = implement(c.StartCampaign, async (ctx, input) =>
  ctx.transaction(async (tx) => {
    const started = await tx.start(c.Campaign, input, { key: `campaign:${input.from}:${input.to}` })
    return { instance: started.instance }
  }),
)

export const campaign = implement(c.Campaign, async (wf, input) => {
  // 1. Select the policies expiring in the window and register a target for each (one transaction).
  const due = await wf.transaction("select", async (tx) => {
    const policies = await tx.find(c.Policy, {
      where: { status: "active", expiresOn: { gte: input.from, lte: input.to } },
      orderBy: [{ field: "expiresOn" }, { field: "number" }],
    })
    const rows: { number: string; holder: string; phone: string; expiresOn: string; policy: string }[] = []
    for (const p of policies) {
      await tx.insert(c.Target, { campaign: wf.instance.id, number: p.number, phone: p.phone })
      rows.push({ number: p.number, holder: p.holder, phone: p.phone, expiresOn: p.expiresOn, policy: p.ref.key })
    }
    return rows
  })

  // 2. One SMS per policy. The occurrence name carries the policy number: a replay or a restart
  //    finds the recorded send and does not send again.
  for (const d of due) {
    await wf.transaction(`send:${d.number}`, async (tx) => {
      await tx.notify(c.Reminder, {
        subject: ref(c.Policy, d.policy),
        payload: { number: d.number, holder: d.holder, expiresOn: d.expiresOn as never },
        to: [recipient.address({ sms: d.phone })],
        channels: ["sms"],
      })
      const target = await tx.find(c.Target, { where: { campaign: wf.instance.id, number: d.number }, limit: 1 })
      if (target[0] !== undefined) await tx.update(c.Target, target[0].ref, { outcome: "sent" })
      return null
    })
  }

  // 3. Collect replies until everybody answered or the window closes.
  const answered = new Set<string>()
  const opened = Date.parse(await wf.now("window"))
  while (answered.size < due.length) {
    const left = input.replyWindowMs - (Date.parse(await wf.now("tick")) - opened)
    if (left <= 0) break
    const got = await wf.waitFor("reply", c.Campaign.signals.reply, { timeout: `${Math.ceil(left)}ms` })
    if (got.outcome === "timeout") break
    const { number, text } = got.payload
    if (answered.has(number) || !due.some((d) => d.number === number)) continue
    answered.add(number)
    await wf.transaction(`reply:${number}`, async (tx) => {
      const target = await tx.find(c.Target, { where: { campaign: wf.instance.id, number }, limit: 1 })
      if (target[0] !== undefined) await tx.update(c.Target, target[0].ref, { outcome: "replied", reply: text })
      return null
    })
  }

  // 4. Whoever did not answer is closed as such.
  const outcomes = await wf.transaction("close", async (tx) => {
    const rows = await tx.find(c.Target, { where: { campaign: wf.instance.id }, orderBy: [{ field: "number" }] })
    const result: { number: string; outcome: string }[] = []
    for (const row of rows) {
      if (row.outcome === "sent") await tx.update(c.Target, row.ref, { outcome: "noReply" })
      result.push({ number: row.number, outcome: row.outcome === "sent" ? "noReply" : row.outcome })
    }
    return result
  })
  return { outcomes }
})

export const reminderTemplates = templates(c.Reminder, {
  sms: ({ payload }) => `${payload.holder}, policy ${payload.number} expires on ${payload.expiresOn}. Reply YES to renew.`,
})

export const targets = implement(c.Targets, async (q, input) => {
  const rows = await q.find(c.Target, { where: { campaign: input.campaign }, orderBy: [{ field: "number" }] })
  return rows.map((r) => ({ number: r.number, outcome: r.outcome, reply: r.reply }))
})

// The SMS gateway posts inbound messages here; the endpoint turns each into a
// signal for the campaign that texted that number.
const Inbound = t.object({ from: t.string(), text: t.string() })
export const smsInbound = endpoint("e2e.renewals.smsInbound", { method: "POST", path: "/sms/inbound", public: true }, async (ctx) => {
  const message = decode(Inbound, ctx.request.body)
  const sent = await ctx.query.find(c.Target, { where: { phone: message.from, outcome: "sent" }, orderBy: [{ field: "createdAt", desc: true }], limit: 1 })
  const target = sent[0]
  if (target === undefined) return { status: 202, body: { accepted: false } }
  await ctx.signal({ workflow: c.Campaign.id, instance: target.campaign }, "reply", { number: target.number, text: message.text })
  return { status: 202, body: { accepted: true } }
})

// Who is acting: the signed-in user for a browser caller, the trigger owner for a bare webhook.
export const who = endpoint("e2e.renewals.who", { method: "GET", path: "/who", public: true }, async (ctx) => {
  return { status: 200, body: { user: ctx.user.userId, kind: ctx.user.kind } }
})

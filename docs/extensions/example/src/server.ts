// Server graph: handlers bound to contracts, plus guards, subscriptions, schedules, endpoints and templates.
import { isoDateTime, ref, t, validation, type ISODateTime } from "@iota-uz/sdk/runtime"
import {
  endpoint, guard, implement, recipient, schedule, subscribe, templates, writeRule,
} from "@iota-uz/sdk/runtime/server"
import * as core from "@iota-kit/core"
import * as crm from "@iota-kit/crm"
import * as c from "./contracts"

// ---- operation: reads and writes in the caller's transaction
export const adjustPoints = implement(c.AdjustPoints, async (tx, input) => {
  const current = (await tx.get(c.ClientLoyalty, input.client))?.points ?? 0
  const next = current + input.delta
  if (next < 0) throw c.NotEnoughPoints.of({ have: current, need: -input.delta })
  const row = await tx.put(c.ClientLoyalty, input.client, { points: next })
  await tx.emit(c.PointsChanged, { subject: input.client, payload: { points: next, delta: input.delta } })
  return { points: row.points }
})

// Additive checks run in the same transaction whoever implements the operation.
export const deltaLimit = guard("acme.loyalty.deltaLimit", c.AdjustPoints, async (_ctx, input) => {
  if (Math.abs(input.delta) > 10_000) throw validation(c.DeltaTooLarge.at(["delta"], { max: 10_000 }))
})

// ---- atomic command
export const createReward = implement(c.CreateReward, async (tx, input) => {
  if (input.cost <= 0) throw validation(c.CostMustBePositive.at(["cost"], { min: 1 }))
  const row = await tx.insert(c.Reward, { name: input.name.trim(), cost: input.cost })
  return { reward: row.ref }
})

// A write rule guards the entity itself, whatever code path writes it.
export const rewardRule = writeRule("acme.loyalty.rewardRule", c.Reward, async (_ctx, change) => {
  if (change.proposed !== null && change.proposed.cost > 1_000_000) throw validation(c.CostMustBePositive.at(["cost"], { min: 1 }))
})

// ---- ordinary command: outside calls happen between transactions, never inside one
export const redeem = implement(c.Redeem, async (ctx, input) => {
  const reward = await ctx.query.get(c.Reward, input.reward)
  if (reward === null || !reward.active) throw c.RewardUnavailable.of({ reward: input.reward.key })
  const stock = await fetch(`https://api.partner.example/stock/${encodeURIComponent(reward.name)}`, {
    headers: { authorization: `Bearer ${await ctx.secrets.get("PARTNER_TOKEN")}` },
  })
  if (!stock.ok) throw c.RewardUnavailable.of({ reward: input.reward.key })

  const redemption = await ctx.transaction(async (tx) => {
    await tx.call(c.AdjustPoints, { client: input.client, delta: -reward.cost })
    const row = await tx.insert(c.Redemption, {
      client: input.client, reward: input.reward, cost: reward.cost, requestedBy: ref(core.User, ctx.user.userId),
    })
    // Outside subscriptions and workflows a start needs a key: the same key never starts a second instance.
    await tx.start(c.Approval, { redemption: row.ref }, { key: `approval:${row.key}` })
    return row
  })
  return { redemption: redemption.ref }
})

export const catalog = implement(c.Catalog, async (q, input) => {
  const rewards = await q.find(c.Reward, {
    where: input.onlyActive === true ? { active: true } : {},
    orderBy: [{ field: "cost" }, { field: "name" }],
  })
  return rewards.map((r) => ({ reward: r.ref, name: r.name, cost: r.cost, active: r.active }))
})

// ---- events: each subscription runs in its own transaction after the publishing one commits
export const onPointsChanged = subscribe("acme.loyalty.onPointsChanged", c.PointsChanged, async (tx, event) => {
  const tier = event.payload.points >= 1000 ? "gold" : event.payload.points >= 300 ? "silver" : "standard"
  await tx.put(c.ClientLoyalty, event.subject, { tier })
})

// ---- workflows: durable, replayed from a journal; every primitive takes a stable name
export const approval = implement(c.Approval, async (wf, input) => {
  const verdict = await wf.task("approve", {
    subject: input.redemption,
    assignee: wf.user,
    decision: t.object({ approved: t.bool() }),
    timeout: "3d",
  })
  const approved = verdict.outcome === "decided" && verdict.decision.approved

  // An external effect: may run again after a crash, so it gets a provider idempotency key.
  const voucher = approved
    ? await wf.step("voucher", async ({ idempotencyKey }) => {
        const res = await fetch("https://api.partner.example/vouchers", {
          method: "POST",
          headers: { "content-type": "application/json", "idempotency-key": idempotencyKey },
          body: JSON.stringify({ redemption: input.redemption.key }),
        })
        const body = (await res.json()) as { code: string }
        return { code: body.code }
      })
    : null

  // Data effects commit together with their journal entry. The result must be JSON.
  await wf.transaction("settle", async (tx) => {
    const row = await tx.get(c.Redemption, input.redemption)
    if (row === null) return { settled: false }
    const reward = await tx.get(c.Reward, row.reward)
    await tx.update(c.Redemption, row.ref, { status: approved ? "approved" : "rejected" })
    if (!approved) await tx.call(c.AdjustPoints, { client: row.client, delta: row.cost })
    await tx.notify(c.RedemptionResult, {
      subject: row.ref,
      payload: { reward: reward?.name ?? "", approved },
      to: [recipient.record(row.client)],
      channels: ["sms", "email"],
    })
    return { settled: true }
  })
  return { approved, voucher: voucher?.code ?? null }
})

export const reminder = implement(c.Reminder, async (wf, input) => {
  await wf.sleep("wait", "30d")
  const answer = await wf.waitFor("optOut", c.Reminder.signals.optOut, { timeout: "1d" })
  if (answer.outcome === "received") return { reminded: false }
  await wf.transaction("remind", async (tx) => {
    const points = (await tx.get(c.ClientLoyalty, input.client))?.points ?? 0
    await tx.notify(c.PointsReminder, {
      subject: input.client,
      payload: { points },
      to: [recipient.record(input.client)],
      channels: ["sms"],
    })
    return { points }
  })
  return { reminded: true }
})

// ---- replacing a native command
// crm.SaveClient is the published intent behind the client forms; its Go default calls the operation crm.UpsertClient
// in one transaction. This implementation is only a candidate: it runs when an administrator binds it to the command
// (and the Go default returns when the binding is removed). It keeps the invariants by writing through the same
// operation, and every guard on that operation (ours included) still applies.
export const saveClient = implement(crm.SaveClient, async (ctx, input) =>
  ctx.transaction(async (tx) => {
    const saved = await tx.call(crm.UpsertClient, input)
    if (saved.created) await tx.call(c.AdjustPoints, { client: saved.client, delta: 50 })
    return saved
  }),
)

// ---- notifications: contact data of a record, and per-channel templates
export const clientConsent = implement(c.ClientConsent, async (q, request) => {
  const loyalty = await q.get(c.ClientLoyalty, request.recipient.ref)
  if (request.channel === "sms" && loyalty?.smsOptOut === true) return { allow: false, reason: "sms_opt_out" }
  return { allow: true }
})

export const clientContacts = implement(c.ClientContacts, async (q, record) => {
  const client = await q.get(crm.Client, record)
  if (client === null) return {}
  return {
    ...(client.phone === null ? {} : { sms: [client.phone] }),
    ...(client.email === null ? {} : { email: [client.email] }),
  }
})

export const resultTemplates = templates(c.RedemptionResult, {
  sms: ({ payload }) => (payload.approved ? `Your reward "${payload.reward}" is approved.` : `Your reward "${payload.reward}" was declined.`),
  email: ({ payload, locale }) => ({
    subject: payload.approved ? "Reward approved" : "Reward declined",
    html: `<p lang="${locale}">${payload.reward}</p>`,
    text: payload.reward,
  }),
})

export const reminderTemplates = templates(c.PointsReminder, {
  sms: ({ payload }) => `You have ${payload.points} loyalty points.`,
})

// ---- triggers: a schedule runs as its owner-user, an endpoint as the caller (or the owner when public)
export const closeStale = schedule("acme.loyalty.closeStale", { cron: "0 3 * * *", timezone: "Asia/Tashkent" }, async (ctx) => {
  const cutoff: ISODateTime = isoDateTime(new Date(Date.parse(ctx.slot) - 7 * 86_400_000).toISOString())
  await ctx.transaction(async (tx) => {
    const stale = await tx.find(c.Redemption, { where: { status: "pending", createdAt: { lt: cutoff } } })
    for (const row of stale) {
      await tx.update(c.Redemption, row.ref, { status: "rejected" })
      await tx.call(c.AdjustPoints, { client: row.client, delta: row.cost })
    }
  })
})

export const publicCatalog = endpoint("acme.loyalty.publicCatalog", { method: "GET", path: "/catalog", public: true }, async (ctx) => {
  const rewards = await ctx.query.find(c.Reward, { where: { active: true }, orderBy: [{ field: "name" }] })
  return { status: 200, body: rewards.map((r) => ({ name: r.name, cost: r.cost })) }
})

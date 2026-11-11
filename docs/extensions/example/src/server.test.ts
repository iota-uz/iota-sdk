// Tests run in an in-memory host: no database, no Node pool, no network. `iota-ext test` runs them.
import {
  assert, createTestHost, fixture, overlayFixture, test, type TestHostOptions,
} from "@iota-uz/sdk/runtime/testing"
import { ref } from "@iota-uz/sdk/runtime"
import { implement, recipient } from "@iota-uz/sdk/runtime/server"
import * as core from "@iota-kit/core"
import * as crm from "@iota-kit/crm"
import * as c from "./contracts"
import * as s from "./server"

const ali = fixture(crm.Client, { firstName: "Ali", phone: "+998901112233", email: "ali@example.uz" }, { key: "7" })
// Native fixtures need an explicit key. createdAt and updatedAt default to EPOCH; the fields the entity requires are yours.
const agent = fixture(core.User, { firstName: "Agent", lastName: "A", email: "agent@example.uz", uiLanguage: "en" }, { key: "42" })

function host(options: TestHostOptions = {}) {
  return createTestHost({
    implementations: [s.adjustPoints, s.createReward, s.redeem, s.catalog, s.approval, s.reminder, s.clientContacts, s.clientConsent, s.resultTemplates, s.reminderTemplates],
    user: { permissions: [c.Manage.id] },
    secrets: { PARTNER_TOKEN: "token-for-tests" },
    ...options,
    fixtures: [ali, agent, ...(options.fixtures ?? [])],
  })
}

test("adjusting points writes the overlay and emits an event", async () => {
  const h = host()
  const out = await h.transaction((tx) => s.adjustPoints.handler(tx, { client: ali.ref, delta: 40 }))
  assert.equal(out.points, 40)
  assert.partial(h.overlayRow(c.ClientLoyalty, ali.ref), { points: 40, tier: "standard" })
  assert.deepEqual(h.emitted.map((e) => e.event), [c.PointsChanged.id])
})

test("a declared domain error rolls the transaction back", async () => {
  const h = host({ fixtures: [overlayFixture(c.ClientLoyalty, ali.ref, { points: 10 })] })
  await assert.rejects(h.transaction((tx) => s.adjustPoints.handler(tx, { client: ali.ref, delta: -50 })), c.NotEnoughPoints)
  assert.equal(h.emitted.length, 0)
  assert.equal(h.overlayRow(c.ClientLoyalty, ali.ref)?.points, 10)
})

test("a guard rejects with a validation issue", async () => {
  const h = host({ guards: [s.deltaLimit] })
  await assert.rejects(h.transaction((tx) => tx.call(c.AdjustPoints, { client: ali.ref, delta: 20_000 })), { kind: "validation", issue: c.DeltaTooLarge })
})

test("an atomic command validates and stores", async () => {
  const h = host()
  await assert.rejects(h.transaction((tx) => s.createReward.handler(tx, { name: "Free", cost: 0 })), { issue: c.CostMustBePositive })
  const created = await h.transaction((tx) => s.createReward.handler(tx, { name: "  Mug ", cost: 50 }))
  assert.deepEqual(h.table(c.Reward).map((r) => r.name), ["Mug"])
  assert.equal(created.reward.entity, c.Reward.id)
})

test("redeeming checks the partner stock over fetch, then settles in one transaction", async () => {
  const mug = fixture(c.Reward, { name: "Mug", cost: 30 })
  const h = host({ fixtures: [mug, overlayFixture(c.ClientLoyalty, ali.ref, { points: 100 })] })
  // fetch() is answered by the fake http routes; an unrouted request fails the test.
  h.fakes.http.route("GET", "https://api.partner.example/stock/Mug", { status: 200, body: { available: 3 } })
  const out = await s.redeem.handler(h.command(), { client: ali.ref, reward: mug.ref })
  assert.equal(h.overlayRow(c.ClientLoyalty, ali.ref)?.points, 70)
  assert.equal(h.table(c.Redemption).length, 1)
  assert.deepEqual(h.started.map((w) => w.workflow), [c.Approval.id])
  assert.equal(h.fakes.http.calls[0]?.headers.authorization, "Bearer token-for-tests")
  assert.equal(out.redemption.entity, c.Redemption.id)
})

test("a stock failure is a declared error and nothing is written", async () => {
  const mug = fixture(c.Reward, { name: "Mug", cost: 30 })
  const h = host({ fixtures: [mug, overlayFixture(c.ClientLoyalty, ali.ref, { points: 100 })] })
  h.fakes.http.route("GET", "https://api.partner.example/stock/Mug", { status: 404 })
  await assert.rejects(s.redeem.handler(h.command(), { client: ali.ref, reward: mug.ref }), c.RewardUnavailable)
  assert.equal(h.table(c.Redemption).length, 0)
})

test("the approval workflow waits for the human task, then settles and notifies", async () => {
  const mug = fixture(c.Reward, { name: "Mug", cost: 30 })
  const pending = fixture(c.Redemption, { client: ali.ref, reward: mug.ref, cost: 30, requestedBy: ref(core.User, "1") }, { key: "00000000-0000-4000-8000-000000000001" })
  const h = host({ fixtures: [mug, pending] })
  h.fakes.http.route("POST", "https://api.partner.example/vouchers", { status: 200, body: { code: "V-123" } })

  const run = h.workflow(s.approval, { redemption: pending.ref })
  const first = await run.run()
  assert.equal(first.status, "suspended")
  assert.deepEqual(run.tasks.map((t) => [t.name, t.status]), [["approve", "open"]])

  run.decide("approve", { approved: true })
  const done = await run.run()
  assert.deepEqual(done.status === "completed" ? done.value : null, { approved: true, voucher: "V-123" })

  // Rendered with the templates, delivered per recipient and channel to the fakes.
  assert.deepEqual(h.fakes.sms.delivered.map((m) => [m.to, m.text]), [["+998901112233", 'Your reward "Mug" is approved.']])
  assert.deepEqual(h.fakes.email.delivered.map((m) => m.to), ["ali@example.uz"])
  assert.deepEqual(h.deliveries.map((d) => [d.channel, d.status]), [["sms", "delivered"], ["email", "delivered"]])
})

test("a delivery policy skips an opted-out SMS; the e-mail to the same client still goes out", async () => {
  const mug = fixture(c.Reward, { name: "Mug", cost: 30 })
  const redemption = fixture(c.Redemption, { client: ali.ref, reward: mug.ref, cost: 30, requestedBy: ref(core.User, "1") })
  const h = host({ fixtures: [mug, redemption, overlayFixture(c.ClientLoyalty, ali.ref, { smsOptOut: true })] })
  await h.transaction((tx) =>
    tx.notify(c.RedemptionResult, { subject: redemption.ref, payload: { reward: "Mug", approved: true }, to: [recipient.record(ali.ref)], channels: ["sms", "email"] }),
  )
  assert.deepEqual(h.fakes.sms.delivered, [])
  assert.deepEqual(h.fakes.email.delivered.map((m) => m.to), ["ali@example.uz"])
  assert.deepEqual(h.deliveries.map((d) => [d.channel, d.status, d.reason]), [["sms", "skipped", "policy_denied:sms_opt_out"], ["email", "delivered", undefined]])
})

test("one notify to a client and an ERP user: every channel is tried for every recipient", async () => {
  const mug = fixture(c.Reward, { name: "Mug", cost: 30 })
  const redemption = fixture(c.Redemption, { client: ali.ref, reward: mug.ref, cost: 30, requestedBy: ref(core.User, "42") })
  const h = host({ fixtures: [mug, redemption] })
  await h.transaction((tx) =>
    tx.notify(c.RedemptionResult, {
      subject: redemption.ref,
      payload: { reward: "Mug", approved: true },
      to: [recipient.record(ali.ref), recipient.user(agent.ref)],
      channels: ["sms", "inApp"],
    }),
  )
  assert.deepEqual(h.deliveries.map((d) => [d.recipient.kind, d.channel, d.status]), [
    ["record", "sms", "delivered"],
    ["record", "inApp", "skipped"], // in-app messages address ERP users, not client records
    ["user", "sms", "skipped"], // the core.User fixture has no phone
    ["user", "inApp", "delivered"],
  ])
})

test("an event reaches its subscription and promotes the tier", async () => {
  const h = host()
  await h.deliver(s.onPointsChanged, { subject: ali.ref, payload: { points: 1200, delta: 200 } })
  assert.equal(h.overlayRow(c.ClientLoyalty, ali.ref)?.tier, "gold")
})

test("a reminder sleeps, waits for the opt-out signal and sends one SMS", async () => {
  const h = host({ fixtures: [overlayFixture(c.ClientLoyalty, ali.ref, { points: 55 })] })
  const run = h.workflow(s.reminder, { client: ali.ref })
  assert.deepEqual(await run.run().then((r) => r.status === "suspended" && r.on.kind), "sleep")
  h.clock.advance("30d")
  assert.deepEqual(await run.run().then((r) => r.status === "suspended" && r.on.kind), "signal")
  h.clock.advance("1d")
  const done = await run.run()
  assert.deepEqual(done.status === "completed" ? done.value : null, { reminded: true })
  assert.deepEqual(h.fakes.sms.delivered.map((m) => m.text), ["You have 55 loyalty points."])
})

test("a replacement of a native command writes through the native operation", async () => {
  // Native contracts have no handler in a test host: bind a double of the Go default.
  const upsert = implement(crm.UpsertClient, async (_tx, input) => ({ client: input.client ?? ali.ref, created: input.client === undefined }))
  const h = host({ implementations: [upsert, s.saveClient, s.adjustPoints] })
  const out = await s.saveClient.handler(h.command(), { firstName: "Vali" })
  assert.equal(out.created, true)
  assert.equal(h.overlayRow(c.ClientLoyalty, ali.ref)?.points, 50)
  const again = await s.saveClient.handler(h.command(), { client: ali.ref, phone: "+998900000000" })
  assert.equal(again.created, false)
  assert.equal(h.overlayRow(c.ClientLoyalty, ali.ref)?.points, 50)
})

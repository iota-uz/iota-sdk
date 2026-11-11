import assert from "node:assert/strict"
import { createTestHost, fixture, test } from "@iota-uz/sdk/runtime/testing"
import { decimal, isoDate, ref } from "@iota-uz/sdk/runtime"
import * as core from "@iota-kit/core"
import * as c from "./contracts"
import * as s from "./server"

function setup() {
  const customer = fixture(c.Customer, { name: "Ali", owner: ref(core.User, "42") })
  const policy = fixture(c.Policy, {
    number: "P-1",
    holder: customer.ref,
    premium: decimal("120.00"),
    expiresOn: isoDate("2027-01-01"),
    status: "active",
  })
  return { customer, policy, host: createTestHost({
      fixtures: [customer, policy],
      implementations: [s.extendExpiry],
      user: { permissions: [c.ReadPolicies.id] },
    }), }
}

test("extendExpiry moves the date and emits an event", async () => {
  const { policy, host } = setup()
  const out = await host.transaction((tx) => s.extendExpiry.handler(tx, { policy: policy.ref, months: 12 }))
  assert.equal(out.expiresOn, "2028-01-01")
  assert.equal(host.emitted.length, 1)
  assert.equal(host.emitted[0]?.event, c.PolicyRenewed.id)
})

test("expiring lists active policies before a date", async () => {
  const { host } = setup()
  const rows = await s.expiring.handler(host.query(), { before: isoDate("2028-01-01") })
  assert.deepEqual(rows.map((r) => r.number), ["P-1"])
})

test("expiring ignores policies whose holder has no owner (ref path isNull)", async () => {
  const customer = fixture(c.Customer, { name: "Vali" })
  const policy = fixture(c.Policy, { number: "P-3", holder: customer.ref, premium: decimal("1.00"), expiresOn: isoDate("2027-01-01"), status: "active" })
  const host = createTestHost({ fixtures: [customer, policy], entities: [c.Customer], user: { permissions: [c.ReadPolicies.id] } })
  assert.deepEqual(await s.expiring.handler(host.query(), { before: isoDate("2028-01-01") }), [])
})

test("a user without read access sees no policies", async () => {
  const customer = fixture(c.Customer, { name: "Ali" })
  const policy = fixture(c.Policy, { number: "P-2", holder: customer.ref, premium: decimal("1.00"), expiresOn: isoDate("2027-01-01"), status: "active" })
  const host = createTestHost({ fixtures: [customer, policy] })
  assert.equal(await host.query().get(c.Policy, policy.ref), null)
})

test("tx.call rejects an ordinary command", async () => {
  const host = createTestHost()
  await assert.rejects(
    // @ts-expect-error ordinary commands are not callable inside a transaction
    host.transaction((tx) => tx.call(c.RenewPolicy, { policy: { entity: c.Policy.id, key: "k" } })),
    /accepts only operations and atomic commands/,
  )
})

test("unknown outbound http is blocked", async () => {
  await assert.rejects(fetch("https://example.org/quote"), /blocked in synthetic tests/)
})

test("fake http routes answer fetch", async () => {
  const host = createTestHost()
  host.fakes.http.route("GET", "https://quotes.example.uz/P-1", { status: 200, body: { premium: "99.00" } })
  const res = await fetch("https://quotes.example.uz/P-1")
  assert.deepEqual(await res.json(), { premium: "99.00" })
})

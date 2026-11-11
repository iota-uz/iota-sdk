// Pure contracts of acme.loyalty: every resource the package declares. No handlers, no Node, no Solid.
import {
  $, column, command, contactResolver, defineSlot, domainError, entity, event, field, issue, notice, operation, overlay, permission, query,
  route, t, workflow,
} from "@iota-uz/sdk/runtime"
import * as core from "@iota-kit/core"
import * as crm from "@iota-kit/crm"

// ---- permissions
export const Manage = permission("acme.loyalty.manage", { label: { en: "Manage loyalty", ru: "Управление лояльностью" } })

// ---- data
// An overlay adds our fields to the platform's client; absence of a row reads the defaults.
export const ClientLoyalty = overlay(
  "acme.loyalty.clientLoyalty",
  crm.Client,
  {
    tier: field.enum(["standard", "silver", "gold"]).default("standard"),
    points: field.int().default(0),
    note: field.text().optional(),
  },
  { access: { fields: { note: [Manage] } } },
)

// The overlay field shows up as a column of the native clients list (sort, filter, export).
export const TierColumn = column(crm.ListsClients, {
  id: "acme.loyalty.tier",
  field: ClientLoyalty.fields.tier,
  label: { en: "Loyalty tier", ru: "Уровень" },
  filterable: true,
  sortable: true,
  exportable: true,
})

export const Reward = entity("acme.loyalty.reward", {
  name: field.text({ maxLength: 80 }).unique(),
  cost: field.int(),
  active: field.bool().default(true),
})

export const Redemption = entity(
  "acme.loyalty.redemption",
  {
    client: field.ref(crm.Client).index(),
    reward: field.ref(Reward),
    cost: field.int(),
    status: field.enum(["pending", "approved", "rejected"]).default("pending"),
    requestedBy: field.ref(core.User),
  },
  // Managers see every redemption, everybody else the ones they requested.
  { access: { read: [{ when: Manage, rows: "all" }, { rows: { requestedBy: $.user } }] } },
)

// ---- errors
export const NotEnoughPoints = domainError("acme.loyalty.notEnoughPoints", t.object({ have: t.int(), need: t.int() }))
export const RewardUnavailable = domainError("acme.loyalty.rewardUnavailable", t.object({ reward: t.string() }))
export const CostMustBePositive = issue("acme.loyalty.costMustBePositive", t.object({ min: t.int() }))
export const DeltaTooLarge = issue("acme.loyalty.deltaTooLarge", t.object({ max: t.int() }))

// ---- callables
// An operation is an atomic domain primitive other packages and transactions can call.
export const AdjustPoints = operation("acme.loyalty.adjustPoints", {
  input: t.object({ client: t.ref(crm.Client), delta: t.int() }),
  output: t.object({ points: t.int() }),
  errors: [NotEnoughPoints],
})

// An atomic command runs entirely in one transaction.
export const CreateReward = command("acme.loyalty.createReward", {
  atomic: true,
  input: t.object({ name: t.string(), cost: t.int() }),
  output: t.object({ reward: t.ref(Reward) }),
  permission: Manage,
})

// An ordinary command may call the outside world between its transactions.
export const Redeem = command("acme.loyalty.redeem", {
  input: t.object({ client: t.ref(crm.Client), reward: t.ref(Reward) }),
  output: t.object({ redemption: t.ref(Redemption) }),
  errors: [NotEnoughPoints, RewardUnavailable],
})

export const Catalog = query("acme.loyalty.catalog", {
  input: t.object({ onlyActive: t.bool().optional() }),
  output: t.array(t.object({ reward: t.ref(Reward), name: t.string(), cost: t.int(), active: t.bool() })),
})

// ---- events, notices, workflows
export const PointsChanged = event("acme.loyalty.pointsChanged.v1", {
  subject: crm.Client,
  payload: t.object({ points: t.int(), delta: t.int() }),
})

export const RedemptionResult = notice("acme.loyalty.redemptionResult.v1", {
  subject: Redemption,
  payload: t.object({ reward: t.string(), approved: t.bool() }),
})

export const PointsReminder = notice("acme.loyalty.pointsReminder.v1", {
  subject: crm.Client,
  payload: t.object({ points: t.int() }),
})

// Lets notifications reach a client record: where to send an SMS or an e-mail.
export const ClientContacts = contactResolver("acme.loyalty.clientContacts", crm.Client)

export const Approval = workflow("acme.loyalty.approval", {
  input: t.object({ redemption: t.ref(Redemption) }),
  output: t.object({ approved: t.bool(), voucher: t.string().nullable() }),
  signals: { withdraw: t.object({ reason: t.string() }) },
})

export const Reminder = workflow("acme.loyalty.reminder", {
  input: t.object({ client: t.ref(crm.Client) }),
  output: t.object({ reminded: t.bool() }),
  signals: { optOut: t.object({}) },
})

// ---- routes
export const RedemptionRoute = route("acme.loyalty.redemptionPage", {
  path: "/loyalty/redemptions/:id",
  params: t.object({ id: t.uuid() }),
})

// ---- slots
// A place in our rewards page where any package may mount a component. The host page names it, types its props
// and passes them; contributions are additive and need no binding.
export const RewardActions = defineSlot("acme.loyalty.rewardActions", { props: t.object({ reward: t.string() }) })

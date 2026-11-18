// Adds loyalty data to CRM clients: an overlay of the platform's client entity,
// three columns of the native clients list, and a command to maintain the data.
import { column, command, field, overlay, permission, t } from "@iota-uz/sdk/runtime"
import * as crm from "@iota-kit/crm"

export const Manage = permission("e2e.clientextra.manage", { label: { en: "Manage client loyalty" } })
export const ViewInternal = permission("e2e.clientextra.viewInternal", { label: { en: "See internal notes" } })

export const ClientExtra = overlay(
  "e2e.clientextra.clientExtra",
  crm.Client,
  {
    tier: field.enum(["standard", "silver", "gold"]).default("standard"),
    score: field.int().optional(),
    internalNote: field.text().optional(),
  },
  { access: { fields: { internalNote: [ViewInternal] } } },
)

export const TierColumn = column(crm.ListsClients, {
  id: "e2e.clientextra.tier",
  field: ClientExtra.fields.tier,
  label: { en: "Loyalty tier", ru: "Уровень", uz: "Daraja" },
  filterable: true,
  sortable: true,
  exportable: true,
})
export const ScoreColumn = column(crm.ListsClients, {
  id: "e2e.clientextra.score",
  field: ClientExtra.fields.score,
  label: { en: "Score", ru: "Баллы" },
  filterable: true,
  sortable: true,
  exportable: true,
})
export const InternalColumn = column(crm.ListsClients, {
  id: "e2e.clientextra.internal",
  field: ClientExtra.fields.internalNote,
  label: { en: "Internal note", ru: "Заметка" },
  filterable: true,
  exportable: true,
})

export const SetProfile = command("e2e.clientextra.setProfile", {
  input: t.object({
    client: t.ref(crm.Client),
    tier: t.enum(["standard", "silver", "gold"]).optional(),
    score: t.int().optional(),
    internalNote: t.string().optional(),
  }),
  output: t.object({ tier: t.string(), score: t.int().nullable() }),
  permission: Manage,
})

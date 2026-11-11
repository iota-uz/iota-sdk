// Client graph of "acme.insurance": pages, navigation, slot, cell, shell (Solid).
import { lazy } from "solid-js"
import { cell, navigation, page, shellRenderer, slot } from "@iota-uz/sdk/runtime/client"
import * as crm from "@iota-kit/crm"
import * as c from "./contracts"

export const policies = page("acme.insurance.policies", {
  path: "/insurance/policies",
  nav: { label: { en: "Policies", ru: "Полисы" }, group: "acme.insurance", icon: "shield" },
  permission: c.ReadPolicies,
  component: lazy(() => import("./PoliciesScreen")),
})
export const policy = page(c.PolicyRoute, { component: lazy(() => import("./PolicyScreen")) })
export const nav = navigation({
  groups: [{ id: "acme.insurance", label: { en: "Insurance" } }],
  patches: [{ ref: "core.users", hide: true }],
  workspaces: [{ id: "acme.insurance.work", label: { en: "Insurance" } }],
  palette: [{ id: "acme.insurance.openPolicies", label: { en: "Open policies" }, target: "acme.insurance.policies" }],
})
export const tierCell = cell(c.ClientsTierColumn, (p) => <span class="badge">{p.value ?? "none"}</span>)
export const clientBadge = slot(crm.ClientToolbar, (p) => <b>{String(p["client"])}</b>)
export const shell = shellRenderer((p) => <div class="acme-shell">{p.outlet}</div>)

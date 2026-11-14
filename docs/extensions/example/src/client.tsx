// Browser graph: pages, navigation, slot contributions and cell renderers (Solid). It imports contracts, never the server entry.
import { lazy } from "solid-js"
import { cell, navigation, page, slot } from "@iota-uz/sdk/runtime/client"
import * as crm from "@iota-kit/crm"
import * as c from "./contracts"

// A page of its own: ID, path and navigation entry.
export const rewards = page("acme.loyalty.rewards", {
  path: "/loyalty/rewards",
  nav: { label: { en: "Rewards", ru: "Награды" }, group: "acme.loyalty.menu", icon: "gift" },
  permission: c.Manage,
  component: lazy(() => import("./RewardsScreen")),
})

// A page bound to a route: typed params, ID "<route id>.page".
export const redemption = page(c.RedemptionRoute, {
  component: lazy(() => import("./RedemptionScreen")),
})

// `nav.group` above refers to a group declared here, by its ID.
export const nav = navigation({
  groups: [{ id: "acme.loyalty.menu", label: { en: "Loyalty", ru: "Лояльность" } }],
})

// How the tier column renders in the native clients list.
export const tierCell = cell(c.TierColumn, (props) => <span class={`tier tier-${props.value}`}>{props.value}</span>)

// A component in the CRM's client sidebar, a slot the platform declares. Additive: no binding, active while this
// version is, shown only to viewers holding `Manage` (the server decides), after contributions with a smaller order.
export const loyaltyCard = slot(crm.ClientSidebar, (props) => <p class="loyalty-card">Loyalty for client {props.clientId}</p>, {
  order: 10,
  permission: c.Manage,
})

// A contribution to a slot of our own package (declared in contracts.ts), typed by it.
export const exportAction = slot(c.RewardActions, (props) => <button type="button">Export {props.reward}</button>)

// A page for the platform's client view route: a replacement candidate, served only once an administrator binds it
// to the `screen` slot of `crm.routes.clientView`.
export const clientView = page(crm.ClientView, { component: lazy(() => import("./ClientScreen")) })

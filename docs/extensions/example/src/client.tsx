// Browser graph: pages, navigation and cell renderers (Solid). It imports contracts, never the server entry.
import { lazy } from "solid-js"
import { cell, navigation, page } from "@iota-uz/sdk/runtime/client"
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

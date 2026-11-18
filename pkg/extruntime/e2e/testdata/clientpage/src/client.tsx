// A screen replacement: the platform's client view route, implemented by a package page. It stays a candidate
// until an administrator binds it to the screen slot.
import { lazy } from "solid-js"
import { page } from "@iota-uz/sdk/runtime/client"
import * as crm from "@iota-kit/crm"

export const clientView = page(crm.ClientView, { component: lazy(() => import("./ClientView")) })

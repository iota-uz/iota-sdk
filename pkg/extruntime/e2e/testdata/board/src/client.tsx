import { lazy } from "solid-js"
import { page, slot } from "@iota-uz/sdk/runtime/client"
import * as c from "./contracts"

export const board = page("e2e.board.board", {
  path: "/board",
  nav: { label: { en: "Board" }, icon: "layout" },
  component: lazy(() => import("./BoardScreen")),
})

// The board's own widgets: the first for everybody, the second only for managers.
export const clock = slot(c.Widgets, (p) => <section data-testid="board-clock">Clock on {p.title}</section>, { order: 10 })
export const admin = slot(c.Widgets, (p) => <section data-testid="board-admin">Admin widget on {p.title}</section>, { order: 30, permission: c.Manage })

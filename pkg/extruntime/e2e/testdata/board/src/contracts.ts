// A board page that offers a slot of its own: other packages mount widgets in it.
import { defineSlot, permission, t } from "@iota-uz/sdk/runtime"

export const Manage = permission("e2e.board.manage", { label: { en: "Manage the board" } })

export const Widgets = defineSlot("e2e.board.widgets", { props: t.object({ title: t.string() }) })

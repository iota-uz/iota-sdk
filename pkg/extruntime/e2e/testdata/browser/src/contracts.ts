// The package of the browser end-to-end suite: a data page with a query and a command, a page with a typed
// route, a slot of its own and a shell renderer. Components report their lifecycle to the page (probe.ts) so the
// suite can tell a disposed root from a leaked one.
import { command, defineSlot, entity, field, permission, query, route, t } from "@iota-uz/sdk/runtime"

export const Use = permission("e2e.browser.use", { label: { en: "Use the browser lab" } })
export const Audit = permission("e2e.browser.audit", { label: { en: "See the audit panels" } })

export const Hit = entity("e2e.browser.hit", { label: field.text({ maxLength: 80 }) }, { access: { read: [{ rows: "all" }] } })

export const RecordHit = command("e2e.browser.recordHit", {
  input: t.object({ label: t.string() }),
  output: t.object({ total: t.int() }),
  permission: Use,
})

export const ListHits = query("e2e.browser.listHits", {
  input: t.object({}),
  output: t.array(t.object({ label: t.string() })),
  permission: Use,
})

// A page with a typed path parameter: `id` is an int, decoded in the browser.
export const ItemRoute = route("e2e.browser.itemRoute", {
  path: "/browser/items/:id",
  params: t.object({ id: t.int() }),
})

// A place on the lab page where contributions are mounted.
export const Panel = defineSlot("e2e.browser.panel", { props: t.object({ title: t.string() }) })

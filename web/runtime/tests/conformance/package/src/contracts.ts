// The probe package of the host conformance suite (../cases.json). The same sources run on the TS test host
// (tests/conformance.test.ts) and, built by the real build worker, on the Go host with the PostgreSQL data store
// (pkg/extruntime/e2e/conformance_test.go). Every step of a case is one invocation of `write` (one transaction),
// `read` (one snapshot) or `flow` (an ordinary command).
import { command, entity, field, operation, overlay, permission, query, t } from "@iota-uz/sdk/runtime"

export const ReadAll = permission("e2e.conformance.readAll", { label: { en: "Read all items" } })
export const ReadPublic = permission("e2e.conformance.readPublic", { label: { en: "Read public items" } })
export const SeeSecret = permission("e2e.conformance.seeSecret", { label: { en: "See item secrets" } })

export const Item = entity(
  "e2e.conformance.item",
  {
    name: field.text().unique(),
    rank: field.int().optional(),
    tag: field.text().optional(),
    secret: field.text().optional(),
  },
  {
    access: {
      read: [{ when: ReadAll, rows: "all" }, { when: ReadPublic, rows: { tag: "public" } }],
      fields: { secret: [SeeSecret] },
    },
  },
)

export const Child = entity("e2e.conformance.child", {
  label: field.text(),
  item: field.ref(Item).optional(),
})

export const ItemExtra = overlay("e2e.conformance.itemExtra", Item, {
  note: field.text().optional(),
  score: field.int().default(0),
})

const Step = t.object({ step: t.json() })
const Result = t.object({ result: t.json().nullable() })

/** One write step in one transaction. */
export const Write = command("e2e.conformance.write", { input: Step, output: Result, atomic: true })
/** One read step in one snapshot. */
export const Read = query("e2e.conformance.read", { input: Step, output: Result })
/** One step of an ordinary command (ctx.transaction / ctx.call rules). */
export const Flow = command("e2e.conformance.flow", { input: Step, output: Result })

/** Targets of tx.call / ctx.call steps. */
export const Touch = operation("e2e.conformance.touch", { input: t.object({ name: t.string() }), output: t.object({ ok: t.bool() }) })
export const Ordinary = command("e2e.conformance.ordinary", { input: t.object({}), output: t.object({ ok: t.bool() }) })

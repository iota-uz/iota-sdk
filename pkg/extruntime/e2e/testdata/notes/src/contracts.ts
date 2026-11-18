// Notes: a small module written the way an independent extension author would.
import { $, command, domainError, entity, field, permission, query, t } from "@iota-uz/sdk/runtime"
import * as core from "@iota-kit/core"

export const ReadAll = permission("e2e.notes.read", { label: { en: "Read every note" } })
export const Write = permission("e2e.notes.write", { label: { en: "Write notes" } })

export const Note = entity(
  "e2e.notes.note",
  {
    title: field.text({ maxLength: 80 }),
    body: field.text().optional(),
    author: field.ref(core.User),
    pinned: field.bool().default(false),
  },
  // Everyone sees their own notes; holders of ReadAll see all of them.
  { access: { read: [{ when: ReadAll, rows: "all" }, { rows: { author: $.user } }] } },
)

export const TitleTaken = domainError("e2e.notes.titleTaken", t.object({ title: t.string() }))

export const AddNote = command("e2e.notes.addNote", {
  input: t.object({ title: t.string(), body: t.string().optional() }),
  output: t.object({ note: t.ref(Note) }),
  errors: [TitleTaken],
  permission: Write,
})

export const ListNotes = query("e2e.notes.listNotes", {
  input: t.object({ pinnedOnly: t.bool().optional() }),
  output: t.array(t.object({ note: t.ref(Note), title: t.string(), author: t.string() })),
})

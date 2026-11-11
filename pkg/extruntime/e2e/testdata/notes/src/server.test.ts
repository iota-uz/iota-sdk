import assert from "node:assert/strict"
import { createTestHost, test } from "@iota-uz/sdk/runtime/testing"
import * as c from "./contracts"
import * as s from "./server"

function host() {
  return createTestHost({ implementations: [s.addNote, s.listNotes], user: { permissions: [c.Write.id] } })
}

test("a note is stored for its author and listed back", async () => {
  const h = host()
  const added = await s.addNote.handler(h.command(), { title: "  First  " })
  assert.equal(added.note.entity, c.Note.id)
  const listed = await s.listNotes.handler(h.query(), {})
  assert.deepEqual(listed.map((n) => n.title), ["First"])
})

test("a duplicate title is a declared domain error", async () => {
  const h = host()
  await s.addNote.handler(h.command(), { title: "Same" })
  await assert.rejects(s.addNote.handler(h.command(), { title: "Same" }), /e2e\.notes\.titleTaken/)
})

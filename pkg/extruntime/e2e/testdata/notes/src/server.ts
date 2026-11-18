import { ref } from "@iota-uz/sdk/runtime"
import { implement } from "@iota-uz/sdk/runtime/server"
import * as core from "@iota-kit/core"
import * as c from "./contracts"

export const addNote = implement(c.AddNote, async (ctx, input) => {
  const title = input.title.trim()
  const note = await ctx.transaction(async (tx) => {
    if ((await tx.count(c.Note, { title })) > 0) throw c.TitleTaken.of({ title })
    return tx.insert(c.Note, { title, body: input.body ?? null, author: ref(core.User, ctx.user.userId) })
  })
  return { note: note.ref }
})

export const listNotes = implement(c.ListNotes, async (q, input) => {
  const rows = await q.find(c.Note, {
    where: input.pinnedOnly === true ? { pinned: true } : undefined,
    orderBy: [{ field: "title" }],
  })
  return rows.map((r) => ({ note: r.ref, title: r.title, author: r.author.key }))
})

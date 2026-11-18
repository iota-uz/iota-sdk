import { For, Show, createSignal } from "solid-js"
import { useCommand, useQuery, type PageProps } from "@iota-uz/sdk/runtime/client"
import * as c from "./contracts"

export default function NotesScreen(_props: PageProps) {
  const notes = useQuery(c.ListNotes, {})
  const add = useCommand(c.AddNote)
  const [title, setTitle] = createSignal("")
  return (
    <section data-testid="notes-screen">
      <h1>Notes</h1>
      <input value={title()} onInput={(e) => setTitle(e.currentTarget.value)} />
      <button
        disabled={add.pending()}
        onClick={async () => {
          await add.run({ title: title() })
          notes.refetch()
        }}
      >
        Add
      </button>
      <Show when={notes.data()}>
        {(rows) => (
          <ul>
            <For each={rows()}>{(n) => <li>{n.title}</li>}</For>
          </ul>
        )}
      </Show>
    </section>
  )
}

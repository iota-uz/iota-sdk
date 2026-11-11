import { For, Show, createSignal } from "solid-js"
import { Slot, useCommand, useQuery, type PageProps } from "@iota-uz/sdk/runtime/client"
import * as c from "./contracts"
import { probe } from "./probe"

export default function Lab(_props: PageProps) {
  probe("lab")
  const hits = useQuery(c.ListHits, {})
  const record = useCommand(c.RecordHit)
  const [label, setLabel] = createSignal("")
  return (
    <section data-testid="lab">
      <h1>Browser lab</h1>
      <input data-testid="lab-label" value={label()} onInput={(e) => setLabel(e.currentTarget.value)} />
      <button
        data-testid="lab-record"
        disabled={record.pending()}
        onClick={async () => {
          await record.run({ label: label() })
          setLabel("")
          hits.refetch()
        }}
      >
        Record
      </button>
      <a data-testid="lab-item-link" href="/browser/items/7">Item 7</a>
      <Show when={hits.data()}>
        {(rows) => (
          <ul data-testid="lab-hits">
            <For each={rows()}>{(h) => <li>{h.label}</li>}</For>
          </ul>
        )}
      </Show>
      <div data-testid="lab-panel">
        <Slot slot={c.Panel} props={{ title: "Lab" }} />
      </div>
    </section>
  )
}

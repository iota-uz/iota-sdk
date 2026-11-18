import { createSignal } from "solid-js"
import { createStore } from "solid-js/store"
import type { PageProps } from "@iota-uz/sdk/runtime/client"

export default function PoliciesScreen(_props: PageProps) {
  const [count, setCount] = createSignal(0)
  const [filters] = createStore({ status: "active" })
  return (
    <section>
      <h1>Policies ({filters.status})</h1>
      <button onClick={() => setCount(count() + 1)}>Clicked {count()}</button>
    </section>
  )
}

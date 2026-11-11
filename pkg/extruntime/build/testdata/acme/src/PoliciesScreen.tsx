import { createSignal } from "solid-js"
import type { PageProps } from "@iota-uz/sdk/runtime/client"

export default function PoliciesScreen(_props: PageProps) {
  const [count, setCount] = createSignal(0)
  return (
    <section>
      <h1>Policies</h1>
      <button onClick={() => setCount(count() + 1)}>Clicked {count()}</button>
    </section>
  )
}

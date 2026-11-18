import type { PageProps } from "@iota-uz/sdk/runtime/client"
import { probe } from "./probe"

// `id` is typed by the route: a number, decoded from the path.
export default function Item(props: PageProps<{ id: number }>) {
  probe("item")
  return (
    <main data-testid="item">
      <h1 data-testid="item-id">Item {props.params.id}</h1>
      <span data-testid="item-type">{typeof props.params.id}</span>
      <a data-testid="item-back" href="/browser/lab">Back to the lab</a>
    </main>
  )
}

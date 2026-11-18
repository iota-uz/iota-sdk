import { Slot } from "@iota-uz/sdk/runtime/client"
import * as c from "./contracts"

export default function BoardScreen() {
  return (
    <section data-testid="board-screen">
      <h1>Board</h1>
      <Slot slot={c.Widgets} props={{ title: "Main board" }} />
    </section>
  )
}

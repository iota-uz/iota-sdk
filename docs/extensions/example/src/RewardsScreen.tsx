import { For, Show, createSignal } from "solid-js"
import { helpHref, isError, Slot, useCommand, useQuery, type PageProps } from "@iota-uz/sdk/runtime/client"
import { Button, Card } from "@iota-uz/sdk/solid-ui"
import * as c from "./contracts"
import styles from "./RewardsScreen.module.css"

export default function RewardsScreen(_props: PageProps) {
  const rewards = useQuery(c.Catalog, { onlyActive: true })
  const create = useCommand(c.CreateReward)
  const [name, setName] = createSignal("")
  const [failure, setFailure] = createSignal("")
  return (
    <section class={`${styles.board} p-6`}>
      <h1 class="text-xl font-semibold">Rewards</h1>
      <a href={helpHref("getting-started")}>How does loyalty work?</a>
      <Card>
        <input value={name()} onInput={(e) => setName(e.currentTarget.value)} />
        <Button
          variant="primary"
          loading={create.pending()}
          onClick={async () => {
            try {
              await create.run({ name: name(), cost: 100 })
              rewards.refetch()
            } catch (err) {
              setFailure(isError(err, c.NotEnoughPoints) ? `need ${err.body.params.need}` : String(err))
            }
          }}
        >
          Add
        </Button>
      </Card>
      <Show when={failure() !== ""}>
        <p role="alert">{failure()}</p>
      </Show>
      <Show when={rewards.data()}>
        {(rows) => (
          <ul>
            <For each={rows()}>
              {(r) => (
                <li class={styles.reward}>
                  {r.name}: {r.cost} <Slot slot={c.RewardActions} props={{ reward: r.name }} />
                </li>
              )}
            </For>
          </ul>
        )}
      </Show>
    </section>
  )
}

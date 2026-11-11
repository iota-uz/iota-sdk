import { For, Show, createSignal } from "solid-js"
import { isError, Slot, useCommand, useQuery, type PageProps } from "@iota-uz/sdk/runtime/client"
import * as c from "./contracts"

export default function RewardsScreen(_props: PageProps) {
  const rewards = useQuery(c.Catalog, { onlyActive: true })
  const create = useCommand(c.CreateReward)
  const [name, setName] = createSignal("")
  const [failure, setFailure] = createSignal("")
  return (
    <section>
      <h1>Rewards</h1>
      <input value={name()} onInput={(e) => setName(e.currentTarget.value)} />
      <button
        disabled={create.pending()}
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
      </button>
      <Show when={failure() !== ""}>
        <p role="alert">{failure()}</p>
      </Show>
      <Show when={rewards.data()}>
        {(rows) => (
          <ul>
            <For each={rows()}>
              {(r) => (
                <li>
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

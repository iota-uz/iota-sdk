// Contributions as the build slice would ship them: `slot()` / `cell()` values with Solid components.
import { Show, onCleanup } from 'solid-js'
import { query } from '../../../../../../src/contracts.ts'
import { t } from '../../../../../../src/schema.ts'
import { useQuery } from '../../../../../../src/client-host/hooks.ts'

const Expiring = query('acme.insurance.expiring', { input: t.object({ days: t.int() }), output: t.object({ count: t.int() }) })

declare global {
  interface Window { __cleanups?: number }
}

function Badge(props: { clientId: number }) {
  const expiring = useQuery(Expiring, { days: 30 })
  onCleanup(() => { window.__cleanups = (window.__cleanups ?? 0) + 1 })
  return (
    <aside data-testid="slot-badge">
      Client {props.clientId}
      <Show when={expiring.data()}>{(data) => <span data-testid="slot-expiring"> · {data().count} expiring</span>}</Show>
    </aside>
  )
}

export const badge = { component: Badge }
export const second = { component: (props: { clientId: number }) => <i data-testid="slot-second">second {props.clientId}</i> }
export const tier = { component: (props: { value: string }) => <b data-testid="tier-cell">{String(props.value).toUpperCase()}</b> }

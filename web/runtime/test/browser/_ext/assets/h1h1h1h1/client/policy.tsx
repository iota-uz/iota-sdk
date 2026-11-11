// A package page as the build slice would ship it: `page()` value with a Solid component.
import { Show } from 'solid-js'
import { query } from '../../../../../../src/contracts.ts'
import { t } from '../../../../../../src/schema.ts'
import { useQuery } from '../../../../../../src/client-host/hooks.ts'

const Expiring = query('acme.insurance.expiring', { input: t.object({ days: t.int() }), output: t.object({ count: t.int() }) })

function PolicyScreen(props: { params: { id: number } }) {
  const expiring = useQuery(Expiring, { days: 30 })
  return (
    <section data-testid="policy-screen">
      <h1>Policy {props.params.id}</h1>
      <Show when={expiring.data()} fallback={<p data-testid="loading">loading…</p>}>
        {(data) => <p data-testid="expiring">{data().count} expiring</p>}
      </Show>
    </section>
  )
}

export const policy = { component: PolicyScreen }

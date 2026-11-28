import { createSignal, onMount, type JSX } from 'solid-js'
import { backtestsPath } from './backtestsModel'
import { useApp } from './context'
import { Icon } from './Icon'

/** The editor's link to the section: «Backtests: N» in the host's actions slot. */
export function BacktestsLink(props: { id: string }): JSX.Element {
  const app = useApp()
  const [count, setCount] = createSignal<number>()
  onMount(async () => {
    try {
      setCount((await app.call('automations.backtest_count', { automation_id: props.id })).count)
    } catch {
      setCount(undefined)
    }
  })
  return (
    <a class="ae-btn ae-btn--ghost" href={backtestsPath(app.props.basePath, props.id)} data-testid="backtests-link">
      <Icon name="filter" />
      <span class="tabular-nums">{count() === undefined ? app.t('Backtests.Link') : app.t('Backtests.LinkCount', { count: count()! })}</span>
    </a>
  )
}

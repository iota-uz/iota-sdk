// Read-only outline for phones (< 768 px): triggers and steps in order of reachability, with their
// outcomes and the run overlay's states and badges (icon and text, never colour alone).
import { createMemo, For, Show, type JSX } from 'solid-js'
import { Dynamic } from 'solid-js/web'
import { catalogNode, findItem, IGNORE, itemOutcomes, reachabilityOrder } from './graph'
import { Icon } from './icons'
import { describeItem } from './names'
import { strings } from './strings'
import type { OutlineProps } from './types'

export function Outline(props: OutlineProps): JSX.Element {
  const t = () => strings(props.locale)
  const locale = () => props.locale ?? 'en'
  const order = createMemo(() => reachabilityOrder(props.graph))
  const reachable = createMemo(() => {
    const out = new Set<string>(props.graph.triggers.map((tr) => tr.id))
    const queue = [...out]
    while (queue.length > 0) {
      const current = queue.shift()!
      for (const edge of props.graph.edges) {
        if (edge.from.node === current && edge.to !== IGNORE && !out.has(edge.to)) {
          out.add(edge.to)
          queue.push(edge.to)
        }
      }
    }
    return out
  })
  const target = (node: string, outcome: string) => {
    const edge = props.graph.edges.find((e) => e.from.node === node && e.from.outcome === outcome)
    if (!edge) return t().canvas.notConnected
    if (edge.to === IGNORE) return t().canvas.ignored
    return describeItem(props.graph, props.catalog, edge.to, locale()).title
  }
  return (
    <section class="ae-outline" aria-label={t().outline.label}>
      <Show when={order().length > 0} fallback={<p class="ae-outline__empty">{t().outline.empty}</p>}>
        <ol class="ae-outline__list">
          <For each={order()}>
            {(id) => {
              const ref = () => findItem(props.graph, id)
              const name = () => describeItem(props.graph, props.catalog, id, locale())
              const state = () => props.overlay?.states.get(id)
              const badge = () => props.overlay?.badges.get(id)
              const icon = () => name().icon ?? (ref()?.kind === 'trigger' ? 'event' : catalogNode(props.catalog, (ref()?.item as { type?: string }).type ?? '')?.behavior ?? 'node')
              return (
                <li class="ae-outline__item" classList={{ 'ae-outline__item--trigger': ref()?.kind === 'trigger' }}>
                  <div role="group" aria-label={name().title}>
                  <Dynamic component={props.onPick ? 'button' : 'div'} class="ae-outline__head" classList={{ 'ae-outline__head--pick': Boolean(props.onPick) }}
                    {...(props.onPick ? { type: 'button', onClick: () => props.onPick?.(id), 'data-pick': id } : {})}>
                    <Icon name={icon()} />
                    <span class="ae-outline__title">{name().title}</span>
                    <Show when={state()}>
                      {(s) => (
                        <span class={`ae-state ae-state--${s()}`}>
                          <Icon name={s()} size={12} />
                          <span class="ae-state__text">{t().canvas.states[s()]}</span>
                        </span>
                      )}
                    </Show>
                    <Show when={badge()}>{(b) => <span class="ae-badge">{b()}</span>}</Show>
                  </Dynamic>
                  <Show when={props.overlay?.notes?.get(id)}>{(note) => <p class="ae-outline__note ae-danger">{note()}</p>}</Show>
                  <Show when={name().subtitle}>
                    <p class="ae-outline__sub">{name().subtitle}</p>
                  </Show>
                  <Show when={ref()?.kind === 'node' && !reachable().has(id)}>
                    <p class="ae-outline__note">{t().outline.unreachable}</p>
                  </Show>
                  <ul class="ae-outline__outs">
                    <For each={itemOutcomes(props.graph, props.catalog, id)}>
                      {(outcome) => (
                        <li classList={{ 'ae-outline__out--taken': props.overlay?.taken.has(`${id}.${outcome}`) ?? false }}>
                          {t().outline.goes(outcome, target(id, outcome))}
                        </li>
                      )}
                    </For>
                  </ul>
                  </div>
                </li>
              )
            }}
          </For>
        </ol>
      </Show>
    </section>
  )
}

import { For, Show, createMemo, createSignal, type Component } from 'solid-js'
import { useConfig } from './context'
import { createLoader } from './load'
import { exampleText, groupByKind, helpDocUrl, matches, pickNamespace, typeDocs } from './reference'
import type { ReferenceItem, ReferenceNamespace } from './types'

export interface ReferenceTabProps {
  /** Namespace of the selected package; its reference is shown until the user picks another. */
  current: string
}

/**
 * ReferenceTab renders the live API reference: what is installed right now
 * (native contracts and the activated package versions), as the signed-in user
 * may see it. It is the model the Help Center renders too; the picker moves
 * between namespaces and the selected package's own is highlighted.
 */
export const ReferenceTab: Component<ReferenceTabProps> = (props) => {
  const cfg = useConfig()
  const { t, api } = cfg
  const model = createLoader((s) => api.reference(undefined, s))
  const [chosen, setChosen] = createSignal<string | undefined>()
  const [filter, setFilter] = createSignal('')
  const ns = createMemo(() => pickNamespace(model.data(), chosen(), props.current))
  const groups = createMemo(() => groupByKind((ns()?.items ?? []).filter((it) => matches(it, filter()))))

  return (
    <div data-testid="reference">
      <div class="row">
        <h3>{t('Reference')}</h3>
        <button type="button" onClick={() => void model.reload()}>{t('Refresh')}</button>
        <Show when={ns()}>{(n) => (
          <a class="iota-cfg-link" data-testid="reference-help-link" href={helpDocUrl(n().name)} target="_blank" rel="noopener">{t('OpenInHelpCenter')}</a>
        )}</Show>
      </div>
      <Show when={model.error()}><p class="error" role="alert">{model.error()?.message}</p></Show>
      <Show when={model.loading() && !model.data()}><p class="muted">{t('Loading')}</p></Show>
      <Show when={(model.data()?.namespaces ?? []).length > 0} fallback={<Show when={!model.loading() && !model.error()}><p class="muted">{t('NoReference')}</p></Show>}>
        <div class="iota-cfg-reference">
          <nav class="iota-cfg-reference-nav" aria-label={t('Namespace')}>
            <ul>
              <For each={model.data()?.namespaces ?? []}>{(n) => (
                <li classList={{ selected: ns()?.name === n.name, current: n.name === props.current }}>
                  <button type="button" onClick={() => setChosen(n.name)}>
                    {n.name}
                    <Show when={n.package}> <span class="iota-cfg-badge">{n.package?.version}</span></Show>
                    <Show when={n.name === props.current}> <span class="iota-cfg-badge ok">{t('Current')}</span></Show>
                  </button>
                </li>
              )}</For>
            </ul>
          </nav>
          <div class="iota-cfg-reference-body">
            <Show when={ns()} keyed>{(n) => (
              <>
                <input type="search" placeholder={t('Filter')} value={filter()} onInput={(e) => setFilter(e.currentTarget.value)} />
                <For each={groups()}>{(g) => (
                  <section>
                    <h4><code>{g.kind}</code></h4>
                    <For each={g.items}>{(it) => <ItemView item={it} />}</For>
                  </section>
                )}</For>
                <EndpointsView ns={n} />
              </>
            )}</Show>
          </div>
        </div>
      </Show>
    </div>
  )
}

const ItemView: Component<{ item: ReferenceItem }> = (props) => {
  const { t } = useConfig()
  const it = props.item
  const docs = typeDocs(it)
  return (
    <details class="iota-cfg-reference-item" data-testid={`reference-item-${it.id}`}>
      <summary>
        <code>{it.id}</code>
        <Show when={it.label}> <span>{it.label}</span></Show>
        <Show when={it.atomic}> <span class="iota-cfg-badge">{t('Atomic')}</span></Show>
        <Show when={it.native}> <span class="iota-cfg-badge">{t('Native')}</span></Show>
        <Show when={it.package}> <span class="iota-cfg-badge">{it.package} {it.version}</span></Show>
        <Show when={it.permission}> <span class="iota-cfg-badge" title={t('Permission')}>{it.permission}</span></Show>
      </summary>
      <Show when={it.description}><p>{it.description}</p></Show>
      <Show when={it.subject}><p class="muted"><code>subject</code> <code>{it.subject}</code></p></Show>
      <Show when={it.base}><p class="muted"><code>extends</code> <code>{it.base}</code></p></Show>
      <Show when={(it.errors ?? []).length > 0}><p class="muted"><code>errors</code> <For each={it.errors}>{(e) => <><code>{e}</code>{' '}</>}</For></p></Show>
      <Show when={(it.fields ?? []).length > 0}>
        <table aria-label={t('Fields')}>
          <thead><tr><th>{t('Fields')}</th><th>type</th><th /></tr></thead>
          <tbody>
            <For each={it.fields}>{(f) => (
              <tr>
                <td><code>{f.id}</code></td>
                <td><code>{f.type}</code></td>
                <td class="muted">{[f.label, f.required ? '' : 'optional', f.unique ? 'unique' : '', ...(f.permissions ?? [])].filter(Boolean).join(' · ')}</td>
              </tr>
            )}</For>
          </tbody>
        </table>
      </Show>
      <For each={docs}>{([label, doc]) => (
        <div class="iota-cfg-reference-type">
          <p><code>{label}</code> <code>{doc.signature}</code></p>
          <details>
            <summary>{t('Example')}</summary>
            <pre class="mono">{exampleText(doc.example)}</pre>
          </details>
          <details>
            <summary>{t('Schema')}</summary>
            <pre class="mono">{exampleText(doc.schema)}</pre>
          </details>
        </div>
      )}</For>
      <Show when={(it.guards ?? []).length > 0}><p class="muted"><code>guards</code> <For each={it.guards}>{(g) => <><code>{g.id}</code> ({g.phase}){' '}</>}</For></p></Show>
    </details>
  )
}

const EndpointsView: Component<{ ns: ReferenceNamespace }> = (props) => {
  const { t } = useConfig()
  return (
    <Show when={(props.ns.endpoints ?? []).length > 0}>
      <section>
        <h4>{t('Endpoints')}</h4>
        <table>
          <tbody>
            <For each={props.ns.endpoints}>{(e) => (
              <tr><td><code>{e.method}</code></td><td><code>{e.url}</code></td><td class="muted">{e.public ? 'public' : ''}</td></tr>
            )}</For>
          </tbody>
        </table>
      </section>
    </Show>
  )
}

import { For, Match, Show, Switch, createSignal, onCleanup, onMount, type JSX } from 'solid-js'
import { Button, EmptyState, Input, Skeleton, Switch as Toggle } from '@iota-uz/sdk/solid-ui'
import { useApp } from './context'
import { DisableDialog, ReviewDialog } from './Dialogs'
import { Icon } from './Icon'
import {
  absoluteTime, errorText, graphPath, graphStatus, graphStatusText, readFilters, relativeTime, runIcon, runStatusKey, runTone, triggerIcon, writeFilters,
  type Filters, type ListItem, type ListResult, type StatusFilter,
} from './model'

const TONE: Record<string, string> = { ok: 'text-green', bad: 'text-red-600', warn: 'text-yellow', muted: 'text-300' }

/** The list of the tenant's automations at /automations (AL1, AL2). */
export function List(): JSX.Element {
  const app = useApp()
  const t = app.t
  const [filters, setFilters] = createSignal<Filters>(readFilters(window.location.search))
  const [data, setData] = createSignal<ListResult>()
  const [loadedAt, setLoadedAt] = createSignal<Date>()
  const [error, setError] = createSignal<string>()
  const [loading, setLoading] = createSignal(true)
  const [more, setMore] = createSignal(false)
  const [review, setReview] = createSignal<ListItem>()
  const [disabling, setDisabling] = createSignal<ListItem>()
  const [highlighted, setHighlighted] = createSignal<string>()
  let seq = 0
  let highlightTimer: ReturnType<typeof setTimeout> | undefined
  onCleanup(() => clearTimeout(highlightTimer))

  // The answer to an older request never replaces a newer one (S6).
  const load = async () => {
    const mine = ++seq
    const f = filters()
    setLoading(true)
    setError(undefined)
    try {
      const res = await app.call('automations.list', { ...(f.query ? { query: f.query } : {}), ...(f.status ? { status: f.status } : {}) })
      if (mine !== seq) return
      setData(res)
      setLoadedAt(new Date())
    } catch (e) {
      if (mine !== seq) return
      setError(errorText(app.t, e, t('Actions.LoadList')))
    } finally {
      if (mine === seq) setLoading(false)
    }
  }
  const loadMore = async () => {
    const d = data()
    if (!d?.next_cursor || more()) return
    const f = filters()
    const mine = seq
    setMore(true)
    try {
      const res = await app.call('automations.list', { cursor: d.next_cursor, ...(f.query ? { query: f.query } : {}), ...(f.status ? { status: f.status } : {}) })
      if (mine !== seq) return
      setData({ ...res, items: [...d.items, ...res.items] })
    } catch (e) {
      app.toast.notify({ variant: 'error', title: errorText(t, e, t('Actions.LoadList')), action: { label: t('Common.Retry'), onAction: () => void loadMore() } })
    } finally {
      setMore(false)
    }
  }
  onMount(() => {
    void load()
    const onPop = () => { setFilters(readFilters(window.location.search)); void load() }
    window.addEventListener('popstate', onPop)
    const up = () => void load()
    window.addEventListener('online', up)
    onCleanup(() => { window.removeEventListener('popstate', onPop); window.removeEventListener('online', up) })
  })

  let searchTimer: ReturnType<typeof setTimeout> | undefined
  onCleanup(() => clearTimeout(searchTimer))
  const apply = (f: Filters, push: boolean) => {
    setFilters(f)
    const href = writeFilters(window.location.href, f)
    if (push) window.history.pushState({}, '', href)
    else window.history.replaceState({}, '', href)
    void load()
  }
  const onSearch = (value: string) => {
    clearTimeout(searchTimer)
    searchTimer = setTimeout(() => apply({ ...filters(), query: value.trim() }, false), 250)
  }
  const filtered = () => Boolean(filters().query || filters().status)
  const reset = () => apply({ query: '', status: '' }, true)

  const changed = (id: string) => {
    setHighlighted(id)
    clearTimeout(highlightTimer)
    highlightTimer = setTimeout(() => setHighlighted(undefined), 4000)
    void load()
  }

  const statusOptions: { value: StatusFilter | ''; key: string }[] = [
    { value: '', key: 'Filter.All' }, { value: 'enabled', key: 'Filter.Enabled' }, { value: 'disabled', key: 'Filter.Disabled' }, { value: 'attention', key: 'Filter.Attention' },
  ]
  const can = () => data()?.can ?? app.props.can
  const create = () => (
    <Show when={can().author}>
      <Button variant="primary" href={`${app.props.basePath}/new`} icon={<Icon name="plus" />} data-testid="create">{t('List.Create')}</Button>
    </Show>
  )

  const statusCell = (item: ListItem) => {
    const s = graphStatus(item)
    return (
      <div class="flex flex-col gap-1">
        <span class="flex items-center gap-1.5 whitespace-nowrap" data-testid="status">
          <Icon name={s.kind === 'enabled' ? 'play' : s.kind === 'pending' ? 'alert' : 'pause'} class={s.kind === 'enabled' ? 'text-green' : s.kind === 'pending' ? 'text-yellow' : 'text-300'} />
          {graphStatusText(t, s)}
        </span>
        <Show when={item.saved_by_agent && item.pending_version}>
          <span class="flex items-center gap-1 text-xs text-300" data-testid="saved-by-agent"><Icon name="robot" size={14} />{t('List.SavedByAgent')}</span>
        </Show>
      </div>
    )
  }
  const toggle = (item: ListItem) => (
    <Show when={can().enable}>
      <Toggle
        size="sm"
        checked={item.enabled}
        aria-label={item.enabled ? t('List.DisableNamed', { name: item.name || t('List.Untitled') }) : t('List.EnableNamed', { name: item.name || t('List.Untitled') })}
        onChange={(event) => {
          event.currentTarget.checked = item.enabled
          if (item.enabled) setDisabling(item)
          else setReview(item)
        }}
        data-testid="toggle"
      />
    </Show>
  )
  const triggers = (item: ListItem) => (
    <Show when={item.triggers.length > 0} fallback={<span class="text-300">{t('List.NoTriggers')}</span>}>
      <ul class="flex flex-col gap-0.5">
        <For each={item.triggers}>{(tr) => <li class="flex items-center gap-1.5"><Icon name={triggerIcon(tr.kind)} class="text-300" /><span class="truncate" title={tr.title}>{tr.title}</span></li>}</For>
      </ul>
    </Show>
  )
  const lastRun = (item: ListItem) => (
    <Show when={item.last_run} fallback={<span class="text-300">{t('List.NeverRan')}</span>}>
      {(r) => (
        <a class="flex flex-col hover:underline" href={graphPath(app.props.basePath, item.id, { run: r().run_id })} data-testid="last-run">
          <span class={`flex items-center gap-1.5 ${TONE[runTone(r().status)]}`}><Icon name={runIcon(r().status)} />{t(runStatusKey(r().status))}</span>
          <time class="text-xs text-300 tabular-nums" datetime={r().at} title={absoluteTime(r().at, app.lang)}>{relativeTime(r().at, app.now(), app.lang)}</time>
        </a>
      )}
    </Show>
  )
  const attention = (item: ListItem) => (
    <Show when={item.attention > 0} fallback={<span class="text-300" aria-label={t('List.NoAttention')}>—</span>}>
      <a class="inline-flex items-center gap-1 text-yellow tabular-nums hover:underline" href={graphPath(app.props.basePath, item.id, { tab: 'runs' })} data-testid="attention">
        <Icon name="alert" />{t('List.AttentionCount', { count: item.attention })}
      </a>
    </Show>
  )
  const updated = (item: ListItem) => (
    <span class="flex flex-col text-sm">
      <span class="truncate">{item.updated_by}</span>
      <time class="text-xs text-300 tabular-nums" datetime={item.updated_at} title={absoluteTime(item.updated_at, app.lang)}>{relativeTime(item.updated_at, app.now(), app.lang)}</time>
    </span>
  )
  const name = (item: ListItem) => (
    <a class="font-medium text-100 hover:underline break-words" href={graphPath(app.props.basePath, item.id)} data-testid="name">{item.name || t('List.Untitled')}</a>
  )

  return (
    <div class="flex flex-col gap-4 p-4 md:p-6 min-w-0" data-testid="automations-list">
      <header class="flex items-center justify-between gap-3">
        <h1 class="text-xl font-semibold truncate">{t('Title')}</h1>
        {create()}
      </header>
      <div class="flex flex-col gap-3 md:flex-row md:items-center">
        <div class="md:w-80">
          <Input
            type="text" placeholder={t('List.Search')} aria-label={t('List.Search')} value={filters().query}
            addonLeft={<Icon name="search" class="text-300" />}
            onInput={(e) => onSearch(e.currentTarget.value)}
            data-testid="search"
          />
        </div>
        <div role="group" aria-label={t('Filter.Label')} class="flex flex-wrap gap-1" data-testid="status-filter">
          <For each={statusOptions}>
            {(o) => (
              <button
                type="button"
                class="min-h-9 [@media(pointer:coarse)]:min-h-11 rounded-md border px-3 text-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-500"
                classList={{ 'border-brand bg-brand/10 text-brand font-medium': filters().status === o.value, 'border-subtle text-200': filters().status !== o.value }}
                aria-pressed={filters().status === o.value}
                onClick={() => apply({ ...filters(), status: o.value }, true)}
              >{t(o.key)}</button>
            )}
          </For>
        </div>
      </div>
      <Show when={data() && data()!.items.length > 0 && !can().enable}>
        {/* AL7: why the rows have no switch, in words. */}
        <p class="flex items-center gap-2 text-sm text-200" data-testid="enable-note"><Icon name="lock" />{app.props.agent ? t('Enable.AgentNote') : t('Enable.NoRight')}</p>
      </Show>
      <Show when={error() && data()}>
        <p role="alert" class="flex flex-wrap items-center gap-2 text-sm text-300" data-testid="list-stale">
          <Icon name="alert" class="text-yellow" />{t('List.Stale', { time: loadedAt() ? absoluteTime(loadedAt()!.toISOString(), app.lang) : '' })}
          <Button variant="ghost" size="sm" onClick={() => void load()}>{t('Common.Retry')}</Button>
        </p>
      </Show>
      <Switch>
        <Match when={!data() && error()}>
          <div role="alert" class="flex flex-col items-start gap-3 rounded-lg border border-subtle p-6" data-testid="list-error">
            <p>{error()}</p>
            <Button variant="secondary" onClick={() => void load()} icon={<Icon name="refresh" />}>{t('Common.Retry')}</Button>
          </div>
        </Match>
        <Match when={!data() && loading()}>
          <div role="status" aria-busy="true" aria-label={t('Common.Loading')} data-testid="list-loading"><Skeleton variant="table" lines={6} /></div>
        </Match>
        <Match when={data() && data()!.items.length === 0 && !filtered()}>
          <EmptyState
            data-testid="list-empty"
            title={t('List.EmptyTitle')}
            description={
              <span class="flex flex-col items-center gap-3">
                <span>{can().author ? t('List.EmptyText') : t('List.EmptyTextReader')}</span>
                <Show when={can().author}>
                  <Button variant="primary" href={`${app.props.basePath}/new`} icon={<Icon name="plus" />}>{t('List.CreateFirst')}</Button>
                </Show>
              </span>
            }
          />
        </Match>
        <Match when={data() && data()!.items.length === 0}>
          <EmptyState
            data-testid="list-empty-filtered"
            title={filters().query ? t('List.NothingFor', { query: filters().query }) : t('List.NothingFiltered')}
            description={<Button variant="secondary" onClick={reset}>{t('List.ResetFilters')}</Button>}
          />
        </Match>
        <Match when={data()}>
          <div class="hidden md:block overflow-x-auto rounded-lg border border-subtle bg-surface-600" aria-busy={loading() || undefined}>
            <table class="w-full text-sm" data-testid="list-table">
              <thead class="bg-surface-500 text-left text-xs text-200">
                <tr>
                  <th scope="col" class="px-4 py-2 font-medium">{t('Columns.Name')}</th>
                  <th scope="col" class="px-4 py-2 font-medium">{t('Columns.Triggers')}</th>
                  <th scope="col" class="px-4 py-2 font-medium">{t('Columns.Status')}</th>
                  <th scope="col" class="px-4 py-2 font-medium">{t('Columns.LastRun')}</th>
                  <th scope="col" class="px-4 py-2 font-medium">{t('Columns.Attention')}</th>
                  <th scope="col" class="px-4 py-2 font-medium">{t('Columns.Updated')}</th>
                </tr>
              </thead>
              <tbody>
                <For each={data()!.items}>
                  {(item) => (
                    <tr class="border-t border-subtle align-top transition-colors" classList={{ 'bg-brand/5': highlighted() === item.id }} data-testid={`row-${item.id}`}>
                      <td class="px-4 py-3 max-w-xs">{name(item)}<Show when={item.description}><p class="text-xs text-300 line-clamp-2">{item.description}</p></Show></td>
                      <td class="px-4 py-3 max-w-xs">{triggers(item)}</td>
                      <td class="px-4 py-3"><div class="flex items-start gap-3">{toggle(item)}{statusCell(item)}</div></td>
                      <td class="px-4 py-3">{lastRun(item)}</td>
                      <td class="px-4 py-3">{attention(item)}</td>
                      <td class="px-4 py-3">{updated(item)}</td>
                    </tr>
                  )}
                </For>
              </tbody>
            </table>
          </div>
          <ul class="flex flex-col gap-3 md:hidden" data-testid="list-cards">
            <For each={data()!.items}>
              {(item) => (
                <li class="flex flex-col gap-3 rounded-lg border border-subtle bg-surface-600 p-4 min-w-0" classList={{ 'bg-brand/5': highlighted() === item.id }} data-testid={`card-${item.id}`}>
                  <div class="flex items-start justify-between gap-3 min-w-0">
                    <div class="min-w-0">{name(item)}</div>
                    {toggle(item)}
                  </div>
                  {statusCell(item)}
                  <div class="text-sm">{triggers(item)}</div>
                  <div class="flex flex-wrap items-start justify-between gap-3 text-sm">{lastRun(item)}{attention(item)}</div>
                  <div class="text-xs text-300">{updated(item)}</div>
                </li>
              )}
            </For>
          </ul>
          <Show when={data()!.next_cursor}>
            <div class="flex justify-center">
              <Button variant="secondary" loading={more()} onClick={() => void loadMore()} data-testid="load-more">{t('List.LoadMore')}</Button>
            </div>
          </Show>
        </Match>
      </Switch>
      <Show when={review()}>
        {(item) => (
          <ReviewDialog
            id={item().id} version={item().latest_version} name={item().name}
            onEnabled={() => { const id = item().id; setReview(undefined); changed(id) }}
            onClose={() => setReview(undefined)}
          />
        )}
      </Show>
      <Show when={disabling()}>
        {(item) => (
          <DisableDialog
            id={item().id} name={item().name}
            onDisabled={() => { const id = item().id; setDisabling(undefined); changed(id) }}
            onClose={() => setDisabling(undefined)}
          />
        )}
      </Show>
    </div>
  )
}

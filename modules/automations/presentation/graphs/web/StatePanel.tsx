import { For, Show, createSignal, onCleanup, onMount, type JSX } from 'solid-js'
import { Button, Input, Skeleton, Textarea } from '@iota-uz/sdk/solid-ui'
import type { RPC } from './Automations.solid.generated'
import { useApp } from './context'
import { absoluteTime, errorCode, errorText, relativeTime } from './model'

type Entry = RPC['automations.state_list']['result']['items'][number]

/** The most characters of a value shown in a row; the editor shows the whole value. */
const PREVIEW = 120
const SEARCH_DELAY_MS = 250

/** A value as a row shows it: its JSON, cut. */
export function preview(value: unknown): string {
  const text = JSON.stringify(value) ?? 'null'
  return text.length > PREVIEW ? `${text.slice(0, PREVIEW)}…` : text
}

/** The value of the editor as JSON text; ok false when it is not JSON. */
export function parseValue(text: string): { ok: true; value: unknown } | { ok: false } {
  if (text.trim() === '') return { ok: false }
  try {
    return { ok: true, value: JSON.parse(text) as unknown }
  } catch {
    return { ok: false }
  }
}

interface Draft {
  /** Names the draft: edits keep it, a new draft has another. */
  uid: number
  /** The key as it was; null for a key being added. */
  original: string | null
  key: string
  value: string
  ttl: string
}

/**
 * The memory of a graph between its runs (#1390): the keys its «Set state» nodes keep, with a search and a
 * change or a delete of each key. It sits in the settings of the graph. Reading needs the page's own
 * rights; changing needs Automations.Author (canEdit).
 */
export function StatePanel(props: { id: string; canEdit: boolean }): JSX.Element {
  const app = useApp()
  const t = app.t
  const [items, setItems] = createSignal<Entry[]>([])
  const [total, setTotal] = createSignal(0)
  const [next, setNext] = createSignal<string | null>(null)
  const [query, setQuery] = createSignal('')
  const [loading, setLoading] = createSignal(false)
  const [loadError, setLoadError] = createSignal<string>()
  const [draft, setDraft] = createSignal<Draft>()
  const [draftError, setDraftError] = createSignal<string>()
  const [confirming, setConfirming] = createSignal<string>()
  const [deleteError, setDeleteError] = createSignal<string>()
  const [busy, setBusy] = createSignal(false)
  let list: HTMLUListElement | undefined
  let section: HTMLElement | undefined
  // The inline confirmation takes the focus on «Cancel» (U1); closing it gives the focus back to «Delete»
  // of the same key, so the keyboard never falls to the page's start (K3).
  // No other confirmation opens while a delete or a save runs: its answer belongs to the row that asked.
  const ask = (key: string) => { if (busy()) return; setDeleteError(undefined); setConfirming(key) }
  const dismiss = () => {
    const key = confirming()
    setConfirming(undefined)
    setDeleteError(undefined)
    if (key === undefined) return
    queueMicrotask(() => {
      for (const li of list?.querySelectorAll<HTMLElement>('li[data-key]') ?? []) {
        if (li.dataset.key === key) { li.querySelector<HTMLElement>('[data-action="state-delete"]')?.focus(); return }
      }
    })
  }

  let seq = 0
  const load = async (more = false) => {
    // No page is asked for while another load runs: a reload of the first page (after a change) would be
    // dropped, and the next page appended to rows that are out of date.
    if (more && loading()) return
    const mine = ++seq
    setLoading(true)
    setLoadError(undefined)
    try {
      const res = await app.call('automations.state_list', {
        id: props.id, ...(query().trim() ? { search: query().trim() } : {}), ...(more && next() ? { cursor: next()! } : {}),
      })
      if (mine !== seq) return
      setItems(more ? [...items(), ...res.items] : res.items)
      setTotal(res.total)
      setNext(res.next_cursor)
    } catch (e) {
      if (mine === seq) setLoadError(errorText(t, e, t('Actions.LoadState')))
    } finally {
      if (mine === seq) setLoading(false)
    }
  }
  onMount(() => void load())

  let timer: ReturnType<typeof setTimeout> | undefined
  onCleanup(() => clearTimeout(timer))
  const search = (value: string) => {
    setQuery(value)
    // Pages of the old query are not continued under the new one: its pending loads are dropped and "Show
    // more" waits for the new first page.
    seq++
    setNext(null)
    clearTimeout(timer)
    timer = setTimeout(() => void load(), SEARCH_DELAY_MS)
  }

  let drafts = 0
  const edit = (e: Entry) => {
    setDraftError(undefined)
    setDraft({ uid: ++drafts, original: e.key, key: e.key, value: JSON.stringify(e.value, null, 2) ?? 'null', ttl: '' })
  }
  const add = () => {
    setDraftError(undefined)
    setDraft({ uid: ++drafts, original: null, key: '', value: '', ttl: '' })
  }
  const patch = (p: Partial<Draft>) => setDraft((d) => (d ? { ...d, ...p } : d))

  const save = async () => {
    const d = draft()
    if (!d || busy()) return
    // A key that is there is saved as it is (its field is read only); only a new key is trimmed.
    const key = d.original ?? d.key.trim()
    if (key === '') { setDraftError(t('State.KeyRequired')); return }
    const parsed = parseValue(d.value)
    if (!parsed.ok) { setDraftError(t('State.InvalidJSON')); return }
    setBusy(true)
    setDraftError(undefined)
    try {
      await app.call('automations.state_set', { id: props.id, key, value: parsed.value, ...(d.ttl.trim() ? { ttl: d.ttl.trim() } : {}) })
      app.toast.notify({ variant: 'success', title: t('State.Saved') })
      // Only what was saved goes: another draft, or edits made to this one while it ran, stay open.
      if (draft() === d) setDraft(undefined)
      await load()
    } catch (e) {
      // A refusal of the memory (a value over 16 KiB, a ttl out of range) carries its own words.
      const message = errorCode(e) === 'invalid_input' ? (e as { message?: string }).message : undefined
      if (draft()?.uid === d.uid) setDraftError(message || errorText(t, e, t('Actions.SaveState')))
    } finally {
      setBusy(false)
    }
  }

  const remove = async (key: string) => {
    if (busy()) return
    setBusy(true)
    setDeleteError(undefined)
    try {
      await app.call('automations.state_delete', { id: props.id, key })
      app.toast.notify({ variant: 'success', title: t('State.Deleted') })
      setConfirming(undefined)
      // The row and its buttons go: the focus moves to the search, not to the page's start.
      queueMicrotask(() => section?.querySelector<HTMLElement>('[data-testid="state-search"]')?.focus())
      await load()
    } catch (e) {
      // A key that expired meanwhile is gone either way.
      if (errorCode(e) === 'not_found') { setConfirming(undefined); await load() }
      // The refusal is told next to the action it answers (F3), the confirmation stays open.
      else setDeleteError(errorText(t, e, t('Actions.DeleteState')))
    } finally {
      setBusy(false)
    }
  }

  return (
    <section ref={section} class="flex flex-col gap-3" aria-labelledby="ae-state-title" data-testid="state-panel">
      <div class="flex items-start justify-between gap-3">
        <div class="flex flex-col gap-1">
          <h3 id="ae-state-title" class="text-sm font-medium">{t('State.Title')}</h3>
          <p class="text-xs text-300">{t('State.Hint')}</p>
        </div>
        <Show when={props.canEdit && !draft()}>
          <Button type="button" variant="secondary" data-testid="state-add" onClick={add}>{t('State.Add')}</Button>
        </Show>
      </div>

      <Show when={draft()}>
        {(d) => (
          <form class="iota-reveal flex flex-col gap-3 rounded-md border border-subtle p-3" data-testid="state-editor"
            onSubmit={(e) => { e.preventDefault(); void save() }}>
            <Input label={t('State.Key')} value={d().key} maxLength={200} readOnly={d().original !== null} data-field="state-key"
              onInput={(e) => patch({ key: e.currentTarget.value })} />
            <Textarea label={t('State.Value')} description={t('State.ValueHint')} value={d().value} rows={5} data-field="state-value"
              onInput={(e) => patch({ value: e.currentTarget.value })} />
            <Input label={t('State.Ttl')} description={t('State.TtlHint')} value={d().ttl} placeholder="2h" data-field="state-ttl"
              onInput={(e) => patch({ ttl: e.currentTarget.value })} />
            <Show when={draftError()}><p role="alert" class="text-sm text-danger" data-testid="state-error">{draftError()}</p></Show>
            <div class="flex justify-end gap-2">
              <Button type="button" variant="secondary" onClick={() => setDraft(undefined)}>{t('Common.Cancel')}</Button>
              <Button type="submit" variant="primary" loading={busy()} data-testid="state-save">{t('State.Save')}</Button>
            </div>
          </form>
        )}
      </Show>

      <Input type="text" aria-label={t('State.Search')} placeholder={t('State.Search')} value={query()} data-testid="state-search"
        onInput={(e) => search(e.currentTarget.value)} />

      <Show when={loadError()}><p role="alert" class="text-sm text-danger">{loadError()}</p></Show>

      <Show when={items().length > 0} fallback={
        // The first load draws the rows' shape (S2); an empty answer says so in words.
        <Show when={!(loading() && total() === 0 && !query().trim())} fallback={<div role="status" aria-busy="true" aria-label={t('State.Loading')} data-testid="state-loading"><Skeleton lines={3} /></div>}>
          <p class="text-sm text-300" data-testid="state-empty">{loading() ? t('State.Loading') : query().trim() ? t('State.NoMatch') : t('State.Empty')}</p>
        </Show>
      }>
        <ul ref={list} class="iota-fade-in flex max-h-72 flex-col divide-y divide-subtle overflow-y-auto rounded-md border border-subtle" data-testid="state-list">
          <For each={items()}>
            {(e) => (
              <li class="flex items-start justify-between gap-3 px-3 py-2" data-key={e.key}>
                <div class="flex min-w-0 flex-col gap-1">
                  <code class="break-all text-sm font-medium">{e.key}</code>
                  <code class="break-all text-xs text-300">{preview(e.value)}</code>
                  <span class="text-xs text-300">
                    {e.expires_at ? t('State.ExpiresAt', { when: absoluteTime(e.expires_at, app.lang) }) : t('State.NoExpiry')}
                    {' · '}{t('State.UpdatedAt', { when: relativeTime(e.updated_at, app.now(), app.lang) })}
                  </span>
                </div>
                <Show when={props.canEdit}>
                  <div class="flex shrink-0 flex-col items-end gap-1">
                    <Show when={confirming() === e.key} fallback={
                      <div class="flex gap-2">
                        <Button type="button" variant="secondary" aria-label={`${t('State.Edit')}: ${e.key}`} onClick={() => edit(e)}>{t('State.Edit')}</Button>
                        <Button type="button" variant="secondary" aria-label={`${t('State.Delete')}: ${e.key}`} data-action="state-delete" onClick={() => ask(e.key)}>{t('State.Delete')}</Button>
                      </div>
                    }>
                      {/* U3: the confirmation opens in place; deleting a key is irreversible, so it is danger-toned. */}
                      <div class="iota-reveal flex gap-2" onKeyDown={(ev) => { if (ev.key === 'Escape' && !busy()) { ev.stopPropagation(); dismiss() } }}>
                        <Button type="button" variant="danger" loading={busy()} data-testid="state-confirm-delete" onClick={() => void remove(e.key)}>{t('State.ConfirmDelete')}</Button>
                        <Button ref={(el: HTMLButtonElement) => queueMicrotask(() => el.focus())} type="button" variant="secondary" aria-disabled={busy() || undefined}
                          onClick={() => { if (!busy()) dismiss() }}>{t('Common.Cancel')}</Button>
                      </div>
                      <Show when={deleteError()}><p role="alert" class="text-xs text-danger" data-testid="state-delete-error">{deleteError()}</p></Show>
                    </Show>
                  </div>
                </Show>
              </li>
            )}
          </For>
        </ul>
      </Show>

      <div class="flex items-center justify-between gap-3 text-xs text-300">
        <span data-testid="state-total">{t('State.Total', { count: total(), max: 10000 })}</span>
        <Show when={next()}>
          <Button type="button" variant="secondary" loading={loading()} disabled={loading()} data-testid="state-more" onClick={() => void load(true)}>{t('State.More')}</Button>
        </Show>
      </div>
    </section>
  )
}

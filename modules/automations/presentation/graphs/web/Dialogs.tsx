import { For, Match, Show, Switch, createSignal, onMount, type JSX } from 'solid-js'
import { Button, Dialog, DialogBusyStatus, Skeleton } from '@iota-uz/sdk/solid-ui'
import { useApp } from './context'
import { Icon } from './Icon'
import { counted, errorText, highlight, problemText, triggerIcon, type ReviewResult } from './model'

type GraphError = { path: string; message: string; code?: string }

const TOKEN_CLASS: Record<string, string> = {
  keyword: 'text-purple font-medium', string: 'text-green', comment: 'text-200 italic', number: 'text-blue', plain: '',
}

/** Source with syntax highlighting; the text is rendered as text, never as HTML. */
export function Code(props: { source: string; label: string }): JSX.Element {
  return (
    <pre class="max-h-72 overflow-auto rounded-md border border-subtle bg-surface-500 p-3 text-xs leading-5 font-mono" tabindex="0" aria-label={props.label} data-testid="code-source">
      <code><For each={highlight(props.source)}>{(tok) => <span class={TOKEN_CLASS[tok.kind]}>{tok.text}</span>}</For></code>
    </pre>
  )
}

function closeButton(onClose: () => void, label: string): JSX.Element {
  return (
    <button type="button" class="drawer-close" aria-label={label} data-dialog-close onClick={onClose}>
      <Icon name="x" size={20} />
    </button>
  )
}

/**
 * «Review before enabling» (AL6): every Code node of the version with its source, the names of the
 * secrets it declares and whether it may fetch, the triggers and whose rights the runs have. Enabling
 * sends the digest received with what is on screen: a version that changed meanwhile is refused and the
 * dialog reloads with the new code (U2). While the request runs the dialog cannot be closed (F4).
 */
export function ReviewDialog(props: { id: string; version: number; name: string; onEnabled: (version: number) => void; onClose: () => void }): JSX.Element {
  const app = useApp()
  const t = app.t
  const [review, setReview] = createSignal<ReviewResult>()
  const [loadError, setLoadError] = createSignal<string>()
  const [busy, setBusy] = createSignal(false)
  const [notice, setNotice] = createSignal<string>()
  const [errors, setErrors] = createSignal<GraphError[]>([])
  // After a refusal the latest version is reviewed (the one saved meanwhile), retries included.
  let latest = false
  const load = async (changed = false) => {
    if (changed) latest = true
    setLoadError(undefined)
    // What is on screen no longer is what would be enabled: nothing is confirmed until the review loads.
    setReview(undefined)
    try {
      const r = await app.call('automations.enable_review', { id: props.id, version: latest ? 0 : props.version })
      setReview(r)
      if (latest) setNotice(t('Review.Changed'))
    } catch (error) {
      setLoadError(errorText(t, error, t('Actions.LoadReview')))
    }
  }
  onMount(() => void load())
  const enable = async () => {
    const r = review()
    if (!r || busy()) return
    setBusy(true)
    setNotice(undefined)
    setErrors([])
    try {
      const res = await app.call('automations.enable', { id: props.id, version: r.version, code_digest: r.code_digest })
      if (res.ok) {
        setBusy(false)
        app.toast.notify({ variant: 'success', title: t('Review.Enabled', { version: res.enabled_version ?? r.version }) })
        props.onEnabled(res.enabled_version ?? r.version)
        return
      }
      switch (res.error) {
        case 'review_required':
          await load(true)
          break
        case 'VALIDATION_FAILED':
          setErrors(res.errors ?? [])
          setNotice(t('Review.Invalid'))
          break
        case 'HUMAN_REQUIRED':
          setNotice(t('Errors.HumanRequired'))
          break
        default:
          setNotice(t('Errors.Forbidden'))
      }
    } catch (error) {
      setNotice(errorText(t, error, t('Actions.Enable')))
    } finally {
      setBusy(false)
    }
  }
  return (
    <Dialog open busy={busy()} onOpenChange={(open) => { if (!open) props.onClose() }} class="md:w-full md:max-w-3xl" aria-labelledby="review-title" data-testid="review-dialog">
      <header class="flex items-center justify-between gap-3 border-b border-subtle py-1 pl-4 pr-2">
        <h3 id="review-title" class="font-medium truncate" data-dialog-title tabindex="-1">{t('Review.Title', { name: props.name || t('List.Untitled'), version: review()?.version ?? props.version })}</h3>
        {closeButton(props.onClose, t('Common.Close'))}
      </header>
      <div class="flex max-h-[70vh] flex-col gap-4 overflow-y-auto px-4 py-3" data-dialog-body>
        <Switch>
          <Match when={loadError()}>
            <div role="alert" class="flex flex-col items-start gap-2" data-testid="review-load-error">
              <p>{loadError()}</p>
              <Button variant="secondary" size="sm" onClick={() => void load()}>{t('Common.Retry')}</Button>
            </div>
          </Match>
          <Match when={!review()}>
            <div role="status" aria-busy="true" aria-label={t('Common.Loading')} data-testid="review-loading"><Skeleton lines={6} /></div>
          </Match>
          <Match when={review()}>
            {(r) => (
              <>
                <Show when={notice()}>
                  <p role="alert" class="flex items-start gap-2 text-danger" data-testid="review-notice"><Icon name="alert" class="mt-0.5" />{notice()}</p>
                </Show>
                <Show when={errors().length > 0}>
                  <ul class="list-disc pl-5 text-sm" data-testid="review-errors"><For each={errors()}>{(e) => <li>{e.message}</li>}</For></ul>
                </Show>
                <section aria-labelledby="review-triggers" class="flex flex-col gap-1">
                  <h4 id="review-triggers" class="text-sm font-medium">{t('Review.Triggers')}</h4>
                  <Show when={r().triggers.length > 0} fallback={<p class="text-sm text-300">{t('Review.NoTriggers')}</p>}>
                    <ul class="flex flex-col gap-1 text-sm" data-testid="review-trigger-list">
                      <For each={r().triggers}>{(tr) => <li class="flex items-center gap-2"><Icon name={triggerIcon(tr.kind)} /><span>{tr.title}</span></li>}</For>
                    </ul>
                  </Show>
                </section>
                <Show when={r().code_nodes.length > 0}>
                  <section aria-labelledby="review-runas" class="flex flex-col gap-1">
                    <h4 id="review-runas" class="text-sm font-medium">{t('Review.RunAs')}</h4>
                    <p class="text-sm" data-testid="review-runas">{r().run_as === 'initiator' ? t('Review.RunAsInitiator') : t('Review.RunAsOwner')}</p>
                  </section>
                  <section aria-labelledby="review-code" class="flex flex-col gap-3">
                    <h4 id="review-code" class="text-sm font-medium">{t('Review.Code')}</h4>
                    <Show when={r().code_nodes.length > 0} fallback={<p class="text-sm text-300" data-testid="review-no-code">{t('Review.NoCode')}</p>}>
                      <For each={r().code_nodes}>
                        {(n) => (
                          <article class="flex flex-col gap-2 rounded-lg border border-subtle p-3" data-testid={`review-node-${n.node_id}`}>
                            <header class="flex flex-wrap items-center gap-2 text-sm">
                              <Icon name="code" />
                              <span class="font-medium">{n.title}</span>
                              <span class="rounded border border-subtle px-1.5 text-xs uppercase">{n.lang}</span>
                              <span class="flex items-center gap-1 text-xs text-300">
                                <Icon name={n.fetch ? 'globe' : 'lock'} size={14} />{n.fetch ? t('Review.FetchAllowed') : t('Review.FetchDenied')}
                              </span>
                            </header>
                            <Code source={n.source} label={t('Review.SourceOf', { node: n.title })} />
                            <div class="flex flex-wrap items-center gap-2 text-xs">
                              <span class="text-300">{t('Review.Secrets')}</span>
                              <Show when={n.secrets.length > 0} fallback={<span>{t('Review.NoSecrets')}</span>}>
                                <For each={n.secrets}>{(s) => <span class="inline-flex items-center gap-1 rounded border border-subtle px-1.5 font-mono"><Icon name="key" size={12} />{s}</span>}</For>
                              </Show>
                            </div>
                          </article>
                        )}
                      </For>
                    </Show>
                  </section>
                </Show>
              </>
            )}
          </Match>
        </Switch>
      </div>
      <footer class="border-t border-subtle px-4 py-3 pb-[max(0.75rem,env(safe-area-inset-bottom))]">
        <DialogBusyStatus class="mb-2" />
        {/* AL7: why «Enable» is off, in text next to it (also on touch), not only in a title. */}
        <Show when={(review()?.problems.length ?? 0) > 0}>
          <div id="review-blocked" role="note" class="mb-3 flex flex-col gap-1 text-sm" data-testid="review-blocked">
            <p class="flex items-start gap-2 font-medium"><Icon name="alert" class="mt-0.5 text-yellow" />{t('Review.Blocked')}</p>
            <ul class="list-disc pl-6 text-200"><For each={review()!.problems}>{(p) => <li>{problemText(t, p)}</li>}</For></ul>
          </div>
        </Show>
        <menu class="flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
          <Button variant="secondary" class="min-h-11 justify-center" data-dialog-close onClick={props.onClose} aria-disabled={busy() || undefined}>{t('Common.Cancel')}</Button>
          <Button variant="primary" class="min-h-11 justify-center" loading={busy()} disabled={!review() || Boolean(loadError()) || review()!.problems.length > 0}
            aria-describedby={(review()?.problems.length ?? 0) > 0 ? 'review-blocked' : undefined} onClick={() => void enable()} data-testid="review-enable">
            {busy() ? t('Review.Enabling') : t('Review.Enable', { version: review()?.version ?? props.version })}
          </Button>
        </menu>
      </footer>
    </Dialog>
  )
}

/** What disabling or deleting cancels, read before the action may be confirmed. */
function useLive(id: string) {
  const app = useApp()
  const [live, setLive] = createSignal<{ runs: number; waits: number }>()
  const [error, setError] = createSignal<string>()
  const load = async () => {
    setError(undefined)
    try {
      setLive(await app.call('automations.live', { id }))
    } catch (e) {
      setError(errorText(app.t, e, app.t('Actions.LoadLive')))
    }
  }
  onMount(() => void load())
  return { live, error, load }
}

/**
 * A confirmation of an irreversible action (AL8, U1): the default focus is «Cancel», the action is
 * confirmed only once its consequence is known (ready), and a request in flight keeps it open (F4).
 */
function SafeConfirm(props: {
  heading: string; text: JSX.Element; confirm: string; busy: boolean; ready: boolean; loadError?: string | undefined; onRetry: () => void
  onConfirm: () => void; onClose: () => void; testid: string; error?: string | undefined
}): JSX.Element {
  const app = useApp()
  const t = app.t
  let cancel: HTMLButtonElement | undefined
  return (
    <Dialog open busy={props.busy} onOpenChange={(open) => { if (!open) props.onClose() }} initialFocus={() => cancel} class="md:w-full md:max-w-md" aria-labelledby={`${props.testid}-title`} data-testid={props.testid}>
      <header class="flex items-center justify-between gap-3 border-b border-subtle py-1 pl-4 pr-2">
        <h3 id={`${props.testid}-title`} class="font-medium truncate" data-dialog-title tabindex="-1">{props.heading}</h3>
        {closeButton(props.onClose, t('Common.Close'))}
      </header>
      <div class="flex min-h-24 flex-col justify-center gap-2 px-4 py-3" data-dialog-body>
        <Show when={props.loadError} fallback={<p>{props.text}</p>}>
          <div role="alert" class="flex flex-wrap items-center gap-2" data-testid={`${props.testid}-load-error`}>
            <span>{props.loadError}</span>
            <Button variant="secondary" size="sm" onClick={props.onRetry}>{t('Common.Retry')}</Button>
          </div>
        </Show>
        <Show when={props.error}><p role="alert" class="text-sm text-danger">{props.error}</p></Show>
      </div>
      <footer class="border-t border-subtle px-4 py-3 pb-[max(0.75rem,env(safe-area-inset-bottom))]">
        <DialogBusyStatus class="mb-2" />
        <menu class="flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
          <Button ref={(el: HTMLButtonElement) => { cancel = el }} variant="secondary" class="min-h-11 justify-center" data-dialog-close data-testid="confirm-cancel" onClick={props.onClose} aria-disabled={props.busy || undefined}>{t('Common.Cancel')}</Button>
          <Button variant="danger" class="min-h-11 justify-center" loading={props.busy} disabled={!props.ready} onClick={() => { if (props.ready && !props.busy) props.onConfirm() }} data-testid="confirm-action">{props.confirm}</Button>
        </menu>
      </footer>
    </Dialog>
  )
}

/** Turning a graph off cancels its pending runs, which is irreversible: it is confirmed, naming them. */
export function DisableDialog(props: { id: string; name: string; onDisabled: () => void; onClose: () => void }): JSX.Element {
  const app = useApp()
  const t = app.t
  const { live, error: loadError, load } = useLive(props.id)
  const [busy, setBusy] = createSignal(false)
  const [error, setError] = createSignal<string>()
  const disable = async () => {
    setBusy(true)
    setError(undefined)
    try {
      const res = await app.call('automations.disable', { id: props.id })
      app.toast.notify({ variant: 'success', title: t('Disable.Done'), message: res.cancelled_runs > 0 ? counted(t, 'Disable.Cancelled', res.cancelled_runs, app.lang) : undefined })
      props.onDisabled()
    } catch (e) {
      setError(errorText(t, e, t('Actions.Disable')))
    } finally {
      setBusy(false)
    }
  }
  return (
    <SafeConfirm
      testid="disable-dialog"
      heading={t('Disable.Title', { name: props.name || t('List.Untitled') })}
      text={<span data-testid="disable-text">{live() === undefined ? t('Common.Loading') : live()!.runs > 0 ? counted(t, 'Disable.Pending', live()!.runs, app.lang) : t('Disable.NoPending')}</span>}
      confirm={t('Disable.Confirm')}
      busy={busy()} ready={live() !== undefined} loadError={loadError()} onRetry={() => void load()} error={error()}
      onConfirm={() => void disable()}
      onClose={props.onClose}
    />
  )
}

/** Deleting a graph cancels its runs and their waits: the confirmation names both. */
export function DeleteDialog(props: { id: string; name: string; onDeleted: () => void; onClose: () => void }): JSX.Element {
  const app = useApp()
  const t = app.t
  const { live, error: loadError, load } = useLive(props.id)
  const [busy, setBusy] = createSignal(false)
  const [error, setError] = createSignal<string>()
  const remove = async () => {
    setBusy(true)
    setError(undefined)
    try {
      await app.call('automations.delete', { id: props.id })
      app.toast.notify({ variant: 'success', title: t('Delete.Done', { name: props.name || t('List.Untitled') }) })
      props.onDeleted()
    } catch (e) {
      setError(errorText(t, e, t('Actions.Delete')))
    } finally {
      setBusy(false)
    }
  }
  const text = () => {
    const l = live()
    if (!l) return t('Common.Loading')
    if (l.runs === 0) return t('Delete.Nothing')
    return t('Delete.Cancels', { runs: counted(t, 'Delete.Runs', l.runs, app.lang), waits: counted(t, 'Delete.Waits', l.waits, app.lang) })
  }
  return (
    <SafeConfirm
      testid="delete-dialog" heading={t('Delete.Title', { name: props.name || t('List.Untitled') })} text={<span data-testid="delete-text">{text()}</span>}
      confirm={t('Delete.Confirm')} busy={busy()} ready={live() !== undefined} loadError={loadError()} onRetry={() => void load()} error={error()}
      onConfirm={() => void remove()} onClose={props.onClose}
    />
  )
}

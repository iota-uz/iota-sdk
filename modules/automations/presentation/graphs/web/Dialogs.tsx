import { For, Match, Show, Switch, createSignal, onMount, type JSX } from 'solid-js'
import { Button, ConfirmationDialog, Dialog, DialogBusyStatus, Skeleton } from '@iota-uz/sdk/solid-ui'
import { useApp } from './context'
import { Icon } from './Icon'
import { counted, errorText, highlight, triggerIcon, type ReviewResult } from './model'

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
  const load = async (changed = false) => {
    setLoadError(undefined)
    try {
      // After a refusal the latest version is reviewed: the one saved meanwhile.
      const r = await app.call('automations.enable_review', { id: props.id, version: changed ? 0 : props.version })
      setReview(r)
      if (changed) setNotice(t('Review.Changed'))
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
                  <p role="alert" class="flex items-start gap-2 text-red-600" data-testid="review-notice"><Icon name="alert" class="mt-0.5" />{notice()}</p>
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
        <menu class="flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
          <Button variant="secondary" class="min-h-11 justify-center" data-dialog-close onClick={props.onClose} aria-disabled={busy() || undefined}>{t('Common.Cancel')}</Button>
          <Button variant="primary" class="min-h-11 justify-center" loading={busy()} disabled={!review()} onClick={() => void enable()} data-testid="review-enable">
            {busy() ? t('Review.Enabling') : t('Review.Enable', { version: review()?.version ?? props.version })}
          </Button>
        </menu>
      </footer>
    </Dialog>
  )
}

/** A confirmation whose default focus is «Cancel» (AL8): the danger action is never one Enter away. */
function SafeConfirm(props: { heading: string; text: JSX.Element; confirm: string; busy: boolean; onConfirm: () => void; onClose: () => void; testid: string; children?: JSX.Element }): JSX.Element {
  const app = useApp()
  let el: HTMLDialogElement | undefined
  return (
    <ConfirmationDialog
      ref={(node: HTMLDialogElement) => { el = node }}
      open
      danger
      busy={props.busy}
      heading={props.heading}
      text={props.text}
      cancelText={app.t('Common.Cancel')}
      confirmText={props.confirm}
      closeLabel={app.t('Common.Close')}
      initialFocus={() => el?.querySelector<HTMLElement>('[data-test-id="dialog-cancel-btn"]') ?? undefined}
      onOpenChange={(open) => { if (!open) props.onClose() }}
      onConfirm={props.onConfirm}
      data-testid={props.testid}
    >
      {props.children}
    </ConfirmationDialog>
  )
}

/** Turning a graph off cancels its pending runs, which is irreversible: it is confirmed, naming them. */
export function DisableDialog(props: { id: string; name: string; onDisabled: () => void; onClose: () => void }): JSX.Element {
  const app = useApp()
  const t = app.t
  const [live, setLive] = createSignal<{ runs: number; waits: number }>()
  const [busy, setBusy] = createSignal(false)
  const [error, setError] = createSignal<string>()
  onMount(async () => {
    try { setLive(await app.call('automations.live', { id: props.id })) } catch { setLive({ runs: 0, waits: 0 }) }
  })
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
      busy={busy()}
      onConfirm={() => void disable()}
      onClose={props.onClose}
    >
      <Show when={error()}><p role="alert" class="text-sm text-red-600">{error()}</p></Show>
    </SafeConfirm>
  )
}

/** Deleting a graph cancels its runs and their waits: the confirmation names both. */
export function DeleteDialog(props: { id: string; name: string; onDeleted: () => void; onClose: () => void }): JSX.Element {
  const app = useApp()
  const t = app.t
  const [live, setLive] = createSignal<{ runs: number; waits: number }>()
  const [busy, setBusy] = createSignal(false)
  const [error, setError] = createSignal<string>()
  onMount(async () => {
    try { setLive(await app.call('automations.live', { id: props.id })) } catch { setLive({ runs: 0, waits: 0 }) }
  })
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
    <SafeConfirm testid="delete-dialog" heading={t('Delete.Title', { name: props.name || t('List.Untitled') })} text={<span data-testid="delete-text">{text()}</span>} confirm={t('Delete.Confirm')} busy={busy()} onConfirm={() => void remove()} onClose={props.onClose}>
      <Show when={error()}><p role="alert" class="text-sm text-red-600">{error()}</p></Show>
    </SafeConfirm>
  )
}

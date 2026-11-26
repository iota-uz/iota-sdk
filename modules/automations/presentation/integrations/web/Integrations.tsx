import { For, Show, createEffect, createMemo, createSignal, on, onCleanup, onMount, untrack, type Component, type JSX } from 'solid-js'
import { FetchRPCTransport, RPCClient, createRPCQuery, useClientHost, type SolidRouteProps } from '@iota-uz/sdk/solid'
import {
  Button, ConfirmationDialog, Dialog, DialogBusyStatus, Dropdown, DropdownItem, EmptyState, Input, PasswordInput, Skeleton,
  Switch, Textarea, ToastProvider, ViewDrawer, useToast,
} from '@iota-uz/sdk/solid-ui'
import { rpcContracts, type Props, type RPC } from './Integrations.solid.generated'
import {
  CATEGORY_KEY, MASKED_VALUE, NAME_RE, rebaseSecretDraft, secretFieldErrorText, secretFormErrors, secretMeta, secretName, secretSaveErrorText, sortSecrets, changedCredentials, oauthErrorText, reasonSentence, reasonText, errorCode, fieldErrorText, groupByCategory, missingRequired, newCredentials, saveErrorText, warningText,
  schemaFields, availablePresets, connectionPreset, knownPresets, presetOptions, slugify, translator, usageHref, usedByText,
  type ConnectionView, type ConnectorView, type FormField, type ListResult, type SecretView, type Status, type Translator, type UsageView,
} from './model'

type Call = <K extends keyof RPC>(method: K, params: RPC[K]['params']) => Promise<RPC[K]['result']>

interface Ctx {
  t: Translator
  lang: string
  call: Call
  automationsPath: string
  canManage: () => boolean
  configured: boolean
  refetch: () => void
  /** The connectors list read from the server, never from the RPC cache (a conflict needs the latest). */
  loadLatest: () => Promise<ListResult>
  highlight: (id: string) => void
  highlighted: () => string | undefined
  openForm: (s: FormState) => void
  openUsage: (c: ConnectionView) => void
  openDelete: (conn: ConnectorView, c: ConnectionView) => void
  setDisabled: (c: ConnectionView, disabled: boolean, undo?: boolean) => Promise<void>
  check: (id: string) => CheckState | undefined
  setCheck: (id: string, s: CheckState | undefined) => void
}

interface CheckState { running: boolean; slow?: boolean; ok?: boolean; text?: string; retry?: boolean }

/** The result of an OAuth2 return the form shows: an error code or the connection it saved. */
interface OAuthReturn { error?: string; connectedId?: string }

type FormState =
  | { mode: 'create'; connector: ConnectorView; oauth?: OAuthReturn; prefill?: { name?: string; displayName?: string } }
  | { mode: 'edit'; connector: ConnectorView; connection: ConnectionView; oauth?: OAuthReturn }

/** Integrations is the screen of /settings/integrations. The host mounts it; the toasts live inside. */
const Integrations: Component<SolidRouteProps<Props>> = (props) => (
  <ToastProvider>
    <Screen {...props} />
  </ToastProvider>
)
export default Integrations

const Screen: Component<SolidRouteProps<Props>> = (props) => {
  const host = useClientHost()
  const toast = useToast()
  const init = props.route.initial
  const lang = host.locale.language
  const t = translator((k) => host.locale.t(k))
  const client = new RPCClient(new FetchRPCTransport(props.route.services?.rpc ?? `${init.basePath}/rpc`, host.session))
  const call: Call = (method, params) => {
    const contract = rpcContracts[method]
    const ctrl = new AbortController()
    return contract.kind === 'query'
      ? client.query(contract, params, ctrl.signal)
      : client.mutate(contract, params, ctrl.signal)
  }
  const query = createRPCQuery<RPC['automations.connectors.list']['params'], ListResult>(client, rpcContracts['automations.connectors.list'], () => ({}))
  // The last list that loaded stays on screen while a refresh runs or fails (S4).
  const [data, setData] = createSignal<ListResult>()
  const [loadedAt, setLoadedAt] = createSignal<Date>()
  createEffect(() => {
    const s = query.state()
    if (s.status === 'success') { setData(s.data); setLoadedAt(new Date()) }
  })
  const refetch = () => { client.cache.invalidate('automations', ['automations.connectors.list']); query.refetch() }

  const [online, setOnline] = createSignal(typeof navigator === 'undefined' ? true : navigator.onLine)
  onMount(() => {
    const up = () => { setOnline(true); refetch() }
    const down = () => setOnline(false)
    window.addEventListener('online', up)
    window.addEventListener('offline', down)
    onCleanup(() => { window.removeEventListener('online', up); window.removeEventListener('offline', down) })
  })

  const [highlighted, setHighlighted] = createSignal<string>()
  let highlightTimer: ReturnType<typeof setTimeout> | undefined
  const highlight = (id: string) => {
    setHighlighted(id)
    clearTimeout(highlightTimer)
    highlightTimer = setTimeout(() => setHighlighted(undefined), 4000)
  }
  onCleanup(() => clearTimeout(highlightTimer))

  const [form, setFormSignal] = createSignal<FormState>()
  // A form session is one drawer for one target: opening the same target again (a history step back to
  // it) keeps its draft; another target starts a new session.
  const target = (s: FormState) => s.mode === 'create' ? `create:${s.connector.kind}` : `edit:${s.connection.id}`
  const setForm = (s: FormState | undefined) => {
    const cur = form()
    if (s && cur && !s.oauth && target(s) === target(cur)) return
    setFormSignal(() => s)
  }
  const [usage, setUsage] = createSignal<ConnectionView>()
  const [deleting, setDeleting] = createSignal<{ connector: ConnectorView; connection: ConnectionView }>()
  const [oauthBanner, setOauthBanner] = createSignal<string>()

  // The open drawer is in the URL (D1): ?connect=<kind> or ?edit=<id>; Back closes it.
  const pushQuery = (key: string | undefined, value?: string) => {
    const url = new URL(window.location.href)
    for (const k of ['connect', 'edit', 'connected', 'oauth_error', 'kind', 'name', 'display_name']) url.searchParams.delete(k)
    if (key && value) url.searchParams.set(key, value)
    if (url.href !== window.location.href) window.history.pushState({}, '', url)
  }
  const openForm = (s: FormState) => {
    setForm(s)
    pushQuery(s.mode === 'create' ? 'connect' : 'edit', s.mode === 'create' ? s.connector.kind : s.connection.id)
  }
  const closeForm = () => { setForm(undefined); pushQuery(undefined) }
  const fromURL = (list: ListResult) => {
    const q = new URLSearchParams(window.location.search)
    const kind = q.get('connect') ?? q.get('kind')
    const edit = q.get('edit')
    const oauthError = q.get('oauth_error')
    const connected = q.get('connected')
    const oauth: OAuthReturn | undefined = oauthError || connected ? { ...(oauthError ? { error: oauthError } : {}), ...(connected ? { connectedId: connected } : {}) } : undefined
    if (connected) { toast.notify({ variant: 'success', title: t('OAuth.Connected') }); highlight(connected) }
    if (edit) {
      for (const c of list.connectors) {
        const conn = c.connections.find((x) => x.id === edit)
        if (conn) { setForm({ mode: 'edit', connector: c, connection: conn, ...(oauth ? { oauth } : {}) }); return }
      }
    }
    const connector = kind ? list.connectors.find((c) => c.kind === kind) : undefined
    if (connector && (q.has('connect') || oauth)) {
      const name = q.get('name') ?? undefined
      const displayName = q.get('display_name') ?? undefined
      setForm({ mode: 'create', connector, ...(oauth ? { oauth } : {}), ...(name || displayName ? { prefill: { ...(name ? { name } : {}), ...(displayName ? { displayName } : {}) } } : {}) })
      return
    }
    // A return the page cannot place (another user's state, a deleted connection) is a banner.
    if (oauthError) setOauthBanner(oauthErrorText(t, oauthError))
    if (!q.has('connect')) setForm(undefined)
  }
  let initialised = false
  createEffect(() => {
    const list = data()
    if (list && !initialised) { initialised = true; fromURL(list) }
  })
  onMount(() => {
    const pop = () => { const list = data(); if (list) fromURL(list) }
    window.addEventListener('popstate', pop)
    onCleanup(() => window.removeEventListener('popstate', pop))
  })

  const setDisabled = async (c: ConnectionView, disabled: boolean, undo = false, confirm?: string[]) => {
    try {
      const r = await call('automations.connections.set_disabled', { id: c.id, disabled, ...(confirm ? { confirm } : {}) })
      if (r.error === 'confirmation_required') {
        // The bot has been given to another service while the connection was off: enabling it again would
        // take its messages away, so the person says so first.
        const codes = r.warnings ?? []
        // What was confirmed on the earlier attempts stays confirmed: the next answer may name one more warning.
        const confirmedNow = [...new Set([...(confirm ?? []), ...codes])]
        toast.notify({
          variant: 'error', title: (codes.length ? codes : ['foreign_webhook']).map((w) => warningText(t, w)).join(' '),
          action: { label: t('Form.ConfirmWarning'), onAction: () => setDisabled(c, disabled, undo, confirmedNow) },
        })
        return
      }
      if (r.error || !r.connection) {
        toast.notify({ variant: 'error', title: t(r.error === 'invalid_input' ? 'Form.BotInUse' : 'Toast.Failed') })
        return
      }
      if (!disabled && r.connection.status !== 'connected') {
        // The provider refused the setup: the connection is saved with its error, not enabled.
        refetch()
        toast.notify({ variant: 'error', title: t('Toast.Failed') })
        return
      }
      highlight(r.connection.id)
      refetch()
      toast.notify({
        variant: 'success',
        title: t(disabled ? 'Toast.Disabled' : 'Toast.Enabled', { name: c.display_name || c.name }),
        ...(undo ? {} : { action: { label: t('Toast.Undo'), onAction: () => setDisabled(c, !disabled, true) } }),
      })
    } catch (e) {
      console.warn('automations: set_disabled failed', errorCode(e))
      toast.notify({ variant: 'error', title: t('Toast.Failed'), action: { label: t('Retry'), onAction: () => setDisabled(c, disabled, undo) } })
    }
  }

  const [checks, setChecks] = createSignal<Record<string, CheckState>>({})
  const ctx: Ctx = {
    check: (id) => checks()[id],
    setCheck: (id, st) => setChecks((cur) => { const next = { ...cur }; if (st) next[id] = st; else delete next[id]; return next }),
    t, lang, call, automationsPath: init.automationsPath, configured: init.configured,
    canManage: () => Boolean(data()?.can_manage),
    refetch, highlight, highlighted, openForm,
    loadLatest: () => {
      client.cache.invalidate('automations', ['automations.connectors.list'])
      return call('automations.connectors.list', {})
    },
    openUsage: (c) => setUsage(c),
    openDelete: (connector, connection) => setDeleting({ connector, connection }),
    setDisabled,
  }

  return (
    <div class="flex flex-col gap-6 p-4 md:p-6 min-w-0" data-testid="integrations">
      <header class="flex flex-col gap-1">
        <h1 class="text-xl font-semibold text-100">{t('Title')}</h1>
        <p class="text-sm text-200">{t('Subtitle')}</p>
      </header>
      <Show when={!online()}>
        <StatusLine tone="error" role="alert" action={<Button size="sm" variant="secondary" onClick={refetch}>{t('Retry')}</Button>}>{t('Offline')}</StatusLine>
      </Show>
      <Show when={!init.configured}>
        <StatusLine tone="warning" role="status">{t('NotConfigured')}</StatusLine>
      </Show>
      <Show when={oauthBanner()}>
        {(text) => <StatusLine tone="error" role="alert" action={<Button size="sm" variant="ghost" onClick={() => setOauthBanner(undefined)}>{t('OAuth.Close')}</Button>}>{text()}</StatusLine>}
      </Show>
      <Show when={query.state().status === 'error' && data()}>
        <StatusLine tone="error" role="alert" action={<Button size="sm" variant="secondary" onClick={refetch}>{t('Retry')}</Button>}>
          {t('LoadFailed')} {loadedAt() ? new Intl.DateTimeFormat(lang, { timeStyle: 'short' }).format(loadedAt()) : ''}
        </StatusLine>
      </Show>
      <Show when={data()} fallback={
        <Show when={query.state().status === 'error'} fallback={<CardsSkeleton />}>
          <div role="alert" class="flex flex-col items-start gap-3 rounded-lg border border-subtle bg-surface-300 p-6" data-testid="integrations-load-error">
            <p class="text-100">{t('LoadFailed')}</p>
            <Button variant="secondary" onClick={refetch}>{t('Retry')}</Button>
          </div>
        </Show>
      }>
        {(list) => (
          <>
            {/* A secrets administrator who does not manage connections sees the secrets only. */}
            <Show when={list().connectors.length > 0} fallback={<Show when={!list().can_manage_secrets}><EmptyState title={t('Empty.Title')} description={t('Empty.Text')} data-testid="integrations-empty" /></Show>}>
              <For each={groupByCategory(list().connectors)}>{(group) => (
                <section class="flex flex-col gap-3" aria-labelledby={`cat-${group.category}`}>
                  <h2 id={`cat-${group.category}`} class="text-base font-medium text-100">{t(`Categories.${CATEGORY_KEY[group.category]}`)}</h2>
                  <div class="grid grid-cols-1 gap-4 md:grid-cols-2 2xl:grid-cols-3">
                    <For each={group.connectors}>{(connector) => <ConnectorCard ctx={ctx} connector={connector} />}</For>
                  </div>
                </section>
              )}</For>
            </Show>
            <Show when={list().can_manage_secrets}><SecretsSection ctx={ctx} writable={list().secrets_configured} /></Show>
          </>
        )}
      </Show>
      {/* Keyed by the form session: another target (history from edit A to edit B) is a new draft with
          its own idempotency key; a save that finishes after its session ended closes nothing. */}
      <Show when={form()} keyed>
        {(s) => <ConnectionForm ctx={ctx} state={s} onClose={closeForm} onSaved={(c, created) => {
          highlight(c.id)
          refetch()
          toast.notify({ variant: 'success', title: t(created ? 'Toast.Saved' : 'Toast.Updated', { name: c.display_name || c.name }) })
          if (form() === s) closeForm()
        }} />}
      </Show>
      <Show when={usage()}>{(c) => <UsageDialog ctx={ctx} connection={c()} onClose={() => setUsage(undefined)} />}</Show>
      <Show when={deleting()}>{(d) => <DeleteDialog ctx={ctx} connector={d().connector} connection={d().connection} onClose={() => setDeleting(undefined)} onDeleted={(name) => {
        setDeleting(undefined)
        refetch()
        toast.notify({ variant: 'success', title: t('Toast.Deleted', { name }) })
      }} />}</Show>
    </div>
  )
}

function StatusLine(props: { tone: 'error' | 'warning' | 'success' | 'info'; role: 'alert' | 'status'; children: JSX.Element; action?: JSX.Element }) {
  const color = () => ({ error: 'text-pink', warning: 'text-yellow', success: 'text-green', info: 'text-blue' })[props.tone]
  return (
    <div role={props.role} class="flex flex-wrap items-center gap-3 border-t border-subtle pt-3 text-sm text-100">
      <span class={`inline-flex h-2 w-2 shrink-0 rounded-full bg-current ${color()}`} aria-hidden="true" />
      <span class="min-w-0 flex-1">{props.children}</span>
      {props.action}
    </div>
  )
}

function Spinner() {
  return (
    <svg aria-hidden="true" width="14" height="14" viewBox="0 0 24 24" fill="none" class="animate-spin motion-reduce:animate-none">
      <circle cx="12" cy="12" r="9" stroke="currentColor" stroke-opacity="0.25" stroke-width="3" />
      <path d="M21 12a9 9 0 0 0-9-9" stroke="currentColor" stroke-width="3" stroke-linecap="round" />
    </svg>
  )
}

function CardsSkeleton() {
  return (
    <div class="flex flex-col gap-3" aria-busy="true" data-testid="integrations-loading">
      <Skeleton variant="text" lines={1} class="w-32" />
      <div class="grid grid-cols-1 gap-4 md:grid-cols-2 2xl:grid-cols-3">
        <For each={[0, 1, 2, 3]}>{() => (
          <div class="flex h-56 flex-col gap-4 rounded-lg border border-subtle bg-surface-300 p-4">
            <Skeleton variant="card" />
            <Skeleton variant="text" lines={3} />
          </div>
        )}</For>
      </div>
    </div>
  )
}

function ConnectorIcon(props: { connector: ConnectorView }) {
  const isURL = () => /^(https?:|\/|data:image\/)/.test(props.connector.icon)
  return (
    <Show when={isURL()} fallback={
      <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-surface-500 text-base font-semibold text-100" aria-hidden="true">
        {(props.connector.title || props.connector.kind).slice(0, 1).toUpperCase()}
      </span>
    }>
      <img src={props.connector.icon} alt="" class="h-10 w-10 shrink-0 rounded-lg object-contain" />
    </Show>
  )
}

const STATUS_TONE: Record<Status, string> = { connected: 'text-green', needs_reauth: 'text-yellow', error: 'text-pink', disabled: 'text-200' }

/** A status in words with its own glyph, never colour alone (A5). */
function StatusBadge(props: { status: Status; t: Translator }) {
  return (
    <span class={`inline-flex items-center gap-1.5 text-sm font-medium ${STATUS_TONE[props.status]}`} data-status={props.status}>
      <svg aria-hidden="true" width="16" height="16" viewBox="0 0 256 256" fill="none" stroke="currentColor" stroke-width="20" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="128" cy="128" r="96" />
        {props.status === 'connected' && <polyline points="88 136 112 160 168 104" />}
        {props.status === 'needs_reauth' && <><line x1="128" y1="80" x2="128" y2="136" /><circle cx="128" cy="172" r="4" fill="currentColor" /></>}
        {props.status === 'error' && <><line x1="160" y1="96" x2="96" y2="160" /><line x1="96" y1="96" x2="160" y2="160" /></>}
        {props.status === 'disabled' && <><line x1="108" y1="96" x2="108" y2="160" /><line x1="148" y1="96" x2="148" y2="160" /></>}
      </svg>
      {props.t(`Status.${props.status}`)}
    </span>
  )
}

function ConnectorCard(props: { ctx: Ctx; connector: ConnectorView }) {
  const t = props.ctx.t
  const c = () => props.connector
  const platform = () => c().platform
  return (
    <article class="flex min-w-0 flex-col gap-4 rounded-lg border border-subtle bg-surface-300 p-4" data-testid={`connector-${c().kind}`} aria-labelledby={`connector-title-${c().kind}`}>
      <div class="flex items-start gap-3">
        <ConnectorIcon connector={c()} />
        <div class="flex min-w-0 flex-1 flex-col gap-1">
          <h3 id={`connector-title-${c().kind}`} class="truncate font-medium text-100" title={c().title}>{c().title}</h3>
          <p class="text-xs text-200">{t(`Auth.${c().auth}`)}</p>
        </div>
      </div>
      <Show when={c().description}><p class="text-sm text-200">{c().description}</p></Show>
      <Show when={platform()?.available}>
        <PlatformRow ctx={props.ctx} connector={c()} />
      </Show>
      <div class="flex flex-col gap-2">
        <Show when={c().connections.length > 0} fallback={
          <p class="text-sm text-200" data-testid="no-connections">
            {t('NoConnections')}{c().nodes.length > 0 ? ' ' + t('NodesUnlock', { nodes: c().nodes.join(', ') }) : ''}
          </p>
        }>
          <ul class="flex flex-col" aria-label={c().title}>
            <For each={c().connections}>{(conn) => <ConnectionRow ctx={props.ctx} connector={c()} connection={conn} />}</For>
          </ul>
        </Show>
      </div>
      <Show when={props.ctx.canManage() && props.ctx.configured}>
        <div class="mt-auto flex flex-col gap-2">
          <div class="flex flex-wrap gap-2">
            <Button variant={c().connections.length > 0 ? 'secondary' : 'primary'} size="sm" class="min-h-11 sm:min-h-9"
              onClick={() => props.ctx.openForm({ mode: 'create', connector: c() })} data-testid={`connect-${c().kind}`}>
              {c().connections.length > 0 ? t('ConnectAnother') : t('Connect')}
            </Button>
            <For each={availablePresets(c())}>{(preset) => <PresetSignIn ctx={props.ctx} connector={c()} preset={preset} />}</For>
          </div>
        </div>
      </Show>
    </article>
  )
}

/**
 * A provider's sign-in straight from the card of a connector that has a form and a sign-in (an email box):
 * «Sign in with Google», «Sign in with Microsoft 365». It names no connection: the server signs the same
 * account in again or names a new pair itself, and the provider sends the browser back to the page.
 */
function PresetSignIn(props: { ctx: Ctx; connector: ConnectorView; preset: string }) {
  const t = props.ctx.t
  const toast = useToast()
  const [busy, setBusy] = createSignal(false)
  const provider = () => t(`Form.PresetProvider.${props.preset}`)
  const start = async () => {
    if (busy()) return
    setBusy(true)
    try {
      const r = await props.ctx.call('automations.connections.oauth_start', { kind: props.connector.kind, name: '', preset: props.preset })
      window.location.assign(r.authorize_url)
    } catch (e) {
      setBusy(false)
      const code = errorCode(e)
      console.warn('automations: oauth_start failed', code)
      toast.notify({ variant: 'error', title: code === 'unavailable' ? t('OAuth.unavailable') : t('Toast.Failed') })
    }
  }
  return (
    <Button variant="secondary" size="sm" class="min-h-11 sm:min-h-9" loading={busy()} aria-disabled={busy() || undefined}
      onClick={() => void start()} data-testid={`signin-${props.connector.kind}-${props.preset}`}>
      {busy() ? t('Form.Redirecting', { connector: provider() }) : t('Form.SignInWith', { connector: provider() })}
    </Button>
  )
}

function PlatformRow(props: { ctx: Ctx; connector: ConnectorView }) {
  const t = props.ctx.t
  const p = () => props.connector.platform!
  // Optimistic: the switch moves at once and returns when the server refuses (F8). One request runs at a
  // time (the switch waits for it), so requests cannot reach the server out of order, and a refusal
  // returns the switch to the last value the server accepted, not to the inverse of the request.
  const [saved, setSaved] = createSignal(p().enabled_for_tenant)
  const [enabled, setEnabled] = createSignal(p().enabled_for_tenant)
  const [pending, setPending] = createSignal(false)
  const [error, setError] = createSignal<string>()
  // Only fresh list data moves the saved value (a new platform object from a refresh), never the end of
  // a request: after a success the accepted value stays even when the following refresh fails.
  createEffect(on(p, (platform) => { if (!untrack(pending)) { setSaved(platform.enabled_for_tenant); setEnabled(platform.enabled_for_tenant) } }, { defer: true }))
  const toggle = async (next: boolean) => {
    if (pending()) return
    setPending(true)
    setEnabled(next)
    setError(undefined)
    try {
      await props.ctx.call('automations.platform.set_enabled', { kind: props.connector.kind, enabled: next })
      setSaved(next)
      props.ctx.refetch()
    } catch (e) {
      console.warn('automations: platform.set_enabled failed', errorCode(e))
      setEnabled(saved())
      setError(t('Platform.Failed', { reason: t('Toast.Failed') }))
    } finally {
      setPending(false)
    }
  }
  return (
    <div class="flex flex-col gap-2 rounded-md border border-subtle p-3 text-sm" data-testid="platform-key">
      <div class="flex flex-wrap items-center gap-2">
        <span class="font-medium text-100">{t('Platform.Label')}</span>
        <span class="text-200">· {t('Platform.Available')}</span>
      </div>
      <Show when={p().policy === 'opt_in' && props.ctx.canManage()}>
        <Switch size="sm" checked={enabled()} disabled={pending()} aria-busy={pending()} label={t('Platform.Use')} onChange={(e) => void toggle(e.currentTarget.checked)} data-testid="platform-switch" />
      </Show>
      <Show when={error()}><p role="alert" class="text-pink">{error()}</p></Show>
      <p class="text-xs text-200">{t('Platform.Override')}</p>
    </div>
  )
}

function ConnectionRow(props: { ctx: Ctx; connector: ConnectorView; connection: ConnectionView }) {
  const t = props.ctx.t
  const c = () => props.connection
  const check = () => props.ctx.check(c().id)
  const testing = () => Boolean(check()?.running)
  const setResult = (r: { ok: boolean; text: string } | undefined) => props.ctx.setCheck(c().id, r ? { running: false, ...r } : undefined)
  const [menuOpen, setMenuOpen] = createSignal(false)
  // F1–F3: one request at a time; «still waiting» after 4 s; a failure in words with «Retry» (a check
  // changes nothing at the provider, so repeating it is safe).
  const runTest = async () => {
    if (testing()) return
    setMenuOpen(false)
    const id = c().id
    props.ctx.setCheck(id, { running: true })
    const slow = setTimeout(() => { const st = props.ctx.check(id); if (st?.running) props.ctx.setCheck(id, { ...st, slow: true }) }, 4000)
    const done = (st: Omit<CheckState, 'running'>) => { clearTimeout(slow); props.ctx.setCheck(id, { running: false, ...st }) }
    try {
      const r = await props.ctx.call('automations.connections.test', { id })
      if (r.ok) done({ ok: true, text: r.identity ? t('Test.Connected', { identity: r.identity }) : t('Test.ConnectedPlain') })
      else if (r.error === 'needs_reauth') done({ ok: false, text: t('Test.Reauth') })
      else done({ ok: false, text: t('Test.Failed', { message: reasonText(t, r.reason) }) })
      props.ctx.refetch()
    } catch (e) {
      console.warn('automations: test failed', errorCode(e))
      done({ ok: false, text: t('Test.Error'), retry: true })
    }
  }
  const reauth = async () => {
    try {
      const r = await props.ctx.call('automations.connections.oauth_start', { kind: props.connector.kind, name: c().name, connection_id: c().id, ...presetParam(connectionPreset(props.connector, c())) })
      window.location.assign(r.authorize_url)
    } catch (e) {
      console.warn('automations: oauth_start failed', errorCode(e))
      setResult({ ok: false, text: errorCode(e) === 'unavailable' ? t('OAuth.unavailable') : t('Toast.Failed') })
    }
  }
  const inUse = () => c().used_by > 0
  const reasonID = () => `delete-reason-${c().id}`
  return (
    <li class={`flex flex-col gap-1 py-3 first:pt-0 ${props.ctx.highlighted() === c().id ? 'rounded-md outline outline-2 outline-offset-4 outline-brand-500' : ''}`} data-testid={`connection-${c().name}`}>
      <div class="flex items-start gap-2">
        <div class="flex min-w-0 flex-1 flex-col gap-1">
          <span class="truncate font-medium text-100" title={c().display_name || c().name}>{c().display_name || c().name}</span>
          <Show when={c().identity}><span class="truncate text-sm text-200" title={c().identity!}>{c().identity}</span></Show>
          <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
            <StatusBadge status={c().status} t={t} />
            <Show when={props.ctx.canManage()} fallback={<span class="text-sm text-200">{usedByText(t, c().used_by, props.ctx.lang)}</span>}>
              <button type="button" class="min-h-11 text-sm text-200 underline-offset-2 hover:underline sm:min-h-0" onClick={() => props.ctx.openUsage(c())} data-testid="used-by">
                {usedByText(t, c().used_by, props.ctx.lang)}
              </button>
            </Show>
          </div>
          <Show when={c().status_reason && c().status !== 'connected' && c().status !== 'disabled'}>
            <p class="text-sm text-200" data-testid="status-reason">{reasonSentence(t, c().status_reason)}</p>
          </Show>
        </div>
        <Show when={props.ctx.canManage()}>
          <Dropdown open={menuOpen()} onOpenChange={setMenuOpen} class={menuOpen() ? 'z-30' : undefined} label={t('Actions', { name: c().display_name || c().name })} menuClass="min-w-56"
            trigger={<span class="flex h-11 w-11 items-center justify-center rounded-md text-200 sm:h-9 sm:w-9" aria-hidden="true">
              <svg width="20" height="20" viewBox="0 0 256 256" fill="currentColor"><circle cx="128" cy="64" r="16" /><circle cx="128" cy="128" r="16" /><circle cx="128" cy="192" r="16" /></svg>
            </span>}>
            <DropdownItem onClick={() => void runTest()} disabled={testing()} data-testid="menu-test">{t('Menu.Test')}</DropdownItem>
            <DropdownItem onClick={() => { setMenuOpen(false); props.ctx.openForm({ mode: 'edit', connector: props.connector, connection: c() }) }} data-testid="menu-edit">{t('Menu.Edit')}</DropdownItem>
            <DropdownItem onClick={() => { setMenuOpen(false); void props.ctx.setDisabled(c(), c().status !== 'disabled') }} data-testid="menu-toggle">
              {c().status === 'disabled' ? t('Menu.Enable') : t('Menu.Disable')}
            </DropdownItem>
            <DropdownItem onClick={() => { setMenuOpen(false); props.ctx.openDelete(props.connector, c()) }} disabled={inUse()} aria-describedby={inUse() ? reasonID() : undefined} class="text-pink disabled:cursor-not-allowed disabled:text-200" data-testid="menu-delete">
              {t('Menu.Delete')}
            </DropdownItem>
            <Show when={inUse()}><li role="none" id={reasonID()} class="px-2 pb-2 text-xs text-200">{t('DeleteBlocked')}</li></Show>
          </Dropdown>
        </Show>
      </div>
      <Show when={c().status === 'needs_reauth' && (props.connector.auth === 'oauth2' || c().sign_in || connectionPreset(props.connector, c()) !== undefined) && props.ctx.canManage()}>
        <div><Button size="sm" onClick={() => void reauth()} class="min-h-11 sm:min-h-9" data-testid="reauth">{t('SignInAgain')}</Button></div>
      </Show>
      <div role="status" aria-live="polite" class="flex min-h-5 flex-col gap-1 text-sm" data-testid="check-status">
        <Show when={testing()}>
          <span class="inline-flex items-center gap-2 text-200"><Spinner />{t('Test.Running')}</span>
          <Show when={check()?.slow}><span class="text-200" data-testid="test-slow">{t('Test.Slow')}</span></Show>
        </Show>
        <Show when={!testing() && check()}>{(st) => (
          <>
            <span class={st().ok ? 'text-green' : 'text-pink'} data-testid="test-result">{st().text}</span>
            <Show when={st().retry}>
              <div><Button size="sm" variant="secondary" class="min-h-11 sm:min-h-9" onClick={() => void runTest()} data-testid="test-retry">{t('Retry')}</Button></div>
            </Show>
          </>
        )}</Show>
      </div>
    </li>
  )
}

function ConnectionForm(props: { ctx: Ctx; state: FormState; onClose: () => void; onSaved: (c: ConnectionView, created: boolean) => void }) {
  const t = props.ctx.t
  const connector = () => props.state.connector
  // The version the edit applies to: the one the drawer opened with, or the latest one the user loaded
  // after a version conflict.
  const [base, setBase] = createSignal(props.state.mode === 'edit' ? props.state.connection : undefined)
  const editing = () => base()
  const fields = createMemo(() => schemaFields(connector().credential_schema))
  const initial = (): Record<string, string> => ({ ...(editing()?.fields ?? {}) })
  const [values, setValues] = createSignal<Record<string, string>>(initial())
  const [replaced, setReplaced] = createSignal<Set<string>>(new Set())
  const [shown, setShown] = createSignal<Set<string>>(new Set())
  const prefill = () => (props.state.mode === 'create' ? props.state.prefill : undefined)
  const [displayName, setDisplayName] = createSignal(editing()?.display_name ?? prefill()?.displayName ?? '')
  const [name, setName] = createSignal(editing()?.name ?? prefill()?.name ?? '')
  const [nameTouched, setNameTouched] = createSignal(Boolean(editing() || prefill()?.name))
  // One key per intent (this form): «Retry» after an ambiguous failure resends the same key, so the
  // server answers with what the first request did instead of doing it twice (F3).
  const newKey = () => typeof crypto !== 'undefined' && 'randomUUID' in crypto ? crypto.randomUUID() : `${Date.now()}-${Math.random()}`
  let idempotencyKey = newKey()
  const [retry, setRetry] = createSignal(false)
  const [conflict, setConflict] = createSignal(false)
  const [notice, setNotice] = createSignal<string>()
  const [fieldErrors, setFieldErrors] = createSignal<Record<string, string>>({})
  const [error, setError] = createSignal<string>()
  // Warnings the connector raised for these credentials (a bot another service uses): the save goes
  // through only once the person confirmed them.
  const [warnings, setWarnings] = createSignal<string[]>([])
  const [confirmed, setConfirmed] = createSignal(false)
  // Warnings the person confirmed on earlier attempts of this draft: a later answer may name one more, and the
  // ones already confirmed are sent with it, so the two never alternate.
  const [accepted, setAccepted] = createSignal<string[]>([])
  const [busy, setBusy] = createSignal(false)
  const [slow, setSlow] = createSignal(false)
  let slowTimer: ReturnType<typeof setTimeout> | undefined
  onCleanup(() => clearTimeout(slowTimer))
  // A connector with a form and a sign-in (an email box) chooses by its preset field: a provider's sign-in,
  // or the form. Only the providers the application can sign in with are offered.
  const preset = () => (editing() ? connectionPreset(connector(), editing()) ?? 'custom' : values().preset || 'custom')
  const presetSignIn = () => (editing() ? connectionPreset(connector(), editing()) !== undefined : availablePresets(connector()).includes(preset()))
  const oauth = () => connector().auth === 'oauth2' || presetSignIn()
  const oauthState = () => props.state.oauth
  // Editing an OAuth2 connection changes its name only; signing in again is its own action.
  const signIn = () => oauth() && !editing()
  const providerTitle = () => (connector().auth === 'oauth2' ? connector().title : t(`Form.PresetProvider.${preset()}`))

  createEffect(() => { if (!nameTouched() && !editing()) setName(slugify(displayName(), connector().kind)) })
  const secretSet = (f: string) => Boolean(editing()) && !replaced().has(f)
  // New credentials are another bot: what was confirmed for the earlier ones does not carry over.
  const setValue = (k: string, v: string) => {
    setValues((cur) => ({ ...cur, [k]: v })); setWarnings([]); setAccepted([]); setConfirmed(false)
    clearConfirmError() // the confirmation asked for was about the earlier credentials, not these
  }
  // Only the "confirm first" message goes when the credentials or the switch change: a version conflict (and its
  // «Load latest») or another failure of the last save stays until the next save.
  const clearConfirmError = () => { if (error() === t('Form.ConfirmRequired')) setError(undefined) }
  const confirmList = () => [...new Set([...accepted(), ...warnings()])]

  const begin = () => { setBusy(true); setSlow(false); setError(undefined); setNotice(undefined); setRetry(false); setConflict(false); slowTimer = setTimeout(() => setSlow(true), 4000) }
  const end = () => { setBusy(false); setSlow(false); clearTimeout(slowTimer) }

  const submit = async (event: Event) => {
    event.preventDefault()
    if (busy()) return // one request at a time (F1)
    const local: Record<string, string> = {}
    if (!editing() && !NAME_RE.test(name())) local.name = 'name'
    if (!oauth()) Object.assign(local, missingRequired(fields(), values(), secretSet))
    setFieldErrors(local)
    if (Object.keys(local).length > 0) { focusFirstError(local); return }
    if (warnings().length > 0 && !confirmed()) {
      setError(t('Form.ConfirmRequired'))
      queueMicrotask(() => formRef?.querySelector<HTMLElement>('[data-field="confirm_warnings"]')?.focus())
      return
    }
    begin()
    try {
      if (signIn()) {
        const r = await props.ctx.call('automations.connections.oauth_start', { kind: connector().kind, name: name(), display_name: displayName(), ...presetParam(connector().auth === 'oauth2' ? undefined : preset()) })
        window.location.assign(r.authorize_url)
        return
      }
      const e = editing()
      const creds = e && !oauth() ? changedCredentials(fields(), values(), initial(), replaced()) : undefined
      const result = e
        ? await props.ctx.call('automations.connections.update', {
          id: e.id, version: e.version, display_name: displayName(), idempotency_key: idempotencyKey, ...(creds ? { credentials: creds } : {}),
          ...(confirmed() ? { confirm: confirmList() } : {}),
        })
        : await props.ctx.call('automations.connections.create', {
          kind: connector().kind, name: name(), display_name: displayName(), credentials: newCredentials(fields(), values()), idempotency_key: idempotencyKey,
          ...(confirmed() ? { confirm: confirmList() } : {}),
        })
      if (result.ok && result.connection) {
        const saved = result.connection
        if (saved.sign_in && saved.status === 'needs_reauth') {
          // The connection signs in with its own authorization server (an MCP server with OAuth): saved
          // first, then the browser goes to the sign-in; the callback brings it back connected.
          const r = await props.ctx.call('automations.connections.oauth_start', { kind: connector().kind, name: saved.name, connection_id: saved.id })
          window.location.assign(r.authorize_url)
          return
        }
        end(); props.onSaved(saved, !e); return
      }
      end()
      if (result.error === 'confirmation_required') {
        // The request did nothing: ask again for the same intent, with the warnings confirmed.
        if (confirmed()) setAccepted(confirmList())
        setWarnings(result.warnings ?? [])
        setConfirmed(false)
        setError(undefined)
        queueMicrotask(() => formRef?.querySelector<HTMLElement>('[data-field="confirm_warnings"]')?.focus())
        return
      }
      const { idempotency_key: changedRetry, ...fieldsLeft } = result.fields ?? {}
      setFieldErrors(fieldsLeft)
      if (changedRetry) {
        // The key belongs to the first payload, whose answer was lost, and the draft changed since: the
        // first save may have gone through. The list shows what was saved; the edit goes through it.
        setError(t('Form.RetryChanged'))
        props.ctx.refetch()
        return
      }
      setError(saveErrorText(t, result))
      if (result.error === 'in_progress') setRetry(true) // the first request is still running: retry with the same key
      if (result.error === 'version_conflict') setConflict(true)
      if (result.fields) focusFirstError(fieldsLeft)
    } catch (e) {
      end()
      const code = errorCode(e)
      console.warn('automations: save failed', code)
      if (code === 'unavailable') setError(t(oauth() ? 'OAuth.unavailable' : 'Form.Unavailable'))
      else if (code === 'version_conflict') { setError(t('Form.VersionConflict')); setConflict(true) }
      else { setError(t('Form.Failed')); setRetry(true) } // the answer was lost: «Retry» with the same key
    }
  }
  // After a version conflict the user loads the latest version explicitly: what they did not change
  // follows it, what they typed stays, and the next save (a new intent, a new key) applies the draft to it.
  const reload = async () => {
    const e = editing()
    if (!e || busy()) return
    begin()
    try {
      // A conflict that came as an RPC error did not invalidate the cached list: read past the cache.
      const list = await props.ctx.loadLatest()
      end()
      const latest = list.connectors.flatMap((c) => c.connections).find((x) => x.id === e.id)
      if (!latest) { setError(t('Form.Failed')); return }
      const before = e.fields ?? {}
      const after = latest.fields ?? {}
      setValues((cur) => {
        const next = { ...cur }
        for (const k of new Set([...Object.keys(before), ...Object.keys(after)])) {
          if ((cur[k] ?? '') !== (before[k] ?? '')) continue
          if (k in after) next[k] = after[k]!
          else delete next[k]
        }
        return next
      })
      if (displayName() === (e.display_name ?? '')) setDisplayName(latest.display_name ?? '')
      setBase(latest)
      idempotencyKey = newKey()
      setNotice(t('Form.Reloaded'))
      props.ctx.refetch()
    } catch (err) {
      end()
      console.warn('automations: reload failed', errorCode(err))
      setError(t('Form.Failed'))
      setConflict(true)
    }
  }
  const reauth = async () => {
    const e = editing()
    if (!e || busy()) return
    begin()
    try {
      const r = await props.ctx.call('automations.connections.oauth_start', { kind: connector().kind, name: e.name, connection_id: e.id, ...presetParam(connectionPreset(connector(), e)) })
      window.location.assign(r.authorize_url)
    } catch (err) {
      end()
      console.warn('automations: oauth_start failed', errorCode(err))
      setError(errorCode(err) === 'unavailable' ? t('OAuth.unavailable') : t('Form.Failed'))
    }
  }
  let formRef: HTMLFormElement | undefined
  const focusFirstError = (errs: Record<string, string>) => {
    queueMicrotask(() => {
      const first = Object.keys(errs)[0]
      const el = first ? formRef?.querySelector<HTMLElement>(`[data-field="${CSS.escape(first)}"]`) : undefined
      el?.focus()
    })
  }

  const title = () => (editing() ? t('Form.TitleEdit', { name: editing()!.display_name || editing()!.name }) : t('Form.TitleNew', { connector: connector().title }))
  return (
    <ViewDrawer open onOpenChange={(open) => { if (!open) props.onClose() }} busy={busy()} title={title()} closeLabel={t('Close')} data-testid="connection-form">
      <form ref={formRef} class="flex flex-col gap-4 p-4 md:p-6" onSubmit={(e) => void submit(e)} novalidate>
        <Show when={oauthState()?.connectedId}>
          <p role="status" class="text-sm text-green">{t('OAuth.Connected')}</p>
        </Show>
        <Show when={oauthState()?.error}>{(code) => (
          <p role="alert" class="text-sm text-pink" data-testid="oauth-error">{oauthErrorText(t, code())}</p>
        )}</Show>
        <Input label={t('Form.DisplayName')} value={displayName()} onInput={(e) => setDisplayName(e.currentTarget.value)} description={t('Form.DisplayNameHint')}
          data-field="display_name" error={fieldErrors().display_name ? fieldErrorText(t, fieldErrors().display_name!) : undefined} disabled={busy()} autocomplete="off" />
        <Input label={t('Form.Name')} value={name()} onInput={(e) => { setNameTouched(true); setName(e.currentTarget.value) }} description={t('Form.NameHint')}
          data-field="name" error={fieldErrors().name ? fieldErrorText(t, fieldErrors().name!) : undefined} disabled={busy() || Boolean(editing())}
          required={!editing()} autocomplete="off" spellcheck={false} />
        <Show when={!oauth()} fallback={
          <Show when={editing()} fallback={<>
            <Show when={connector().auth !== 'oauth2'}><PresetField t={t} connector={connector()} field={fields().find((f) => f.name === 'preset')} value={preset()} onInput={(v) => setValue('preset', v)} disabled={busy()} /></Show>
            <p class="text-sm text-200">{t('Form.OAuthHint', { connector: providerTitle() })}</p>
          </>}>
            <div class="flex flex-col items-start gap-2">
              <p class="text-sm text-200">{t('Form.OAuthHint', { connector: providerTitle() })}</p>
              <Button type="button" variant="secondary" class="min-h-11 sm:min-h-9" onClick={() => void reauth()} disabled={busy()} data-testid="form-reauth">{t('SignInAgain')}</Button>
            </div>
          </Show>
        }>
          <For each={fields()}>{(f) => <Show when={f.name !== 'preset' || knownPresets(connector()).length === 0} fallback={
            // A saved connection with a login and a password stays one: it is not turned into a provider's sign-in
            // from here (an edit saves credentials, it does not start a sign-in), so the choice is for a new one.
            <Show when={!editing()}><PresetField t={t} connector={connector()} field={f} value={preset()} onInput={(v) => setValue('preset', v)} disabled={busy()} /></Show>}>
            <CredentialField t={t} field={f} value={values()[f.name] ?? ''} onInput={(v) => setValue(f.name, v)}
            error={fieldErrors()[f.name] ? fieldErrorText(t, fieldErrors()[f.name]!) : undefined} disabled={busy()}
            secretSet={secretSet(f.name)} shown={shown().has(f.name)}
            onShow={(s) => setShown((cur) => { const n = new Set(cur); if (s) n.add(f.name); else n.delete(f.name); return n })}
            onReplace={(r) => setReplaced((cur) => { const n = new Set(cur); if (r) n.add(f.name); else { n.delete(f.name); setValue(f.name, '') } return n })} /></Show>}</For>
        </Show>
        <Show when={warnings().length > 0}>
          <div class="flex flex-col items-start gap-2 rounded-md border border-subtle p-3" role="group" data-testid="form-warnings">
            <For each={warnings()}>{(code) => <p role="alert" class="text-sm text-pink">{warningText(t, code)}</p>}</For>
            <Switch checked={confirmed()} label={t('Form.ConfirmWarning')} onChange={(e) => { setConfirmed(e.currentTarget.checked); clearConfirmError() }}
              disabled={busy()} data-field="confirm_warnings" data-testid="form-confirm" />
          </div>
        </Show>
        <Show when={error()}>
          <div class="flex flex-col items-start gap-2">
            <p role="alert" class="text-sm text-pink" data-testid="form-error">{error()}</p>
            <Show when={conflict() && editing()}>
              <Button type="button" size="sm" variant="secondary" class="min-h-11 sm:min-h-9" onClick={() => void reload()} disabled={busy()} data-testid="form-reload">{t('Form.LoadLatest')}</Button>
            </Show>
          </div>
        </Show>
        <Show when={notice()}><p role="status" class="text-sm text-200" data-testid="form-notice">{notice()}</p></Show>
        <Show when={slow()}><p role="status" class="text-sm text-200">{t('Form.StillWaiting')}</p></Show>
        <div class="sticky bottom-0 flex flex-col-reverse gap-2 border-t border-subtle bg-surface-300 pt-4 sm:flex-row sm:justify-end">
          <Button type="button" variant="secondary" class="min-h-11 justify-center" data-dialog-close aria-disabled={busy() || undefined}
            onClick={() => { if (!busy()) props.onClose() }}>{t('Form.Cancel')}</Button>
          <Button type="submit" class="min-h-11 justify-center" loading={busy()} aria-disabled={busy() || undefined} data-testid="form-save">
            {busy() ? (signIn() ? t('Form.Redirecting', { connector: providerTitle() }) : t('Form.Saving'))
              : signIn() ? t('Form.SignInWith', { connector: providerTitle() }) : retry() ? t('Retry') : t('Form.Save')}
          </Button>
        </div>
      </form>
    </ViewDrawer>
  )
}

/** The preset choice of a connector with a form and a sign-in: the form, and the providers that can be signed in with. */
function PresetField(props: { t: Translator; connector: ConnectorView; field: FormField | undefined; value: string; onInput: (v: string) => void; disabled: boolean }) {
  const options = () => presetOptions(props.connector, props.field?.options)
  return (
    <Show when={props.field && options().length > 1}>
      <label class="flex flex-col gap-2 text-sm">
        <span class="form-control-label">{props.field!.title}</span>
        <select class="form-control h-11 rounded-md border border-default bg-surface-300 px-3 text-100" value={props.value} disabled={props.disabled}
          onChange={(e) => props.onInput(e.currentTarget.value)} data-field="preset" data-testid="preset">
          <For each={options()}>{(o) => <option value={o} selected={o === props.value}>{props.t(`Form.Preset.${o}`)}</option>}</For>
        </select>
      </label>
    </Show>
  )
}

/** The preset a sign-in is for, as an RPC param (a connector whose own auth is OAuth2 has none). */
function presetParam(preset: string | undefined): { preset?: string } {
  return preset && preset !== 'custom' ? { preset } : {}
}

function CredentialField(props: {
  t: Translator; field: FormField; value: string; onInput: (v: string) => void; error: string | undefined; disabled: boolean
  secretSet: boolean; shown: boolean; onShow: (shown: boolean) => void; onReplace: (replace: boolean) => void
}) {
  const t = props.t
  const f = () => props.field
  return (
    <Show when={f().kind === 'secret'} fallback={
      <Show when={f().kind === 'boolean'} fallback={
        <Show when={f().kind === 'enum'} fallback={
          <Show when={f().kind === 'textarea'} fallback={
          <Input label={f().title} description={f().description} type={f().kind === 'number' ? 'number' : 'text'} value={props.value}
            onInput={(e) => props.onInput(e.currentTarget.value)} error={props.error} disabled={props.disabled} required={f().required}
            placeholder={f().placeholder} data-field={f().name} autocomplete="off" spellcheck={false} />
          }>
            <Textarea label={f().title} description={f().description} value={props.value} rows={4}
              onInput={(e) => props.onInput(e.currentTarget.value)} error={props.error} disabled={props.disabled} required={f().required}
              placeholder={f().placeholder} data-field={f().name} autocomplete="off" spellcheck={false} />
          </Show>
        }>
          <label class="flex flex-col gap-2 text-sm">
            <span class="form-control-label">{f().title}{f().required ? ' *' : ''}</span>
            <select class="form-control h-11 rounded-md border border-default bg-surface-300 px-3 text-100" value={props.value} disabled={props.disabled}
              onChange={(e) => props.onInput(e.currentTarget.value)} data-field={f().name} aria-invalid={props.error ? true : undefined}>
              <option value="" />
              <For each={f().options}>{(o) => <option value={o}>{o}</option>}</For>
            </select>
            <Show when={props.error}><span role="alert" class="text-pink">{props.error}</span></Show>
          </label>
        </Show>
      }>
        <Switch checked={props.value === 'true'} label={f().title} onChange={(e) => props.onInput(e.currentTarget.checked ? 'true' : 'false')} disabled={props.disabled} data-field={f().name} />
      </Show>
    }>
      <Show when={!props.secretSet} fallback={
        <div class="flex flex-col gap-2">
          <span class="form-control-label">{f().title}</span>
          <div class="flex flex-wrap items-center gap-3">
            <span class="text-sm text-200" data-testid={`secret-set-${f().name}`}>{t('Form.SecretSet')}</span>
            <Button type="button" size="sm" variant="secondary" class="min-h-11 sm:min-h-9" onClick={() => props.onReplace(true)} disabled={props.disabled} data-field={f().name}>{t('Form.Replace')}</Button>
          </div>
        </div>
      }>
        <div class="flex flex-col gap-1">
          <Input label={f().title} description={f().description} type={props.shown ? 'text' : 'password'} value={props.value}
            onInput={(e) => props.onInput(e.currentTarget.value)} error={props.error} disabled={props.disabled} required={f().required}
            data-field={f().name} autocomplete="new-password" spellcheck={false}
            addonRight={<button type="button" class="mx-2 min-h-11 px-2 text-sm text-200 sm:min-h-0" aria-pressed={props.shown} onClick={() => props.onShow(!props.shown)}>
              {props.shown ? t('Form.Hide') : t('Form.Show')}
            </button>} />
        </div>
      </Show>
    </Show>
  )
}

function UsageDialog(props: { ctx: Ctx; connection: ConnectionView; onClose: () => void }) {
  const t = props.ctx.t
  const [usages, setUsages] = createSignal<UsageView[]>()
  const [failed, setFailed] = createSignal(false)
  const load = async () => {
    setFailed(false)
    try { setUsages((await props.ctx.call('automations.connections.usage', { id: props.connection.id })).usages) } catch (e) { console.warn('automations: usage failed', errorCode(e)); setFailed(true) }
  }
  void load()
  return (
    <Dialog open onOpenChange={(open) => { if (!open) props.onClose() }} class="w-full max-w-lg" data-testid="usage-dialog" aria-labelledby="usage-title">
      <div class="flex flex-col gap-4 p-4 md:p-6">
        <h2 id="usage-title" class="font-medium text-100">{t('Usage.Title', { name: props.connection.display_name || props.connection.name })}</h2>
        <Show when={!failed()} fallback={<div role="alert" class="flex items-center gap-3 text-sm"><span class="text-pink">{t('Usage.LoadFailed')}</span><Button size="sm" variant="secondary" onClick={() => void load()}>{t('Retry')}</Button></div>}>
          <Show when={usages()} fallback={<Skeleton variant="text" lines={3} />}>
            {(list) => <UsageList t={t} base={props.ctx.automationsPath} usages={list()} />}
          </Show>
        </Show>
        <div class="flex justify-end"><Button variant="secondary" class="min-h-11" data-dialog-close onClick={props.onClose}>{t('Usage.Close')}</Button></div>
      </div>
    </Dialog>
  )
}

function UsageList(props: { t: Translator; base: string; usages: UsageView[] }) {
  const t = props.t
  return (
    <Show when={props.usages.length > 0} fallback={<p class="text-sm text-200">{t('Usage.Empty')}</p>}>
      <ul class="flex flex-col gap-2" data-testid="usage-list">
        <For each={props.usages}>{(u) => (
          <li class="flex flex-wrap items-center gap-x-3 gap-y-1 rounded-md border border-subtle p-3 text-sm">
            <a href={usageHref(props.base, u)} class="font-medium text-100 underline-offset-2 hover:underline">{u.name}</a>
            <span class="text-200">{t('Usage.Version', { version: u.version })}</span>
            <span class={u.enabled ? 'text-green' : 'text-200'}>{u.enabled ? t('Usage.Enabled') : t('Usage.NotEnabled')}</span>
            <Show when={u.node_ids.length > 0}>
              <span class="text-200">{u.node_ids.map((n) => t('Usage.Node', { node: n })).join(', ')}</span>
            </Show>
          </li>
        )}</For>
      </ul>
    </Show>
  )
}

function DeleteDialog(props: { ctx: Ctx; connector: ConnectorView; connection: ConnectionView; onClose: () => void; onDeleted: (name: string) => void }) {
  const t = props.ctx.t
  const name = () => props.connection.display_name || props.connection.name
  const [busy, setBusy] = createSignal(false)
  const [error, setError] = createSignal<string>()
  const [inUse, setInUse] = createSignal<UsageView[]>()
  let root: HTMLDivElement | undefined
  const confirm = async () => {
    if (busy()) return
    setBusy(true)
    setError(undefined)
    try {
      const r = await props.ctx.call('automations.connections.delete', { id: props.connection.id })
      setBusy(false)
      if (r.ok) props.onDeleted(name())
      else if (r.error === 'CONNECTION_IN_USE') setInUse(r.usages ?? [])
      else setError(t('Toast.Failed'))
    } catch (e) {
      setBusy(false)
      console.warn('automations: delete failed', errorCode(e))
      setError(errorCode(e) === 'not_found' ? t('Toast.Failed') : t('Toast.Failed'))
    }
  }
  return (
    <Show when={!inUse()} fallback={
      <Dialog open onOpenChange={(open) => { if (!open) props.onClose() }} class="w-full max-w-lg" data-testid="in-use-dialog" aria-labelledby="in-use-title">
        <div class="flex flex-col gap-4 p-4 md:p-6">
          <h2 id="in-use-title" class="font-medium text-100">{t('Delete.InUseTitle', { name: name() })}</h2>
          <p class="text-sm text-200" role="alert">{t('Delete.InUseText')}</p>
          <UsageList t={t} base={props.ctx.automationsPath} usages={inUse()!} />
          <div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <Button variant="secondary" class="min-h-11 justify-center" data-dialog-close onClick={props.onClose}>{t('Delete.Close')}</Button>
            <Show when={props.connection.status !== 'disabled'}>
              <Button class="min-h-11 justify-center" onClick={() => { props.onClose(); void props.ctx.setDisabled(props.connection, true) }} data-testid="disable-instead">{t('Delete.DisableInstead')}</Button>
            </Show>
          </div>
        </div>
      </Dialog>
    }>
      <div ref={root}>
        <ConfirmationDialog open onOpenChange={(open) => { if (!open && !busy()) props.onClose() }} busy={busy()} danger
          heading={t('Delete.Title', { name: name() })}
          text={<>
            {t('Delete.Text')}
            <Show when={props.connector.nodes.length > 0}> {t('Delete.Nodes', { nodes: props.connector.nodes.join(', ') })}</Show>
            <Show when={error()}><span role="alert" class="mt-2 block text-pink">{error()}</span></Show>
          </>}
          confirmText={t('Delete.Confirm')} cancelText={t('Delete.Cancel')} closeLabel={t('Close')}
          initialFocus={() => root?.ownerDocument.querySelector<HTMLElement>('[data-testid="delete-dialog"] [data-test-id="dialog-cancel-btn"]') ?? undefined}
          onCancel={props.onClose} onConfirm={() => void confirm()} data-testid="delete-dialog">
          <DialogBusyStatus class="px-4" />
        </ConfirmationDialog>
      </div>
    </Show>
  )
}

type SecretFormState = { mode: 'create' } | { mode: 'edit' | 'rotate'; secret: SecretView }

/**
 * The built-in «Secret» connector: the tenant's secrets that Code and HTTP nodes read by name. A value is
 * entered (create, rotate) and never comes back: a secret is shown by its name, description and dates,
 * its value only as a mask.
 */
function SecretsSection(props: { ctx: Ctx; writable: boolean }) {
  const t = props.ctx.t
  const toast = useToast()
  const [secrets, setSecrets] = createSignal<SecretView[]>()
  const [failed, setFailed] = createSignal(false)
  const [form, setForm] = createSignal<SecretFormState>()
  const [deleting, setDeleting] = createSignal<SecretView>()
  // Loads overlap (a save, a delete, a retry): only the latest request may set the list or the failure, so a
  // slower earlier response never replaces a newer list.
  let generation = 0
  const load = async (): Promise<SecretView[] | undefined> => {
    const current = ++generation
    setFailed(false)
    try {
      const list = sortSecrets((await props.ctx.call('automations.secrets.list', {})).secrets)
      if (current === generation) setSecrets(list)
      return list
    } catch (e) {
      console.warn('automations: secrets.list failed', errorCode(e))
      if (current === generation) setFailed(true)
      return undefined
    }
  }
  void load()
  const saved = (secret: SecretView, mode: SecretFormState['mode']) => {
    setForm(undefined)
    props.ctx.highlight(secret.id)
    void load()
    const key = mode === 'create' ? 'Secrets.Toast.Created' : mode === 'rotate' ? 'Secrets.Toast.Rotated' : 'Secrets.Toast.Updated'
    toast.notify({ variant: 'success', title: t(key, { name: secret.name }) })
  }
  return (
    <section class="flex flex-col gap-3" aria-labelledby="cat-secrets" data-testid="secrets-section">
      <h2 id="cat-secrets" class="text-base font-medium text-100">{t('Secrets.Category')}</h2>
      {/* The same grid as the connector cards: the secret card is one of them. */}
      <div class="grid grid-cols-1 gap-4 md:grid-cols-2 2xl:grid-cols-3">
      <article class="flex min-w-0 flex-col gap-4 rounded-lg border border-subtle bg-surface-300 p-4" data-testid="connector-secret" aria-labelledby="connector-title-secret">
        <div class="flex flex-wrap items-start gap-3">
          <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-surface-500 text-100" aria-hidden="true">
            <svg width="20" height="20" viewBox="0 0 256 256" fill="none" stroke="currentColor" stroke-width="16" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="84" cy="172" r="44" /><line x1="115" y1="141" x2="208" y2="48" /><line x1="176" y1="80" x2="208" y2="112" /><line x1="148" y1="108" x2="172" y2="132" />
            </svg>
          </span>
          <div class="flex min-w-0 flex-1 flex-col gap-1">
            <h3 id="connector-title-secret" class="truncate font-medium text-100">{t('Secrets.Title')}</h3>
            <p class="text-xs text-200">{t('Secrets.Kind')}</p>
          </div>
          {/* Without a store that can write (no encryption key) a value cannot be saved: no create offered. */}
          <Button size="sm" variant={(secrets()?.length ?? 0) > 0 || !props.writable ? 'secondary' : 'primary'} class="min-h-11 sm:min-h-9"
            disabled={!props.writable} aria-describedby={props.writable ? undefined : 'secrets-not-configured'}
            onClick={() => setForm({ mode: 'create' })} data-testid="secret-add">{t('Secrets.Add')}</Button>
        </div>
        <p class="text-sm text-200">{t('Secrets.Summary')}</p>
        <Show when={!props.writable}>
          <p id="secrets-not-configured" role="status" class="flex items-start gap-2 text-sm text-100" data-testid="secrets-not-configured">
            <span class="mt-1.5 inline-flex h-2 w-2 shrink-0 rounded-full bg-current text-yellow" aria-hidden="true" />
            <span class="min-w-0 flex-1">{t('Secrets.NotConfigured')}</span>
          </p>
        </Show>
        <Show when={!failed()} fallback={
          <div role="alert" class="flex flex-wrap items-center gap-3 text-sm" data-testid="secrets-load-error">
            <span class="text-pink">{t('Secrets.LoadFailed')}</span>
            <Button size="sm" variant="secondary" onClick={() => void load()}>{t('Retry')}</Button>
          </div>
        }>
          <Show when={secrets()} fallback={<Skeleton variant="text" lines={2} />}>
            {(list) => (
              <Show when={list().length > 0} fallback={<p class="text-sm text-200" data-testid="no-secrets">{t('Secrets.Empty')}</p>}>
                <ul class="flex flex-col" aria-label={t('Secrets.Category')} data-testid="secrets-list">
                  <For each={list()}>{(secret) => (
                    <SecretRow ctx={props.ctx} secret={secret} writable={props.writable} onEdit={() => setForm({ mode: 'edit', secret })}
                      onRotate={() => setForm({ mode: 'rotate', secret })} onDelete={() => setDeleting(secret)} />
                  )}</For>
                </ul>
              </Show>
            )}
          </Show>
        </Show>
      </article>
      </div>
      <Show when={form()} keyed>
        {(f) => <SecretForm ctx={props.ctx} state={f} onClose={() => setForm(undefined)} onSaved={(secret) => saved(secret, f.mode)} reload={load} />}
      </Show>
      <Show when={deleting()} keyed>
        {(secret) => <DeleteSecretDialog ctx={props.ctx} secret={secret} onClose={() => setDeleting(undefined)} onDeleted={() => {
          setDeleting(undefined)
          void load()
          toast.notify({ variant: 'success', title: t('Secrets.Toast.Deleted', { name: secret.name }) })
        }} />}
      </Show>
    </section>
  )
}

function SecretRow(props: { ctx: Ctx; secret: SecretView; writable: boolean; onEdit: () => void; onRotate: () => void; onDelete: () => void }) {
  const t = props.ctx.t
  const s = () => props.secret
  const [menuOpen, setMenuOpen] = createSignal(false)
  const pick = (fn: () => void) => () => { setMenuOpen(false); fn() }
  return (
    <li class={`flex items-start gap-2 border-t border-subtle py-3 first:border-t-0 first:pt-0 ${props.ctx.highlighted() === s().id ? 'rounded-md outline outline-2 outline-offset-4 outline-brand-500' : ''}`} data-testid={`secret-${s().name}`}>
      <div class="flex min-w-0 flex-1 flex-col gap-1">
        <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
          <span class="break-all font-mono text-sm font-medium text-100">{s().name}</span>
          <span class="font-mono text-sm tracking-widest text-200" role="img" aria-label={t('Secrets.Masked')} data-testid="secret-masked">{MASKED_VALUE}</span>
        </div>
        <Show when={s().description}><p class="text-sm text-200">{s().description}</p></Show>
        <p class="text-xs text-200" data-testid="secret-meta">{secretMeta(t, s(), props.ctx.lang).join(' · ')}</p>
      </div>
      <Dropdown open={menuOpen()} onOpenChange={setMenuOpen} class={menuOpen() ? 'z-30' : undefined} label={t('Secrets.Actions', { name: s().name })} menuClass="min-w-56"
        trigger={<span class="flex h-11 w-11 items-center justify-center rounded-md text-200 sm:h-9 sm:w-9" aria-hidden="true">
          <svg width="20" height="20" viewBox="0 0 256 256" fill="currentColor"><circle cx="128" cy="64" r="16" /><circle cx="128" cy="128" r="16" /><circle cx="128" cy="192" r="16" /></svg>
        </span>}>
        <DropdownItem onClick={pick(props.onEdit)} data-testid="secret-menu-edit">{t('Secrets.Menu.Edit')}</DropdownItem>
        <DropdownItem onClick={pick(props.onRotate)} disabled={!props.writable} class="disabled:cursor-not-allowed disabled:text-200" data-testid="secret-menu-rotate">{t('Secrets.Menu.Rotate')}</DropdownItem>
        <DropdownItem onClick={pick(props.onDelete)} class="text-pink" data-testid="secret-menu-delete">{t('Secrets.Menu.Delete')}</DropdownItem>
      </Dropdown>
    </li>
  )
}

/** Creates a secret, renames or describes one, or writes its new value. The value field is never prefilled. */
function SecretForm(props: { ctx: Ctx; state: SecretFormState; onClose: () => void; onSaved: (s: SecretView) => void; reload: () => Promise<SecretView[] | undefined> }) {
  const t = props.ctx.t
  const mode = props.state.mode
  // The version the change applies to: the one the form opened with, or the latest after a conflict.
  const [base, setBase] = createSignal(props.state.mode === 'create' ? undefined : props.state.secret)
  const [name, setName] = createSignal(base()?.name ?? '')
  const [description, setDescription] = createSignal(base()?.description ?? '')
  const [value, setValue] = createSignal('')
  const [shown, setShown] = createSignal(false)
  const [fieldErrors, setFieldErrors] = createSignal<Record<string, string>>({})
  const [error, setError] = createSignal<string>()
  const [busy, setBusy] = createSignal(false)
  let formRef: HTMLFormElement | undefined
  const focusFirstError = (errs: Record<string, string>) => queueMicrotask(() => {
    const first = Object.keys(errs)[0]
    if (first) formRef?.querySelector<HTMLElement>(`[data-field="${CSS.escape(first)}"]`)?.focus()
  })
  const submit = async (event: Event) => {
    event.preventDefault()
    if (busy()) return
    const local = secretFormErrors(mode, name(), value())
    setFieldErrors(local)
    setError(undefined)
    if (Object.keys(local).length > 0) { focusFirstError(local); return }
    setBusy(true)
    try {
      const b = base()
      const r = mode === 'create'
        ? await props.ctx.call('automations.secrets.create', { name: name().trim(), description: description().trim(), value: value() })
        : mode === 'rotate'
          ? await props.ctx.call('automations.secrets.rotate', { id: b!.id, version: b!.version, value: value() })
          : await props.ctx.call('automations.secrets.update', {
            id: b!.id, version: b!.version,
            ...(name().trim() !== b!.name ? { name: name().trim() } : {}),
            ...(description().trim() !== b!.description ? { description: description().trim() } : {}),
          })
      if (r.ok && r.secret) {
        setBusy(false)
        setValue('') // the value leaves the page with the request
        props.onSaved(r.secret)
        return
      }
      setFieldErrors(r.fields ?? {})
      setError(secretSaveErrorText(t, r))
      if (r.error === 'version_conflict' && b) {
        // The latest version is loaded: what the person typed stays, the next save applies it to that version.
        // A field the person did not touch follows the latest version, so the save does not send the old one back.
        // The form stays busy until the draft and the base are rebased: a save meanwhile would send a stale version.
        try {
          const latest = (await props.reload())?.find((x) => x.id === b.id)
          if (latest) {
            const next = rebaseSecretDraft({ name: name(), description: description() }, b, latest)
            setName(next.name)
            setDescription(next.description)
            setBase(latest)
          }
        } catch (e) {
          console.warn('automations: reloading the secret after a conflict failed', errorCode(e))
        }
      }
      setBusy(false)
      if (r.fields) focusFirstError(r.fields)
    } catch (e) {
      setBusy(false)
      const code = errorCode(e)
      console.warn('automations: secret save failed', code)
      setError(t(code === 'unavailable' ? 'Secrets.Form.Unavailable' : 'Secrets.Form.Failed'))
    }
  }
  const title = () => mode === 'create' ? t('Secrets.Form.TitleNew') : t(mode === 'rotate' ? 'Secrets.Form.TitleRotate' : 'Secrets.Form.TitleEdit', { name: base()!.name })
  const fieldError = (f: string) => (fieldErrors()[f] ? secretFieldErrorText(t, fieldErrors()[f]!) : undefined)
  return (
    <ViewDrawer open onOpenChange={(open) => { if (!open && !busy()) props.onClose() }} busy={busy()} title={title()} closeLabel={t('Close')} data-testid="secret-form">
      <form ref={formRef} class="flex flex-col gap-4 p-4 md:p-6" onSubmit={(e) => void submit(e)} novalidate data-version={base()?.version}>
        <Show when={mode !== 'rotate'}>
          <Input label={t('Secrets.Form.Name')} value={name()} onInput={(e) => setName(secretName(e.currentTarget.value))}
            description={t('Secrets.Form.NameHint')} placeholder="API_KEY" data-field="name" error={fieldError('name')} disabled={busy()} required autocomplete="off" spellcheck={false} class="font-mono" />
          <Textarea label={t('Secrets.Form.DescriptionLabel')} value={description()} onInput={(e) => setDescription(e.currentTarget.value)} rows={3}
            description={t('Secrets.Form.DescriptionHint')} data-field="description" error={fieldError('description')} disabled={busy()} />
          <Show when={mode === 'edit'}><p class="text-sm text-200" data-testid="secret-rename-hint">{t('Secrets.Form.RenameHint')}</p></Show>
        </Show>
        <Show when={mode !== 'edit'}>
          <Show when={mode === 'rotate'}><p class="text-sm text-200">{t('Secrets.Form.RotateHint')}</p></Show>
          <Input label={t(mode === 'rotate' ? 'Secrets.Form.NewValue' : 'Secrets.Form.Value')} type={shown() ? 'text' : 'password'} value={value()}
            onInput={(e) => setValue(e.currentTarget.value)} data-field="value" error={fieldError('value')}
            disabled={busy()} required autocomplete="new-password" spellcheck={false}
            addonRight={<button type="button" class="mx-2 min-h-11 px-2 text-sm text-200 sm:min-h-0" aria-pressed={shown()} onClick={() => setShown(!shown())}>
              {shown() ? t('Form.Hide') : t('Form.Show')}
            </button>} />
          <p class="text-sm text-200" data-testid="secret-value-hint">{t('Secrets.Form.ValueHint')}</p>
        </Show>
        <Show when={error()}><p role="alert" class="text-sm text-pink" data-testid="secret-form-error">{error()}</p></Show>
        <div class="sticky bottom-0 flex flex-col-reverse gap-2 border-t border-subtle bg-surface-300 pt-4 sm:flex-row sm:justify-end">
          <Button type="button" variant="secondary" class="min-h-11 justify-center" data-dialog-close aria-disabled={busy() || undefined}
            onClick={() => { if (!busy()) props.onClose() }}>{t('Form.Cancel')}</Button>
          <Button type="submit" class="min-h-11 justify-center" loading={busy()} aria-disabled={busy() || undefined} data-testid="secret-save">
            {busy() ? t('Secrets.Form.Saving') : t(mode === 'rotate' ? 'Secrets.Form.Rotate' : 'Secrets.Form.Save')}
          </Button>
        </div>
      </form>
    </ViewDrawer>
  )
}

function DeleteSecretDialog(props: { ctx: Ctx; secret: SecretView; onClose: () => void; onDeleted: () => void }) {
  const t = props.ctx.t
  const [busy, setBusy] = createSignal(false)
  const [error, setError] = createSignal<string>()
  let root: HTMLDivElement | undefined
  const confirm = async () => {
    if (busy()) return
    setBusy(true)
    setError(undefined)
    try {
      await props.ctx.call('automations.secrets.delete', { id: props.secret.id })
      setBusy(false)
      props.onDeleted()
    } catch (e) {
      setBusy(false)
      const code = errorCode(e)
      console.warn('automations: secret delete failed', code)
      // Already gone (another administrator deleted it): the list is what is left.
      if (code === 'not_found') { props.onDeleted(); return }
      setError(t(code === 'unavailable' ? 'Secrets.Form.Unavailable' : 'Toast.Failed'))
    }
  }
  return (
    <div ref={root}>
      <ConfirmationDialog open onOpenChange={(open) => { if (!open && !busy()) props.onClose() }} busy={busy()} danger
        heading={t('Secrets.Delete.Title', { name: props.secret.name })}
        text={<>
          {t('Secrets.Delete.Text', { name: props.secret.name })}
          <Show when={error()}><span role="alert" class="mt-2 block text-pink">{error()}</span></Show>
        </>}
        confirmText={t('Secrets.Delete.Confirm')} cancelText={t('Secrets.Delete.Cancel')} closeLabel={t('Close')}
        initialFocus={() => root?.ownerDocument.querySelector<HTMLElement>('[data-testid="secret-delete-dialog"] [data-test-id="dialog-cancel-btn"]') ?? undefined}
        onCancel={props.onClose} onConfirm={() => void confirm()} data-testid="secret-delete-dialog">
        <DialogBusyStatus class="px-4" />
      </ConfirmationDialog>
    </div>
  )
}

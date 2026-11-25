import { ErrorBoundary, Match, Switch, createSignal, lazy, onCleanup, onMount, Suspense, type Component, type JSX } from 'solid-js'
import { FetchRPCTransport, RPCClient, useClientHost, type SolidRouteProps } from '@iota-uz/sdk/solid'
import { EditorSkeleton } from '@iota-uz/sdk/automation-editor'
import { Button, ToastProvider, useToast } from '@iota-uz/sdk/solid-ui'
import '@iota-uz/sdk/automation-editor/styles.css'
import { rpcContracts, type Props } from './Automations.solid.generated'
import { AppContext, useApp, type Call, type Ctx } from './context'
import { List } from './List'
import { editorLocale, errorCode, errorText, graphPath, translator } from './model'

// The editor page and, through it, the editor's canvas chunk load only on a graph's page (AL4, P3).
const EditorPage = lazy(() => import('./EditorPage'))

/** Automations is the screen of /automations, /automations/new and /automations/{id}. */
const Automations: Component<SolidRouteProps<Props>> = (props) => (
  <ToastProvider>
    <Screen {...props} />
  </ToastProvider>
)
export default Automations

const Screen: Component<SolidRouteProps<Props>> = (props) => {
  const host = useClientHost()
  const toast = useToast()
  const init = props.route.initial
  const lang = host.locale.language
  const client = new RPCClient(new FetchRPCTransport(props.route.services?.rpc ?? `${init.basePath}/rpc`, host.session))
  const call: Call = (method, params) => {
    const contract = rpcContracts[method]
    const ctrl = new AbortController()
    return contract.kind === 'query'
      ? client.query(contract, params, ctrl.signal)
      : client.mutate(contract, params, ctrl.signal)
  }
  const [now, setNow] = createSignal(new Date())
  const timer = setInterval(() => setNow(new Date()), 60_000)
  onCleanup(() => clearInterval(timer))
  const ctx: Ctx = {
    t: translator((k) => host.locale.t(k)),
    lang,
    call,
    props: init,
    toast,
    now,
    navigate: (path) => window.location.assign(path),
  }
  return (
    <AppContext.Provider value={ctx}>
      <div lang={lang} class="min-w-0">
        <Switch>
          <Match when={init.view === 'list'}><List /></Match>
          <Match when={init.view === 'run'}><RunLink /></Match>
          <Match when={init.view !== 'list'}>
            <ErrorBoundary fallback={(error) => {
              console.warn('automations: the editor did not load', error)
              return (
                <div role="alert" class="m-4 flex flex-col items-start gap-3 rounded-lg border border-subtle p-6" data-testid="editor-load-error">
                  <p>{ctx.t('Editor.LoadFailed')}</p>
                  {/* The browser keeps a failed module import: a retry loads the page again. */}
                  <Button variant="secondary" onClick={() => window.location.reload()}>{ctx.t('Common.Retry')}</Button>
                </div>
              )
            }}>
              <Suspense fallback={<div class="h-[calc(100vh-4rem)]"><EditorSkeleton locale={editorLocale(lang)} /></div>}>
                <EditorPage />
              </Suspense>
            </ErrorBoundary>
          </Match>
        </Switch>
      </div>
    </AppContext.Provider>
  )
}

/**
 * A link that knows only a run's id (a notification, a task, the journal: RV6, D3): the run names its
 * graph, and the graph's Runs tab opens on it. A run that is deleted or another tenant's is «not found».
 */
function RunLink(): JSX.Element {
  const app = useApp()
  const t = app.t
  const [missing, setMissing] = createSignal(false)
  const [failed, setFailed] = createSignal<string>()
  const open = async () => {
    setFailed(undefined)
    const run = app.props.run ?? ''
    try {
      const data = await app.call('automations.run', { run_id: run })
      // The node the link names stays chosen in the run (D1).
      const node = new URLSearchParams(window.location.search).get('node')
      window.location.replace(graphPath(app.props.basePath, data.run.automation_id, { run, ...(node ? { node } : {}) }))
    } catch (e) {
      if (errorCode(e) === 'not_found') setMissing(true)
      else setFailed(errorText(t, e, t('Actions.LoadRuns')))
    }
  }
  onMount(() => void open())
  return (
    <div class="m-4 flex flex-col items-start gap-3 rounded-lg border border-subtle p-6" data-testid="run-link">
      <Switch fallback={<p role="status" class="text-200">{t('Runs.Opening')}</p>}>
        <Match when={missing()}>
          <div role="alert" class="flex flex-col items-start gap-2" data-testid="run-not-found">
            <h1 class="text-lg font-medium">{t('Runs.NotFound')}</h1>
            <p class="text-200">{t('Runs.NotFoundText')}</p>
            <Button variant="secondary" href={app.props.basePath}>{t('Editor.BackToList')}</Button>
          </div>
        </Match>
        <Match when={failed()}>
          <div role="alert" class="flex flex-col items-start gap-2">
            <p>{failed()}</p>
            <Button variant="secondary" onClick={() => void open()}>{t('Common.Retry')}</Button>
          </div>
        </Match>
      </Switch>
    </div>
  )
}

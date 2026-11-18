// Activation of a package shell renderer through the client host's ShellHost.
// The host (PR #1245) owns the single app-wide renderer, the outlet handoff and
// the fall back to the standard shell; this module loads the package module and
// adapts ShellRendererContext into the author-facing ShellProps.
import { createComponent, createSignal, ErrorBoundary, onCleanup, type Accessor, type Component } from 'solid-js'
import { render } from 'solid-js/web'
import type {
  NavGroupModel, NavItemModel, NavModel, NavWorkspaceModel, NotificationsModel, PaletteModel, ShellProps, ThemeMode, ThemeModel, UserModel,
} from '../client.ts'
import { RuntimeProvider } from './hooks.ts'
import type { RuntimeContext } from './protocol.ts'
import { createSlotSource, SlotsProvider } from './slots.ts'
import { ensureStyles } from './styles.ts'
import type { RuntimeTransport } from './transport.ts'

// Structural views of the client host objects (see web/client-host/src/shell-*.ts).
export interface ShellNode {
  key: string
  title: string
  href?: string
  icon?: string
  workspace?: string
  children: ShellNode[]
}

export interface ShellNavigationSource {
  navigation(): ShellNode[]
  subscribe(listener: () => void): () => void
}

export interface ThemeServiceLike {
  current(): 'light' | 'dark'
  set(theme: 'light' | 'dark'): void
  subscribe(listener: (theme: 'light' | 'dark') => void): () => void
}

export interface ShellRendererContextLike {
  root: HTMLElement
  outlet: HTMLElement
  navigation: ShellNavigationSource
  theme: ThemeServiceLike
  locale: { language: string }
  user: { name: string; email: string } | null
  actions: { logout(): void }
  notifications: { subscribe(listener: () => void): () => void }
  fail(cause: unknown): void
}

export interface ShellRendererLike {
  id: string
  mount(context: ShellRendererContextLike): void | (() => void)
}

export interface ShellHostLike {
  activate(renderer: ShellRendererLike): boolean
  diagnostics(): { code: string; renderer: string; message: string }[]
}

/** What a loaded shell module exposes: the value built by `shellRenderer()`. */
export interface LoadedShell {
  component: Component<ShellProps>
}

/**
 * The standard layout's theme controller (`window.iotaTheme`, theme.js) owns the class of <html> that Tailwind's
 * `dark:` variants read and the stored choice. The host theme service only writes `data-theme`, so a renderer that
 * changed the theme through it alone would leave the legacy page content in the old theme and forget the choice on
 * the next page. Keep the controller in step; without one, the class is the resolved theme.
 */
function syncLegacyTheme(win: Window, mode: ThemeMode, resolved: 'light' | 'dark'): void {
  const legacy = (win as Window & { iotaTheme?: { change?(choice: string): void } }).iotaTheme
  if (typeof legacy?.change === 'function') {
    legacy.change(mode)
    return
  }
  const root = win.document.documentElement
  root.classList.remove('light', 'dark', 'system')
  root.classList.add(resolved)
}

function toItem(node: ShellNode, location: Location | undefined): NavItemModel {
  const href = node.href ?? ''
  return {
    id: node.key,
    label: node.title,
    href,
    ...(node.icon ? { icon: node.icon } : {}),
    active: href !== '' && !!location && (location.pathname === href || location.pathname.startsWith(href.endsWith('/') ? href : href + '/')),
    children: node.children.map((child) => toItem(child, location)),
  }
}

/**
 * Builds the NavModel the shell renders: roots with children become groups, the
 * leaf roots form one unnamed group, workspaces and palette come from Go.
 */
export function buildNavModel(
  nodes: readonly ShellNode[],
  context: Pick<RuntimeContext, 'navigation'>,
  activeWorkspace: string,
  location: Location | undefined,
): NavModel {
  const workspaces = [...context.navigation.workspaces].sort((a, b) => a.order - b.order)
  const fallback = workspaces[0]?.id ?? ''
  const visible = nodes.filter((n) => workspaces.length === 0 || (n.workspace ?? fallback) === activeWorkspace)
  const groups: NavGroupModel[] = []
  const leaves = visible.filter((n) => n.children.length === 0)
  if (leaves.length > 0) groups.push({ id: '', label: '', items: leaves.map((n) => toItem(n, location)) })
  for (const n of visible.filter((x) => x.children.length > 0)) {
    groups.push({ id: n.key, label: n.title, ...(n.icon ? { icon: n.icon } : {}), items: n.children.map((c) => toItem(c, location)) })
  }
  return {
    workspaces: workspaces.map<NavWorkspaceModel>((w) => ({ id: w.id, label: w.label, ...(w.icon ? { icon: w.icon } : {}), active: w.id === activeWorkspace })),
    groups,
    palette: context.navigation.palette.map<PaletteModel>((p) => ({ id: p.id, label: p.label, href: p.href, keywords: p.keywords })),
  }
}

export interface ShellRendererOptions {
  id: string
  shell: LoadedShell
  context: RuntimeContext
  window: Window
  /** Lets the shell use `useQuery` / `useCommand`. */
  transport?: RuntimeTransport
  importModule?: (url: string) => Promise<Record<string, unknown>>
}

/** Wraps a loaded shell as a renderer for ShellHost.activate. */
export function createShellRenderer(options: ShellRendererOptions): ShellRendererLike {
  const { shell, context } = options
  const win = options.window
  return {
    id: options.id,
    mount(ctx) {
      const firstWorkspace = [...context.navigation.workspaces].sort((a, b) => a.order - b.order)[0]?.id ?? ''
      const subscriptions: (() => void)[] = []
      let mounting = true
      let early: { error: unknown } | undefined
      // Everything reactive lives in this root, so one disposer releases it.
      const disposeRoot = render(() => {
        const [nodes, setNodes] = createSignal<ShellNode[]>(ctx.navigation.navigation())
        const [path, setPath] = createSignal(win.location.pathname)
        const [mode, setMode] = createSignal<ThemeMode>(ctx.theme.current())
        const [resolved, setResolved] = createSignal<'light' | 'dark'>(ctx.theme.current())
        const [unread] = createSignal(0)

        subscriptions.push(ctx.navigation.subscribe(() => setNodes(ctx.navigation.navigation())))
        subscriptions.push(ctx.theme.subscribe((t) => setResolved(t)))
        const onNavigate = () => setPath(win.location.pathname)
        win.addEventListener('popstate', onNavigate)
        win.document.addEventListener('htmx:pushedIntoHistory', onNavigate)
        onCleanup(() => {
          win.removeEventListener('popstate', onNavigate)
          win.document.removeEventListener('htmx:pushedIntoHistory', onNavigate)
        })

        const theme: ThemeModel = {
          get mode() { return mode() },
          get resolved() { return resolved() },
          set(next) {
            setMode(next)
            const target = next === 'system' ? (win.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light') : next
            syncLegacyTheme(win, next, target)
            ctx.theme.set(target)
          },
        }
        const notifications: NotificationsModel = {
          get unread() { return unread() },
          items: [],
          markRead() { /* the client host publishes no notification feed yet */ },
        }
        const email = context.user?.email ?? ctx.user?.email
        const user: UserModel = {
          id: context.user?.id ?? '',
          name: context.user?.name ?? ctx.user?.name ?? '',
          ...(email ? { email } : {}),
          permissions: context.permissions,
        }
        const props: ShellProps = {
          outlet: ctx.outlet,
          get nav() { return buildNavModel(nodes(), context, firstWorkspace, { pathname: path() } as Location) },
          user,
          locale: ctx.locale.language,
          theme,
          notifications,
          logout: async () => { ctx.actions.logout() },
        }
        const slots = createSlotSource({
          contributions: context.slots ?? {},
          ...(options.importModule ? { importModule: options.importModule } : {}),
          document: win.document,
          console: console,
        })
        const body = () => createComponent(SlotsProvider, {
          source: slots,
          get children() { return createComponent(shell.component, props) },
        })
        return createComponent(ErrorBoundary, {
          fallback: (error: unknown) => {
            // A failure while mounting makes mount() throw, so activation reports
            // false; a later one puts the standard shell back through ctx.fail.
            if (mounting) early = { error }
            else queueMicrotask(() => ctx.fail(error))
            return null
          },
          get children() {
            const transport = options.transport
            return transport ? createComponent(RuntimeProvider, { transport, get children() { return body() } }) : body()
          },
        })
      }, ctx.root)
      mounting = false
      const release = () => {
        for (const stop of subscriptions.splice(0)) stop()
        disposeRoot()
      }
      if (early) {
        release()
        throw early.error
      }
      return release
    },
  }
}

export interface ActivateShellOptions<Host extends ShellHostLike> {
  context: RuntimeContext
  host: Host
  window: Window & typeof globalThis
  importModule?: (url: string) => Promise<Record<string, unknown>>
  /** Transport for the shell's own queries and commands. */
  transport?: RuntimeTransport
}

/**
 * Loads the bound shell module and activates it app-wide. The standard shell
 * stays visible while the module loads and whenever loading, validating or
 * mounting fails, so a broken package can never leave the user without a
 * shell. Resolves true once the renderer is active.
 */
export async function activateRuntimeShell<Host extends ShellHostLike>(options: ActivateShellOptions<Host>): Promise<boolean> {
  const ref = options.context.shell
  if (!ref) return false
  const load = options.importModule ?? ((url: string) => import(/* @vite-ignore */ url) as Promise<Record<string, unknown>>)
  const console = options.window.console
  let module: Record<string, unknown>
  try {
    const [loaded] = await Promise.all([load(ref.module), ensureStyles(options.window.document, ref.styles)])
    module = loaded
  } catch (cause) {
    console.error(`Shell renderer ${ref.id} could not be loaded; the standard shell stays`, cause)
    return false
  }
  const value = (ref.export ? module[ref.export] : module.default) as Partial<LoadedShell> | undefined
  if (!value || typeof value.component !== 'function') {
    console.error(`Shell renderer ${ref.id}: module ${ref.module} has no shell export "${ref.export ?? 'default'}"`)
    return false
  }
  return options.host.activate(createShellRenderer({
    id: ref.id, shell: value as LoadedShell, context: options.context, window: options.window,
    ...(options.transport ? { transport: options.transport } : {}),
    ...(options.importModule ? { importModule: options.importModule } : {}),
  }))
}

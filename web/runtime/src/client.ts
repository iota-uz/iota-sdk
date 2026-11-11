// Client graph: pages, navigation, slots, cell renderers and the shell. Declarations and branded
// values only; the canonical client host mounts them. Server values must never be imported here
// (the build slice rejects that edge; it cannot be expressed in types).
import type { Component } from 'solid-js'
import type { Column, Command, AtomicCommand, InputOf, Permission, Query, ResultOf, Route, SlotRef } from './contracts.ts'
import type { ImplKind, Json, Label, NavGroupJSON, NavPatchJSON, PaletteEntryJSON, ResourceJSON, WorkspaceJSON } from './model.ts'
import { contract, omitEmpty, type Contract } from './resource.ts'
import type { AnySchema, Infer } from './schema.ts'
import { decode, encode } from './wire.ts'

/** How a client value without an explicit ID is named at manifest collection. */
export type ImplicitId = 'export' | 'navigation' | 'shell'

export interface ClientValue<K extends 'page' | 'navigation' | 'slot' | 'cell' | 'shell' = 'page'> extends Contract<string, K> {
  readonly implicitId?: ImplicitId
}

export interface NavSpec {
  label: Label
  group?: string
  parent?: string
  icon?: string
  order?: number
}

export interface PageProps<P = Record<string, string>> {
  readonly params: P
}

export interface Page<P = Record<string, string>> extends ClientValue<'page'> {
  readonly component: Component<PageProps<P>>
}

export interface PageSpec<P> {
  nav?: NavSpec
  permission?: Permission
  component: Component<PageProps<P>>
}

/** A page bound to a shared route (typed params) or declared with its own ID and path. */
export function page<R extends Route>(route: R, spec: PageSpec<Infer<R['params']>>): Page<Infer<R['params']>>
export function page(id: string, spec: PageSpec<Record<string, string>> & { path: string }): Page
export function page(
  target: string | Route,
  spec: PageSpec<never> & { path?: string },
): Page<never> | Page {
  const isRoute = typeof target !== 'string'
  const id = isRoute ? target.id : target
  const path = isRoute ? target.path : spec.path
  if (path === undefined) throw new Error(`page ${id}: path is required`)
  const resource = omitEmpty({
    id,
    kind: 'page' as const,
    path,
    route: isRoute ? target.id : undefined,
    nav: spec.nav as ResourceJSON['nav'],
    permission: spec.permission?.id,
  }) as ResourceJSON
  return { ...contract({ ...resource, id, kind: 'page' }), component: spec.component as Component<PageProps<never>> }
}

export interface NavigationSpec {
  id?: string
  groups?: readonly NavGroupJSON[]
  /** Hide, rename or move existing nodes by their resource ID (core or other packages). */
  patches?: readonly NavPatchJSON[]
  workspaces?: readonly WorkspaceJSON[]
  /** Command-palette / search entries pointing at pages or commands. */
  palette?: readonly PaletteEntryJSON[]
}

export interface Navigation extends ClientValue<'navigation'> {}

export function navigation(spec: NavigationSpec): Navigation {
  const id = spec.id ?? ''
  const resource = omitEmpty({
    id,
    kind: 'navigation' as const,
    groups: spec.groups && [...spec.groups],
    patches: spec.patches && [...spec.patches],
    workspaces: spec.workspaces && [...spec.workspaces],
    palette: spec.palette && [...spec.palette],
  }) as ResourceJSON
  return { ...contract({ ...resource, id, kind: 'navigation' }), ...(spec.id ? {} : { implicitId: 'navigation' as const }) }
}

export interface Slot<P extends object = Record<string, Json>> extends ClientValue<'slot'> {
  readonly component: Component<P>
}

/** Contributes a component to a slot (`target` is the slot ID or a typed `SlotRef`). */
export function slot<P extends object>(target: SlotRef<P> | string, component: Component<P>, opts: { id?: string } = {}): Slot<P> {
  const targetId = typeof target === 'string' ? target : target.slotId
  const id = opts.id ?? ''
  const resource = omitEmpty({ id, kind: 'slot' as const, target: targetId }) as ResourceJSON
  return { ...contract({ ...resource, id, kind: 'slot' }), component, ...(opts.id ? {} : { implicitId: 'export' as const }) }
}

export interface CellProps<V = Json> {
  readonly value: V
  readonly row: Readonly<Record<string, Json>>
}

export interface Cell<V = Json> extends ClientValue<'cell'> {
  readonly component: Component<CellProps<V>>
}

/** Custom renderer for an overlay column in the native list. */
export function cell<V>(column: Column<string, V>, component: Component<CellProps<V>>, opts: { id?: string } = {}): Cell<V> {
  const id = opts.id ?? ''
  const resource = omitEmpty({ id, kind: 'cell' as const, target: column.id }) as ResourceJSON
  return { ...contract({ ...resource, id, kind: 'cell' }), component, ...(opts.id ? {} : { implicitId: 'export' as const }) }
}

// ---- shell ----

export interface NavItemModel {
  readonly id: string
  readonly label: string
  readonly href: string
  readonly icon?: string
  readonly active: boolean
  readonly children: readonly NavItemModel[]
}

export interface NavGroupModel {
  readonly id: string
  readonly label: string
  readonly icon?: string
  readonly items: readonly NavItemModel[]
}

export interface NavWorkspaceModel {
  readonly id: string
  readonly label: string
  readonly icon?: string
  readonly active: boolean
}

export interface PaletteModel {
  readonly id: string
  readonly label: string
  readonly href: string
  readonly keywords: readonly string[]
}

/** Navigation projected by Go from the activated manifest, localized and permission-filtered. */
export interface NavModel {
  readonly workspaces: readonly NavWorkspaceModel[]
  /** Groups of the active workspace. */
  readonly groups: readonly NavGroupModel[]
  readonly palette: readonly PaletteModel[]
}

export interface UserModel {
  readonly id: string
  readonly name: string
  readonly email?: string
  readonly avatarUrl?: string
  readonly permissions: readonly string[]
}

export type ThemeMode = 'light' | 'dark' | 'system'

export interface ThemeModel {
  readonly mode: ThemeMode
  readonly resolved: 'light' | 'dark'
  set(mode: ThemeMode): void
}

export interface NotificationModel {
  readonly id: string
  readonly title: string
  readonly body: string
  readonly href?: string
  readonly read: boolean
  readonly createdAt: string
}

export interface NotificationsModel {
  readonly unread: number
  readonly items: readonly NotificationModel[]
  markRead(id: string): void
}

export interface ShellProps {
  /** Host-owned element that contains the current page (legacy or Solid); render it once, e.g. `{props.outlet}`. */
  readonly outlet: HTMLElement
  readonly nav: NavModel
  readonly user: UserModel
  readonly locale: string
  readonly theme: ThemeModel
  readonly notifications: NotificationsModel
  logout(): Promise<void>
}

export interface Shell extends ClientValue<'shell'> {
  readonly component: Component<ShellProps>
}

/** Replaces the whole authenticated shell around every page. */
export function shellRenderer(component: Component<ShellProps>, opts: { id?: string } = {}): Shell {
  const id = opts.id ?? ''
  return { ...contract({ id, kind: 'shell' }), component, ...(opts.id ? {} : { implicitId: 'shell' as const }) }
}

// ---- transport ----

export interface RemoteErrorBody {
  readonly kind: 'domain' | 'validation' | 'denied' | 'internal'
  readonly code: string
  readonly message?: string
  readonly params?: Json
  readonly issues?: readonly {
    readonly code: string
    readonly phase: 'input' | 'write'
    readonly source: string
    readonly path: readonly string[]
    readonly params: Json
  }[]
}

export class RemoteError extends Error {
  readonly body: RemoteErrorBody

  constructor(body: RemoteErrorBody) {
    super(body.message ?? body.code)
    this.name = 'RemoteError'
    this.body = body
  }
}

/** Provided by the client host; carries wire JSON only. */
export interface ClientTransport {
  invoke(kind: Extract<ImplKind, 'query' | 'command' | 'atomic'>, resource: string, input: Json): Promise<Json>
}

export interface ApiClient {
  query<Q extends Query>(query: Q, input: InputOf<Q>): Promise<Infer<Q['output']>>
  command<C extends Command | AtomicCommand>(command: C, input: InputOf<C>): Promise<ResultOf<C>>
}

/** Typed client over a transport: validates input and output against the contract schemas. */
export function createClient(transport: ClientTransport): ApiClient {
  async function query<Q extends Query>(q: Q, input: InputOf<Q>): Promise<Infer<Q['output']>> {
    const raw = await transport.invoke('query', q.id, encode(q.input, input))
    // decode validates against q.output, so the value has the contract's output type
    return decode(q.output, raw) as Infer<Q['output']>
  }
  async function command<C extends Command | AtomicCommand>(c: C, input: InputOf<C>): Promise<ResultOf<C>> {
    const raw = await transport.invoke(c.atomic ? 'atomic' : 'command', c.id, encode(c.input, input))
    const output: AnySchema | undefined = c.output
    // output is undefined exactly when ResultOf<C> is void
    return (output === undefined ? undefined : decode(output, raw)) as ResultOf<C>
  }
  return { query, command }
}

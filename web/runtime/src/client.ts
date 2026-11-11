// Client graph: pages, navigation, slots, cell renderers and the shell. Declarations and branded
// values only; the canonical client host mounts them. Server values must never be imported here
// (the build slice rejects that edge; it cannot be expressed in types).
import type { Component } from 'solid-js'
import type {
  AtomicCommand, Column, Command, DomainError, InputOf, Issue, PathParamNames, Permission, Query, ResultOf, Route, SlotRef,
} from './contracts.ts'
import type { ImplKind, Json, Label, NavGroupJSON, NavPatchJSON, PaletteEntryJSON, ResourceJSON, WorkspaceJSON } from './model.ts'
import { contract, omitEmpty, type Contract } from './resource.ts'
import type { AnySchema, Infer } from './schema.ts'
import { decode, encode, validate } from './wire.ts'

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

/** Props of a page declared by path: each `:name` segment is a string. */
export type PathProps<Path extends string> = { readonly [K in PathParamNames<Path>]: string }

/**
 * A page bound to a route (typed params) or declared with its own ID and path.
 * - Own route: the page ID is `<route id>.page`.
 * - Route of the platform or another package (from the kit): a screen replacement. The page gets an ID in this
 *   package (`<namespace>.<export name>`), `route` names the replaced route, and it is only a candidate: it becomes
 *   active through an explicit `screen` binding at activation.
 */
export function page<R extends Route>(route: R, spec: PageSpec<Infer<R['params']>>): Page<Infer<R['params']>>
export function page<const Path extends string>(id: string, spec: PageSpec<PathProps<Path>> & { path: Path }): Page<PathProps<Path>>
export function page(
  target: string | Route,
  spec: PageSpec<never> & { path?: string },
): Page<never> | Page {
  const isRoute = typeof target !== 'string'
  const replacement = isRoute && target.foreign
  const id = isRoute ? (replacement ? '' : `${target.id}.page`) : target
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
  return {
    ...contract({ ...resource, id, kind: 'page' }),
    component: spec.component as Component<PageProps<never>>,
    ...(replacement ? { implicitId: 'export' as const } : {}),
  }
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

/** A contribution built by `slot()`. */
export interface SlotValue<P extends object = Record<string, Json>> extends ClientValue<'slot'> {
  readonly component: Component<P>
}

export interface SlotOptions {
  /** Explicit ID; default `<namespace>.<export name>`. */
  id?: string
  /** Position among the contributions of the slot, ascending; ties break by package, then ID. Default 0. */
  order?: number
  /** Only viewers holding it get this contribution (checked on the server, on top of the slot's own permission). */
  permission?: Permission
}

/**
 * Contributes a component to a slot (`target` is a typed `SlotRef` from the kit or `defineSlot`, or a slot ID).
 * A contribution is additive: it needs no binding, it is active while its package version is, and every
 * contribution of every active package is mounted, in `order`. The component receives the host page's props.
 */
export function slot<P extends object>(target: SlotRef<P> | string, component: Component<P>, opts: SlotOptions = {}): SlotValue<P> {
  const targetId = typeof target === 'string' ? target : target.slotId
  const id = opts.id ?? ''
  const resource = omitEmpty({ id, kind: 'slot' as const, target: targetId, order: opts.order, permission: opts.permission?.id }) as ResourceJSON
  return { ...contract({ ...resource, id, kind: 'slot' }), component, ...(opts.id ? {} : { implicitId: 'export' as const }) }
}

export interface CellProps<V = Json> {
  /** The column's value for this row (wire JSON: int64 and decimal are strings), `null` when absent. */
  readonly value: V
  /**
   * The row's contributed values by column ID (only columns the viewer may read) and `ref`, the native record
   * (`{ entity, key }`).
   */
  readonly row: Readonly<Record<string, Json>>
}

export interface Cell<V = Json> extends ClientValue<'cell'> {
  readonly component: Component<CellProps<V>>
}

/**
 * Custom renderer of a `column(...)` in the native list. The list stays server-rendered (the standard cell is the
 * fallback until this module loads, or if it fails); filters, sorting and exports never use it: they read the raw
 * value.
 */
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

export { RuntimeProvider, useCommand, useQuery } from './client-host/hooks.ts'
export { Slot } from './client-host/slots.ts'
export type { SlotProps } from './client-host/slots.ts'
export type { CallError, CommandResult, QueryResult } from './client-host/hooks.ts'
export { TransportError, UnauthenticatedError } from './client-host/transport.ts'

/** A remote error narrowed to a declared domain error: `params` has the contract's type. */
export interface DomainRemoteError<Id extends string, P> extends RemoteError {
  readonly body: RemoteErrorBody & { readonly kind: 'domain'; readonly code: Id; readonly params: P }
}

/** One validation issue narrowed to a declared issue contract. */
export interface TypedRemoteIssue<Id extends string, P> {
  readonly code: Id
  readonly phase: 'input' | 'write'
  readonly source: string
  readonly path: readonly string[]
  readonly params: P
}

/**
 * `if (isError(err, NotActive)) err.body.params.status`: true when `err` is that declared domain error
 * (and its params match the contract schema).
 */
export function isError<Id extends string, P extends AnySchema>(err: unknown, error: DomainError<Id, P>): err is DomainRemoteError<Id, Infer<P>> {
  if (!(err instanceof RemoteError) || err.body.kind !== 'domain' || err.body.code !== error.id) return false
  try {
    validate(error.params, err.body.params)
    return true
  } catch {
    return false
  }
}

/** The validation issues of `err` that are `issue`, with typed params (other issues and malformed params are skipped). */
export function issuesOf<Id extends string, P extends AnySchema>(err: unknown, issue: Issue<Id, P>): readonly TypedRemoteIssue<Id, Infer<P>>[] {
  if (!(err instanceof RemoteError) || err.body.kind !== 'validation') return []
  const out: TypedRemoteIssue<Id, Infer<P>>[] = []
  for (const i of err.body.issues ?? []) {
    if (i.code !== issue.id) continue
    try {
      validate(issue.params, i.params)
    } catch {
      continue
    }
    out.push({ code: issue.id, phase: i.phase, source: i.source, path: i.path, params: i.params as Infer<P> })
  }
  return out
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

import type { Action, Frame, Level, Panel, Source } from '../contract'

export interface LeafActionContext {
  fields: Readonly<Record<string, unknown>>
  variables: Readonly<Record<string, unknown>>
  location: URL
}

export function recordForRow(frame: Frame, row: Array<unknown>): Record<string, unknown> {
  return Object.fromEntries(frame.columns.map((column, index) => [column.name, row[index]]))
}

export function variablesFromLocation(location: URL): Record<string, unknown> {
  const variables: Record<string, unknown> = {}
  for (const name of new Set(location.searchParams.keys())) {
    const values = location.searchParams.getAll(name)
    variables[name] = values.length > 1 ? values : values[0]
  }
  return variables
}

function sourceValue(source: Source, context: LeafActionContext): unknown {
  let value: unknown
  if (source.kind === 'literal') value = source.value
  if (source.kind === 'field' && source.name) value = context.fields[source.name]
  if (source.kind === 'variable' && source.name) value = context.variables[source.name]
  return value ?? source.fallback
}

function withPreservedQuery(target: URL, current: URL): void {
  for (const [name, value] of current.searchParams) {
    if (!target.searchParams.has(name)) target.searchParams.append(name, value)
  }
}

function parameterText(value: unknown): string | undefined {
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') return String(value)
  return JSON.stringify(value)
}

function resolveTemplate(action: Action, context: LeafActionContext): string | undefined {
  if (!action.urlTemplate) return undefined
  let resolved = action.urlTemplate
  for (const param of action.params) {
    const value = sourceValue(param.source, context)
    if (value === undefined || value === null) return undefined
    const text = parameterText(value)
    if (text === undefined) return undefined
    resolved = resolved.replaceAll(`{${param.name}}`, encodeURIComponent(text))
  }
  if (/\{[^}]+\}/.test(resolved)) return undefined
  return resolved
}

export function resolveLeafActionURL(action: Action, context: LeafActionContext): string | undefined {
  if (action.kind !== 'navigate_to_leaf' && action.kind !== 'open_drawer') return undefined
  return resolveActionURL(action, context)
}

/**
 * Builds an action's URL regardless of whether it navigates to a leaf record
 * or to another dashboard view: panel-level `navigate` actions resolve exactly
 * the same way, they just belong to a card instead of a row.
 */
export function resolveActionURL(action: Action, context: LeafActionContext): string | undefined {
  if (action.kind !== 'navigate' && action.kind !== 'navigate_to_leaf' && action.kind !== 'open_drawer') return undefined
  let resolved: string | undefined
  if (action.urlSource) {
    const value = sourceValue(action.urlSource, context)
    if (value === undefined || value === null) return undefined
    resolved = typeof value === 'string' ? value : parameterText(value)
  } else {
    resolved = resolveTemplate(action, context)
  }
  if (resolved === undefined) return undefined
  // An empty (or whitespace-only) field/template value is not a destination: it
  // means this row/segment is inert. Without this guard `new URL('', location)`
  // resolves to the current page, so an OpenDrawer bound to an empty action_url
  // would try to open the dashboard page itself as a Lens document (the drawer
  // then shows "not valid JSON" because it fetched HTML).
  if (resolved.trim() === '') return undefined

  let target: URL
  try {
    target = new URL(resolved, context.location)
  } catch {
    return undefined
  }
  if (target.origin !== context.location.origin) return undefined
  if (action.preserveQuery) withPreservedQuery(target, context.location)
  return target.href
}

export function resolveColumnActionURL(
  action: Action,
  frame: Frame,
  row: Array<unknown>,
  location: URL,
): string | undefined {
  return resolveLeafActionURL(action, {
    fields: recordForRow(frame, row),
    variables: variablesFromLocation(location),
    location,
  })
}

function matchingNode(level: Level, panel: Panel, frame: Frame, row: Array<unknown>) {
  const idField = level.encoding?.id ?? panel.encoding.id
  const idIndex = idField ? frame.columns.findIndex(({ name }) => name === idField) : -1
  const id = idIndex >= 0 ? String(row[idIndex]) : undefined
  return level.children.find(({ key }) => key === id || Boolean(id && key.endsWith(`/${id}`)))
}

export function resolveRowLeafActionURL(
  panel: Panel,
  frame: Frame,
  row: Array<unknown>,
  location: URL,
  level?: Level,
): string | undefined {
  const node = level ? matchingNode(level, panel, frame, row) : undefined
  const action = rowLeafAction(panel, level, node)
  return action ? resolveLeafActionURL(action, {
    fields: recordForRow(frame, row),
    variables: variablesFromLocation(location),
    location,
  }) : undefined
}

export function rowLeafAction(panel: Panel, level?: Level, node?: Level['children'][number]): Action | undefined {
  return node?.action ?? (node || !level?.children.length
    ? panel.actions.find(({ kind }) => kind === 'navigate_to_leaf' || kind === 'open_drawer')
    : undefined)
}

export function actionForRow(panel: Panel, frame: Frame, row: Array<unknown>, level?: Level): Action | undefined {
  return rowLeafAction(panel, level, level ? matchingNode(level, panel, frame, row) : undefined)
}

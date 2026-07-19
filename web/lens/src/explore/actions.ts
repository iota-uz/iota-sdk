import type { Action, Source } from '../contract'

export interface LeafActionContext {
  fields: Readonly<Record<string, unknown>>
  variables: Readonly<Record<string, unknown>>
  location: URL
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

export function resolveLeafActionURL(action: Action, context: LeafActionContext): string | undefined {
  if (action.kind !== 'navigate_to_leaf' || !action.urlTemplate) return undefined
  let resolved = action.urlTemplate
  for (const param of action.params) {
    const value = sourceValue(param.source, context)
    if (value === undefined || value === null) return undefined
    const text = parameterText(value)
    if (text === undefined) return undefined
    resolved = resolved.replaceAll(`{${param.name}}`, encodeURIComponent(text))
  }
  if (/\{[^}]+\}/.test(resolved)) return undefined

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

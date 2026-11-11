// Package client bundles must not carry their own copy of Solid or of the
// runtime client: reactivity and the runtime context only work with the
// instances the host renders with. The build slice externalizes these specifiers
// and resolves them, in the browser, from the registry published here.
import * as solid from 'solid-js'
import * as solidWeb from 'solid-js/web'
import * as solidStore from 'solid-js/store'
import * as author from '../client.ts'
import * as hooks from './hooks.ts'
import * as transport from './transport.ts'

/** `globalThis[SHARED_MODULES]` maps an external specifier to the host's module namespace. */
export const SHARED_MODULES = Symbol.for('iota.runtime.shared')

export type SharedModules = Readonly<Record<'solid-js' | 'solid-js/web' | 'solid-js/store' | '@iota-uz/sdk/runtime/client', object>>

/** Publishes the host's instances for package bundles. Idempotent. */
export function publishSharedModules(target: object = globalThis): SharedModules {
  const existing = (target as Record<symbol, SharedModules | undefined>)[SHARED_MODULES]
  if (existing) return existing
  const modules: SharedModules = Object.freeze({
    'solid-js': solid,
    'solid-js/web': solidWeb,
    'solid-js/store': solidStore,
    '@iota-uz/sdk/runtime/client': Object.freeze({ ...author, ...hooks, ...transport }),
  })
  Object.defineProperty(target, SHARED_MODULES, { value: modules, enumerable: false })
  return modules
}

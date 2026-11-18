// `@iota-uz/sdk/runtime/kit` — INTERNAL entry used only by generated `@iota-kit/*` modules.
//
// A package author never imports this. The developer kit renders platform and other-package contracts with
// these builders so they are marked `foreign`: referenced and implemented by this package, never emitted into
// its manifest. Everything the author-facing `@iota-uz/sdk/runtime` entry builds is owned by the package, and
// `collectManifest` rejects any resource outside the package namespace that did not come through here.
// Go re-validates every foreign reference against the catalog snapshot at publish time.
import { KIT, type Contract } from './resource.ts'

export { nativeEntity } from './entity.ts'
export { nativeList } from './contracts.ts'

/** Marks a contract as declared elsewhere (kit-generated refs): it is referenced, never emitted into this manifest. */
export function foreign<C extends Contract>(c: C): C {
  return { ...c, foreign: true, [KIT]: true }
}

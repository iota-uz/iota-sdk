// The SDK's Solid UI kit as the host shares it with package client bundles (`@iota-uz/sdk/solid-ui`). It is a lazy
// chunk of the host bundle: nothing here is evaluated, downloaded or styled until a package version that imports the
// kit loads (shared.ts holds the loader; the package bundle's shim awaits it). Because the host owns the instance, the kit
// shares the host's Solid and theme with the package's components, and it is always the version of the host's release.
import * as kitModule from '@iota-uz/sdk/solid-ui'
import kitCss from '@iota-uz/sdk/solid-ui/embedded-css'

/** Marks the style element holding the kit's stylesheet. */
export const KIT_STYLE_ATTR = 'data-iota-kit-css'

/**
 * Adds the kit's utilities to the document once. They come inside the same chunk as the kit (no second request, never a
 * stylesheet of another release), as a constructed sheet when the browser has them (they need no inline-style allowance
 * from a content security policy) and as a style element otherwise. Everything the kit does not carry itself (the SDK tokens,
 * its component classes) is the host page's own stylesheet.
 */
export function installKitStyles(doc: Document = document): void {
  const sheets = doc as Document & { adoptedStyleSheets?: CSSStyleSheet[] }
  if (doc.querySelector(`style[${KIT_STYLE_ATTR}]`) || (sheets.adoptedStyleSheets ?? []).some((s) => (s as CSSStyleSheet & { iotaKit?: boolean }).iotaKit)) return
  const Sheet = doc.defaultView?.CSSStyleSheet
  if (typeof Sheet === 'function' && 'adoptedStyleSheets' in doc) {
    try {
      const sheet = new Sheet() as CSSStyleSheet & { iotaKit?: boolean }
      sheet.replaceSync(kitCss)
      sheet.iotaKit = true
      sheets.adoptedStyleSheets = [...(sheets.adoptedStyleSheets ?? []), sheet]
      return
    } catch {
      // fall back to a style element
    }
  }
  const style = doc.createElement('style')
  style.setAttribute(KIT_STYLE_ATTR, '')
  style.textContent = kitCss
  doc.head.append(style)
}

export const kit: object = kitModule

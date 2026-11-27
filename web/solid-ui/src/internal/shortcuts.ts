// Shortcut ownership: an embedded surface (an editor, a canvas) claims keyboard shortcuts with
// `data-claim-shortcuts="mod+k mod+z"` on its root; page-wide listeners (global search) skip a claimed
// shortcut. A claim holds for events whose target is inside the claiming element, and for events with no
// focused target (the body) while a claiming element is on the page. An event a handler already took
// (defaultPrevented) is never handled twice either. The same rule is in modules/core/.../alpine.js.

/** `mod` is ⌘ or Ctrl; combos are lowercase, `+`-joined, e.g. `mod+shift+l`. */
export function isShortcutClaimed(event: KeyboardEvent, combo: string): boolean {
  if (event.defaultPrevented) return true
  const selector = `[data-claim-shortcuts~="${combo}"]`
  const target = event.target
  const doc = target instanceof Node ? target.ownerDocument ?? (target as Document) : typeof document === 'undefined' ? null : document
  if (!doc) return false
  if (target === null || target === doc || target === doc.body || target === doc.documentElement) return doc.querySelector(selector) !== null
  return target instanceof Element && target.closest(selector) !== null
}

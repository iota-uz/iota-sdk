// The kit's embedded sheet is a second Tailwind build on a page that already has the host's. Both put their utilities in
// `@layer utilities`, so rules of equal specificity are decided by order, and the kit cannot control its order: the runtime
// host adopts it as a constructed sheet (which always cascades after the document's sheets) and Lens injects it at run time.
// A plain kit `.hidden` placed after the host's `@media (min-width:64rem){.lg\:block{…}}` hid the host's desktop sidebar.
//
// Zero specificity settles it independently of order: every kit utility selector becomes `:where(…)`, so any host utility
// wins wherever both define the class, and the kit still styles what only it defines (nothing else in the utilities layer
// competes with it; the host's base and components layers lose to utilities by layer, not specificity). A pseudo-element
// stays outside, because `:where()` cannot hold one.
import postcss from 'postcss'

/** Splits `.a:hover::before` into `.a:hover` and `::before`; a selector without a pseudo-element keeps an empty tail. */
function splitPseudoElement(selector) {
  let depth = 0
  for (let i = 0; i < selector.length; i++) {
    const ch = selector[i]
    if (ch === '\\') {
      i++
      continue
    }
    if (ch === '(' || ch === '[') depth++
    else if (ch === ')' || ch === ']') depth--
    else if (depth === 0 && ch === ':' && selector[i + 1] === ':') return [selector.slice(0, i), selector.slice(i)]
  }
  return [selector, '']
}

/** `:where(…)` for one selector of a kit utility rule. */
export function zeroSpecificity(selector) {
  const trimmed = selector.trim()
  if (trimmed.startsWith(':where(')) return trimmed
  const [compound, pseudoElement] = splitPseudoElement(trimmed)
  return compound ? `:where(${compound})${pseudoElement}` : trimmed
}

function insideUtilities(rule) {
  for (let node = rule.parent; node; node = node.parent) {
    if (node.type === 'atrule' && node.name === 'layer' && node.params.trim() === 'utilities') return true
  }
  return false
}

/** The kit sheet with every selector in its utilities layer at zero specificity. */
export function lowerKitUtilities(css) {
  const root = postcss.parse(css)
  root.walkRules((rule) => {
    if (!insideUtilities(rule)) return
    // A rule nested in another rule (Tailwind's `&` variants) inherits its parent's selector.
    if (rule.parent?.type === 'rule') return
    rule.selectors = rule.selectors.map(zeroSpecificity)
  })
  return root.toString()
}

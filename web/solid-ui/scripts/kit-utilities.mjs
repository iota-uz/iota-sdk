// The kit's embedded sheet is a second Tailwind build on a page that already has the host's. Both put their utilities in
// `@layer utilities`, so rules of equal specificity are decided by order, and the kit cannot control its order: the runtime
// host adopts it as a constructed sheet (which always cascades after the document's sheets) and Lens injects it at run time.
// A plain kit `.hidden` placed after the host's `@media (min-width:64rem){.lg\:block{…}}` hid the host's desktop sidebar.
//
// The kit's *base* utilities (`.hidden`, `.flex`, `.p-4`) go to zero specificity (`:where(…)`): a host variant such as
// `lg:block` then beats them whatever the order, and a host base utility of the same name carries the same declarations.
// Variant utilities (`lg:block`, `hover:bg-…`, `dark:…`, recognisable by the escaped colon in the class name) keep their
// specificity, so a variant only the kit defines still overrides a base utility the host defines, as Tailwind intends.
// A pseudo-element stays outside `:where()`, which cannot hold one.
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

/**
 * A variant utility: its leading class name carries a variant prefix (`lg\:block`, `hover\:bg-x`), an escaped colon
 * outside escaped brackets — `[--gap:4px]` is an arbitrary property, not a variant.
 */
export function isVariant(selector) {
  const trimmed = selector.trim()
  if (!trimmed.startsWith('.')) return false
  let depth = 0
  for (let i = 1; i < trimmed.length; i++) {
    const ch = trimmed[i]
    if (ch === '\\') {
      const next = trimmed[i + 1]
      if (next === '[') depth++
      else if (next === ']') depth--
      else if (next === ':' && depth === 0) return true
      i++
      continue
    }
    // The class name ends at the first unescaped structural character.
    if ('.:#[>+~,() '.includes(ch)) return false
  }
  return false
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

/** The kit sheet with every base utility selector of its utilities layer at zero specificity. */
export function lowerKitUtilities(css) {
  const root = postcss.parse(css)
  root.walkRules((rule) => {
    if (!insideUtilities(rule)) return
    // A rule nested in another rule (Tailwind's `&` variants) inherits its parent's selector.
    if (rule.parent?.type === 'rule') return
    rule.selectors = rule.selectors.map((selector) => (isVariant(selector) ? selector : zeroSpecificity(selector)))
  })
  return root.toString()
}

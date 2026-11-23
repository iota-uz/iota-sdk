/** `:where(…)` for one selector of a kit utility rule; a pseudo-element stays outside. */
export function zeroSpecificity(selector: string): string
/** A variant utility selector (`lg\:block`, `hover\:bg-x`): its leading class carries a variant prefix. */
export function isVariant(selector: string): boolean
/** The kit sheet with every base utility selector of its utilities layer at zero specificity. */
export function lowerKitUtilities(css: string): string

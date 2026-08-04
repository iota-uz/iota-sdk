export type LensThemeMode = 'light' | 'dark'

/** An explicit light/dark theme wins; otherwise the root dark class is used. */
export function normalizeLensTheme(value: string | null | undefined, hasDarkClass = false): LensThemeMode {
  if (value === 'dark' || value === 'light') return value
  return hasDarkClass ? 'dark' : 'light'
}

export type LensThemeMode = 'light' | 'dark'

/** What a host declares: a fixed mode, or `auto` to follow the host application. */
export type LensThemeSetting = LensThemeMode | 'auto'

/** An explicit light/dark theme wins; otherwise the root dark class is used. */
export function normalizeLensTheme(value: string | null | undefined, hasDarkClass = false): LensThemeMode {
  if (value === 'dark' || value === 'light') return value
  return hasDarkClass ? 'dark' : 'light'
}

/** Reads a theme attribute: `auto` stays `auto`, anything unknown is light. */
export function parseLensThemeSetting(value: string | null | undefined): LensThemeSetting {
  return value === 'auto' ? 'auto' : normalizeLensTheme(value)
}

const darkScheme = '(prefers-color-scheme: dark)'

/**
 * The theme the host application is showing right now. The `dark` class on
 * `<html>` is the application's own switch and wins; an explicit `light` class
 * is the same declaration the other way. A page that declares neither follows
 * the operating system.
 */
export function detectAppTheme(
  root: HTMLElement | undefined = typeof document === 'undefined' ? undefined : document.documentElement,
  media: Pick<MediaQueryList, 'matches'> | undefined = typeof window === 'undefined' || !window.matchMedia
    ? undefined
    : window.matchMedia(darkScheme),
): LensThemeMode {
  if (root?.classList.contains('dark')) return 'dark'
  if (root?.classList.contains('light')) return 'light'
  return media?.matches ? 'dark' : 'light'
}

/**
 * Calls `onChange` with the application's theme whenever its `<html>` class or
 * the system colour scheme changes. Returns the disposer.
 */
export function observeAppTheme(onChange: (theme: LensThemeMode) => void): () => void {
  if (typeof document === 'undefined') return () => undefined
  const root = document.documentElement
  const media = typeof window !== 'undefined' && window.matchMedia ? window.matchMedia(darkScheme) : undefined
  const notify = () => onChange(detectAppTheme(root, media))
  const observer = typeof MutationObserver === 'undefined' ? undefined : new MutationObserver(notify)
  observer?.observe(root, { attributes: true, attributeFilter: ['class'] })
  media?.addEventListener?.('change', notify)
  return () => {
    observer?.disconnect()
    media?.removeEventListener?.('change', notify)
  }
}

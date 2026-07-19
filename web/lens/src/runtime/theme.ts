export type LensThemeMode = 'light' | 'dark'

export function normalizeLensTheme(value: string | null | undefined): LensThemeMode {
  return value === 'dark' ? 'dark' : 'light'
}

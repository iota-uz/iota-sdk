import type { Theme } from '../../contract'
import { normalizeLensTheme, type LensThemeMode } from '../../runtime/theme'

const fallbackSeries = ['#2563eb', '#059669', '#d97706', '#7c3aed', '#0891b2', '#dc2626']

export interface EChartsTheme {
  mode: LensThemeMode
  background: string
  card: string
  text: string
  mutedText: string
  border: string
  divider: string
  accent: string
  selectedBorder: string
  fontFamily: string
  colors: string[]
  seriesColor(name: string): string | undefined
}

function css(styles: CSSStyleDeclaration, name: string, fallback: string): string {
  return styles.getPropertyValue(name).trim() || fallback
}

function resolvePaletteColor(value: string | undefined, theme: Theme): string | undefined {
  if (!value) return undefined
  return theme.palette[value] ?? value
}

export function buildEChartsTheme(element: HTMLElement, theme: Theme): EChartsTheme {
  const root = element.closest<HTMLElement>('.lens-root') ?? element
  const styles = getComputedStyle(root)
  const mode = normalizeLensTheme(root.dataset.theme)
  const configuredColors = Object.values(theme.palette).filter((color) => color.trim() !== '')
  const colors = configuredColors.length > 0 ? configuredColors : fallbackSeries

  return {
    mode,
    background: css(styles, '--lens-bg-page', mode === 'dark' ? '#0f172a' : '#f6f7f9'),
    card: css(styles, '--lens-bg-card', mode === 'dark' ? '#1e293b' : '#ffffff'),
    text: css(styles, '--lens-text', mode === 'dark' ? '#e2e8f0' : '#334155'),
    mutedText: css(styles, '--lens-text-muted', mode === 'dark' ? '#cbd5e1' : '#64748b'),
    border: css(styles, '--lens-border', mode === 'dark' ? '#475569' : '#e2e8f0'),
    divider: css(styles, '--lens-divider', mode === 'dark' ? '#334155' : '#f1f5f9'),
    accent: css(styles, '--lens-accent-500', '#2563eb'),
    selectedBorder: css(styles, '--lens-text-strong', mode === 'dark' ? '#f8fafc' : '#0f172a'),
    fontFamily: css(styles, '--lens-font', 'Inter, ui-sans-serif, system-ui, sans-serif'),
    colors,
    seriesColor(name: string) {
      return resolvePaletteColor(theme.series[name], theme)
    },
  }
}

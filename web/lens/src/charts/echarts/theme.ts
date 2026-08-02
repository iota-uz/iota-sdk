import type { Theme } from '../../contract'
import { normalizeLensTheme } from '../../runtime/theme'
import { fallbackSeries } from '../palette'

export interface EChartsTheme {
  card: string
  text: string
  mutedText: string
  faintText: string
  border: string
  divider: string
  selectedBorder: string
  fontFamily: string
  /** The ink a stated threshold is drawn in, and the wash behind its chip. */
  warn: string
  warnSoft: string
  /** The ink an annotated event is marked with on the axis. */
  accent: string
  /** The ink a fitted line is drawn in — never a data series' own hue. */
  trend: string
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
  const mode = normalizeLensTheme(root.dataset.theme, root.classList.contains('dark'))
  const configuredColors = Object.values(theme.palette).filter((color) => color.trim() !== '')
  const colors = configuredColors.length > 0 ? configuredColors : [...fallbackSeries]

  return {
    card: css(styles, '--lens-bg-card', mode === 'dark' ? '#1e293b' : '#ffffff'),
    text: css(styles, '--lens-text', mode === 'dark' ? '#e2e8f0' : '#334155'),
    mutedText: css(styles, '--lens-text-muted', mode === 'dark' ? '#cbd5e1' : '#64748b'),
    faintText: css(styles, '--lens-text-faint', '#94a3b8'),
    border: css(styles, '--lens-border', mode === 'dark' ? '#475569' : '#e2e8f0'),
    divider: css(styles, '--lens-divider', mode === 'dark' ? '#334155' : '#f1f5f9'),
    selectedBorder: css(styles, '--lens-text-strong', mode === 'dark' ? '#f8fafc' : '#0f172a'),
    fontFamily: css(styles, '--lens-font', 'Inter, ui-sans-serif, system-ui, sans-serif'),
    warn: css(styles, '--lens-warn', mode === 'dark' ? '#f59e0b' : '#d97706'),
    warnSoft: css(styles, '--lens-warn-soft', mode === 'dark' ? 'rgba(217, 119, 6, 0.16)' : '#fffbeb'),
    accent: css(styles, '--lens-accent-500', mode === 'dark' ? '#60a5fa' : '#2563eb'),
    trend: css(styles, '--lens-overlay-trend', mode === 'dark' ? '#c4b5fd' : '#7c3aed'),
    colors,
    seriesColor(name: string) {
      return resolvePaletteColor(theme.series[name], theme)
    },
  }
}

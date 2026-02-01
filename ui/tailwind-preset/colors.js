/**
 * Design Token Colors for IOTA SDK
 *
 * All colors use CSS variables for runtime theming support.
 * This enables:
 * - Dark mode via CSS variable swap
 * - Tenant-specific branding via CSS variable override
 * - Consistent styling between Templ pages and React applets
 *
 * The SDK defines these CSS variables in its global stylesheet.
 * Applets consume the same variables for unified theming.
 */

module.exports = {
  // Primary brand colors
  primary: {
    DEFAULT: 'var(--color-primary)',
    50: 'var(--color-primary-50)',
    100: 'var(--color-primary-100)',
    200: 'var(--color-primary-200)',
    300: 'var(--color-primary-300)',
    400: 'var(--color-primary-400)',
    500: 'var(--color-primary-500)',
    600: 'var(--color-primary-600)',
    700: 'var(--color-primary-700)',
    800: 'var(--color-primary-800)',
    900: 'var(--color-primary-900)',
  },

  // Semantic colors
  background: 'var(--color-background)',
  foreground: 'var(--color-foreground)',
  text: {
    DEFAULT: 'var(--color-text)',
    muted: 'var(--color-text-muted)',
    inverse: 'var(--color-text-inverse)',
  },

  // UI state colors
  success: {
    DEFAULT: 'var(--color-success)',
    light: 'var(--color-success-light)',
    dark: 'var(--color-success-dark)',
  },
  error: {
    DEFAULT: 'var(--color-error)',
    light: 'var(--color-error-light)',
    dark: 'var(--color-error-dark)',
  },
  warning: {
    DEFAULT: 'var(--color-warning)',
    light: 'var(--color-warning-light)',
    dark: 'var(--color-warning-dark)',
  },
  info: {
    DEFAULT: 'var(--color-info)',
    light: 'var(--color-info-light)',
    dark: 'var(--color-info-dark)',
  },

  // Border and divider colors
  border: {
    DEFAULT: 'var(--color-border)',
    light: 'var(--color-border-light)',
    dark: 'var(--color-border-dark)',
  },

  // Surface colors (cards, panels)
  surface: {
    DEFAULT: 'var(--color-surface)',
    elevated: 'var(--color-surface-elevated)',
    overlay: 'var(--color-surface-overlay)',
  },

  // Neutral grays (fallback to Tailwind defaults if CSS vars not defined)
  gray: {
    50: 'var(--color-gray-50, #f9fafb)',
    100: 'var(--color-gray-100, #f3f4f6)',
    200: 'var(--color-gray-200, #e5e7eb)',
    300: 'var(--color-gray-300, #d1d5db)',
    400: 'var(--color-gray-400, #9ca3af)',
    500: 'var(--color-gray-500, #6b7280)',
    600: 'var(--color-gray-600, #4b5563)',
    700: 'var(--color-gray-700, #374151)',
    800: 'var(--color-gray-800, #1f2937)',
    900: 'var(--color-gray-900, #111827)',
  },
}

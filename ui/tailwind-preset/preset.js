/**
 * IOTA SDK Tailwind CSS Preset
 *
 * Shared Tailwind configuration for all IOTA SDK applets.
 * Provides unified design tokens, typography, and styling.
 *
 * Usage:
 * ```js
 * // In applet's tailwind.config.js
 * const iotaPreset = require('@iotauz/tailwind-preset')
 *
 * module.exports = {
 *   presets: [iotaPreset],
 *   content: ['./src/**\/*.{ts,tsx}'],
 *   // Applet-specific overrides if needed
 * }
 * ```
 */

const colors = require('./colors')
const typography = require('./typography')

module.exports = {
  theme: {
    extend: {
      // Design token colors (CSS variable-based for theming)
      colors,

      // Typography scales
      fontFamily: typography.fontFamily,
      fontSize: typography.fontSize,
      fontWeight: typography.fontWeight,
      lineHeight: typography.lineHeight,
      letterSpacing: typography.letterSpacing,

      // Spacing scale (extends Tailwind defaults)
      spacing: {
        18: '4.5rem',   // 72px
        88: '22rem',    // 352px
        112: '28rem',   // 448px
        128: '32rem',   // 512px
      },

      // Border radius
      borderRadius: {
        '4xl': '2rem',  // 32px
      },

      // Box shadows
      boxShadow: {
        'soft': '0 2px 8px rgba(0, 0, 0, 0.08)',
        'medium': '0 4px 16px rgba(0, 0, 0, 0.12)',
        'hard': '0 8px 32px rgba(0, 0, 0, 0.16)',
      },

      // Animation durations
      transitionDuration: {
        '2000': '2000ms',
        '3000': '3000ms',
      },

      // Z-index layers
      zIndex: {
        'dropdown': '1000',
        'sticky': '1020',
        'fixed': '1030',
        'modal-backdrop': '1040',
        'modal': '1050',
        'popover': '1060',
        'tooltip': '1070',
      },
    },
  },
  plugins: [
    require('@tailwindcss/forms'),
    require('@tailwindcss/typography'),
  ],
}

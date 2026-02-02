/**
 * Typography Design Tokens for IOTA SDK
 *
 * Font scales, line heights, and spacing that match the SDK's design system.
 * Provides consistent typography across all applets.
 */

module.exports = {
  // Font families
  fontFamily: {
    sans: [
      'Inter',
      'system-ui',
      '-apple-system',
      'BlinkMacSystemFont',
      'Segoe UI',
      'Roboto',
      'Helvetica Neue',
      'Arial',
      'sans-serif',
    ],
    mono: [
      'JetBrains Mono',
      'Fira Code',
      'ui-monospace',
      'SFMono-Regular',
      'Menlo',
      'Monaco',
      'Consolas',
      'Liberation Mono',
      'Courier New',
      'monospace',
    ],
  },

  // Font sizes with matching line heights
  fontSize: {
    xs: ['0.75rem', { lineHeight: '1rem' }],       // 12px / 16px
    sm: ['0.875rem', { lineHeight: '1.25rem' }],   // 14px / 20px
    base: ['1rem', { lineHeight: '1.5rem' }],      // 16px / 24px
    lg: ['1.125rem', { lineHeight: '1.75rem' }],   // 18px / 28px
    xl: ['1.25rem', { lineHeight: '1.75rem' }],    // 20px / 28px
    '2xl': ['1.5rem', { lineHeight: '2rem' }],     // 24px / 32px
    '3xl': ['1.875rem', { lineHeight: '2.25rem' }], // 30px / 36px
    '4xl': ['2.25rem', { lineHeight: '2.5rem' }],  // 36px / 40px
    '5xl': ['3rem', { lineHeight: '1' }],          // 48px / 48px
    '6xl': ['3.75rem', { lineHeight: '1' }],       // 60px / 60px
    '7xl': ['4.5rem', { lineHeight: '1' }],        // 72px / 72px
    '8xl': ['6rem', { lineHeight: '1' }],          // 96px / 96px
    '9xl': ['8rem', { lineHeight: '1' }],          // 128px / 128px
  },

  // Font weights
  fontWeight: {
    thin: '100',
    extralight: '200',
    light: '300',
    normal: '400',
    medium: '500',
    semibold: '600',
    bold: '700',
    extrabold: '800',
    black: '900',
  },

  // Line heights
  lineHeight: {
    none: '1',
    tight: '1.25',
    snug: '1.375',
    normal: '1.5',
    relaxed: '1.625',
    loose: '2',
  },

  // Letter spacing
  letterSpacing: {
    tighter: '-0.05em',
    tight: '-0.025em',
    normal: '0em',
    wide: '0.025em',
    wider: '0.05em',
    widest: '0.1em',
  },
}

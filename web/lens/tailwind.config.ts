import type { Config } from 'tailwindcss'

export default {
  content: ['./index.html', './src/**/*.{ts,tsx}', './.ladle/**/*.{ts,tsx}'],
  prefix: 'lens-',
  corePlugins: {
    preflight: false,
  },
  theme: {
    // The Lens type scale, reachable as `lens-text-3xs … lens-text-3xl`. It sits
    // on `theme.fontSize` rather than `theme.extend.fontSize` on purpose: this
    // is the *only* type vocabulary in the runtime, so Tailwind's own steps are
    // replaced, not joined. A straggler `lens-text-xs` no longer resolves to
    // Tailwind's 12px — it resolves to the token's 10px — and a step name that
    // is not on this list (there are none left) fails to build at all.
    //
    // Each entry is a [size, line-height] tuple because a bare size drops the
    // leading Tailwind's steps ship with, and dense text silently reflows to the
    // inherited line-height. Sizes come from the `--lens-type-*` tokens; the
    // leading is stated in px beside them so the ratio is readable here.
    //
    // These keys cannot collide with the colour keys below (page, card, inset,
    // border, text, strong, muted, faint, neg, neg-alt, warn, accent-*) nor with
    // Tailwind's default palette, so `lens-text-*` stays unambiguous.
    fontSize: {
      '3xs': ['var(--lens-type-3xs)', '12px'], // 8/12
      '2xs': ['var(--lens-type-2xs)', '12px'], // 9/12
      xs: ['var(--lens-type-xs)', '14px'], // 10/14
      sm: ['var(--lens-type-sm)', '16px'], // 11/16
      base: ['var(--lens-type-base)', '16px'], // 12/16
      md: ['var(--lens-type-md)', '20px'], // 14/20
      lg: ['var(--lens-type-lg)', '24px'], // 16/24
      xl: ['var(--lens-type-xl)', '28px'], // 21/28
      '2xl': ['var(--lens-type-2xl)', '34px'], // 28/34
      '3xl': ['var(--lens-type-3xl)', '40px'], // 34/40
    },
    extend: {
      colors: {
        page: 'var(--lens-bg-page)',
        card: 'var(--lens-bg-card)',
        inset: 'var(--lens-bg-inset)',
        border: 'var(--lens-border)',
        'border-strong': 'var(--lens-border-strong)',
        text: 'var(--lens-text)',
        strong: 'var(--lens-text-strong)',
        muted: 'var(--lens-text-muted)',
        faint: 'var(--lens-text-faint)',
        neg: 'var(--lens-neg)',
        'neg-alt': 'var(--lens-neg-alt)',
        warn: 'var(--lens-warn)',
        accent: {
          DEFAULT: 'var(--lens-accent-500)',
          50: 'var(--lens-accent-50)',
          100: 'var(--lens-accent-100)',
          300: 'var(--lens-accent-300)',
          500: 'var(--lens-accent-500)',
          600: 'var(--lens-accent-600)',
          700: 'var(--lens-accent-700)',
        },
      },
      // Hit areas for anything a pointer acts on, as `lens-min-h-control-*`.
      // Layout boxes keep the numeric spacing scale.
      minHeight: {
        'control-sm': 'var(--lens-control-h-sm)',
        'control-md': 'var(--lens-control-h-md)',
        'control-lg': 'var(--lens-control-h-lg)',
      },
      // `--lens-radius-sm` is missing on purpose: `sm` is already Tailwind's
      // 2px corner, and the token is 7px.
      borderRadius: {
        card: 'var(--lens-radius-card)',
        control: 'var(--lens-radius-control)',
        badge: 'var(--lens-radius-badge)',
      },
      boxShadow: {
        card: 'var(--lens-shadow-card)',
        popover: 'var(--lens-shadow-popover)',
      },
      fontFamily: {
        sans: 'var(--lens-font)',
      },
      // Motion. `ease-out` deliberately resolves to the Lens out-curve rather
      // than Tailwind's: inside this prefix the token block is the vocabulary,
      // and no rule reaches for the stock curve today.
      transitionTimingFunction: {
        standard: 'var(--lens-ease-standard)',
        out: 'var(--lens-ease-out)',
        spring: 'var(--lens-ease-spring)',
      },
      transitionDuration: {
        fast: 'var(--lens-dur-fast)',
        base: 'var(--lens-dur-base)',
        slow: 'var(--lens-dur-slow)',
      },
      zIndex: {
        sticky: 'var(--lens-z-sticky)',
        popover: 'var(--lens-z-popover)',
        overlay: 'var(--lens-z-overlay)',
        tooltip: 'var(--lens-z-tooltip)',
      },
      // Spacing is deliberately NOT re-keyed onto --lens-space-*. Tailwind's
      // own 0.5–6 already compute to the same 2/4/8/12/16/20/24px, so routing
      // them through the tokens buys nothing and costs two real changes: the
      // utilities stop being rem (they would no longer follow a host root font
      // size) and every `padding: a b` shorthand splits into four longhands,
      // because a var() cannot be collapsed. The tokens stay the vocabulary for
      // raw declarations in the sheet; the numeric utilities stay Tailwind's.
    },
  },
} satisfies Config

import type { Config } from 'tailwindcss'

export default {
  content: ['./index.html', './src/**/*.{ts,tsx}', './.ladle/**/*.{ts,tsx}'],
  prefix: 'lens-',
  corePlugins: {
    preflight: false,
  },
  theme: {
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
      // The Lens type scale, reachable as `lens-text-type-*`. The keys carry the
      // `type-` prefix on purpose: Tailwind resolves `text-*` against fontSize
      // and textColor from the same namespace, and the bare step names (xs, sm,
      // base, lg, xl, 2xl, 3xl) are already Tailwind's own font sizes. Shadowing
      // those would silently resize every existing `lens-text-xs`/`lens-text-sm`
      // in the sheet — and drop the line-height they pair with. `type-*` cannot
      // collide with the colour keys above (page, card, inset, border, text,
      // strong, muted, faint, neg, neg-alt, warn, accent) nor with any default.
      fontSize: {
        'type-3xs': 'var(--lens-type-3xs)',
        'type-2xs': 'var(--lens-type-2xs)',
        'type-xs': 'var(--lens-type-xs)',
        'type-sm': 'var(--lens-type-sm)',
        'type-base': 'var(--lens-type-base)',
        'type-md': 'var(--lens-type-md)',
        'type-lg': 'var(--lens-type-lg)',
        'type-xl': 'var(--lens-type-xl)',
        'type-2xl': 'var(--lens-type-2xl)',
        'type-3xl': 'var(--lens-type-3xl)',
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

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
        accent: {
          DEFAULT: 'var(--lens-accent-500)',
          50: 'var(--lens-accent-50)',
          500: 'var(--lens-accent-500)',
          600: 'var(--lens-accent-600)',
          700: 'var(--lens-accent-700)',
        },
      },
      borderRadius: {
        card: 'var(--lens-radius-card)',
        control: 'var(--lens-radius-control)',
        badge: 'var(--lens-radius-badge)',
      },
      boxShadow: {
        card: 'var(--lens-shadow-card)',
      },
      fontFamily: {
        sans: 'var(--lens-font)',
      },
    },
  },
} satisfies Config

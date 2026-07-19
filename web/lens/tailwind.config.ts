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
        text: 'var(--lens-text)',
        strong: 'var(--lens-text-strong)',
        muted: 'var(--lens-text-muted)',
        accent: 'var(--lens-accent-500)',
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

/** @type {import('tailwindcss').Config} */
import { bichatTailwindPreset } from '../../../../web/chat-ui/src/tailwind.ts'

export default {
  presets: [bichatTailwindPreset],
  content: [
    './index.html',
    './src/**/*.{js,ts,jsx,tsx}',
  ],
  darkMode: 'selector',
  theme: {
    extend: {
      boxShadow: {
        xs: 'var(--shadow-xs)',
        sm: 'var(--shadow-sm)',
        md: 'var(--shadow-md)',
        lg: 'var(--shadow-lg)',
        xl: 'var(--shadow-xl)',
      },
      transitionTimingFunction: {
        smooth: 'var(--ease-smooth)',
        out: 'var(--ease-out)',
      },
    },
  },
  plugins: [],
}

/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: 'class',
  content: [
    // UI sources
    '../src/**/*.{ts,tsx}',
    // Storybook-only helpers + preview
    './**/*.{ts,tsx,js,css}',
  ],
  theme: {
    extend: {
      colors: {
        // Match BiChat web app Tailwind mapping (CSS variable driven)
        primary: {
          50: 'oklch(var(--primary-50))',
          100: 'oklch(var(--primary-100))',
          200: 'oklch(var(--primary-200))',
          300: 'oklch(var(--primary-300))',
          400: 'oklch(var(--primary-400))',
          500: 'oklch(var(--primary-500))',
          600: 'oklch(var(--primary-600))',
          700: 'oklch(var(--primary-700))',
          800: 'oklch(var(--primary-800))',
          900: 'oklch(var(--primary-900))',
        },
      },
      fontFamily: {
        sans: ['Gilroy', 'system-ui', '-apple-system', 'sans-serif'],
      },
    },
  },
  plugins: [require('@tailwindcss/typography')],
}


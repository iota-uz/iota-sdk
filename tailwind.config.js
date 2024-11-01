/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
      "./internal/presentation/templates/**/*.{html,js,templ}",
      "./internal/presentation/modules/**/templates/**/*.{html,js,templ}",
  ],
  theme: {
    extend: {
      fontFamily: {
        sans: ["Gilroy"]
      },
      backgroundColor: {
        surface: {
          100: "oklch(var(--clr-surface-100))",
          200: "oklch(var(--clr-surface-200))",
          300: "oklch(var(--clr-surface-300))",
          400: "oklch(var(--clr-surface-400))",
          500: "oklch(var(--clr-surface-500))",
          600: "oklch(var(--clr-surface-600))",
        },
        avatar: "oklch(var(--clr-avatar-bg))",
      },
      borderColor: {
        primary: "oklch(var(--clr-border-primary))",
        secondary: "oklch(var(--clr-border-secondary))",
      },
      colors: {
        100: "oklch(var(--clr-text-100))",
        200: "oklch(var(--clr-text-200))",
        300: "oklch(var(--clr-text-300))",
        avatar: "oklch(var(--clr-avatar-text))",
        black: {
          DEFAULT: "oklch(var(--black))",
          950: "oklch(var(--black-950))",
        },
        brand: {
          500: "oklch(var(--primary-500) / <alpha-value>)",
          600: "oklch(var(--primary-600) / <alpha-value>)",
          700: "oklch(var(--primary-700) / <alpha-value>)",
        },
        gray: {
          100: "oklch(var(--gray-100) / <alpha-value>)",
          200: "oklch(var(--gray-200) / <alpha-value>)",
          300: "oklch(var(--gray-300) / <alpha-value>)",
          400: "oklch(var(--gray-400) / <alpha-value>)",
          500: "oklch(var(--gray-500) / <alpha-value>)",
        },
        red: {
          100: "oklch(var(--red-100) / <alpha-value>)",
          200: "oklch(var(--red-200) / <alpha-value>)",
          500: "oklch(var(--red-500) / <alpha-value>)",
        }
      },
    },
  },
  plugins: [],
}


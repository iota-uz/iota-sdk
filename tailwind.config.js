/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./modules/**/templates/**/*.{html,js,templ}",
    "./components/**/*.{html,js,templ,go}",
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
        green: "oklch(var(--clr-border-green))",
        pink: "oklch(var(--clr-border-pink))",
        yellow: "oklch(var(--clr-border-yellow))",
        blue: "oklch(var(--clr-border-blue))",
        purple: "oklch(var(--clr-border-purple))",
      },
      colors: {
        100: "oklch(var(--clr-text-100))",
        200: "oklch(var(--clr-text-200))",
        300: "oklch(var(--clr-text-300))",
        green: "oklch(var(--clr-text-green))",
        pink: "oklch(var(--clr-text-pink))",
        yellow: "oklch(var(--clr-text-yellow))",
        blue: "oklch(var(--clr-text-blue))",
        purple: "oklch(var(--clr-text-purple))",
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
          100: "oklch(var(--red-100))",
          200: "oklch(var(--red-200))",
          500: "oklch(var(--red-500) / <alpha-value>)",
        },
        badge: {
          pink: "oklch(var(--clr-badge-pink))",
          yellow: "oklch(var(--clr-badge-yellow))",
          green: "oklch(var(--clr-badge-green))",
          blue: "oklch(var(--clr-badge-blue))",
          purple: "oklch(var(--clr-badge-purple))",
          gray: "oklch(var(--clr-badge-gray))",
        },
        success: {
          DEFAULT: "oklch(var(--green-500) / <alpha-value>)"
        },
        on: {
          success: "oklch(var(--white))"
        }
      },
    },
  },
  plugins: [],
}


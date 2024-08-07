/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./templates/**/*.{html,js,templ}"],
  theme: {
    extend: {
      fontFamily: {
        sans: ["Inter"]
      },
      backgroundColor: {
        default: "oklch(var(--clr-default-bg))",
        content: "oklch(var(--clr-content-bg))",
        primary: "oklch(var(--clr-primary-bg))",
        navbar: "oklch(var(--clr-navbar-bg))",
        avatar: "oklch(var(--clr-avatar-bg))",
        dropdown: "oklch(var(--clr-dropdown-bg))",
        table: "oklch(var(--clr-table-bg))",
        "table-heading": "oklch(var(--clr-table-heading-bg))",
        "dropdown-item-hover": "oklch(var(--clr-dropdown-item-hover))"
      },
      borderColor: {
        primary: "oklch(var(--clr-border-primary))",
        secondary: "oklch(var(--clr-border-secondary))",
      },
      colors: {
        primary: "oklch(var(--clr-primary-text))",
        secondary: "oklch(var(--clr-secondary-text))",
        avatar: "oklch(var(--clr-avatar-text))",
        "table-heading": "oklch(var(--clr-table-heading-text))",
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
      }
    },
  },
  plugins: [],
}


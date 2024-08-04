/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./templates/**/*.{html,js,templ}"],
  theme: {
    extend: {
      fontFamily: {
        sans: ["Inter"]
      },
      colors: {
        black: {
          DEFAULT: "oklch(var(--black))"
        },
        primary: {
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


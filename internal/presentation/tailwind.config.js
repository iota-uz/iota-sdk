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
          500: "oklch(var(--primary-500))",
          600: "oklch(var(--primary-600))",
          700: "oklch(var(--primary-700))",
        },
        gray: {
          100: "oklch(var(--gray-100))",
          200: "oklch(var(--gray-200))",
          300: "oklch(var(--gray-300))",
          400: "oklch(var(--gray-400))",
          500: "oklch(var(--gray-500))",
        },
        red: {
          100: "oklch(var(--red-100))",
          200: "oklch(var(--red-200))",
          500: "oklch(var(--red-500))",
        }
      }
    },
  },
  plugins: [],
}


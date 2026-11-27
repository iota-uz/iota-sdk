/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: "class",
  theme: {
    extend: {
      // Motion tokens (#1352): `duration-fast ease-out`, never a literal duration on a screen.
      transitionDuration: {
        instant: "var(--motion-instant)",
        fast: "var(--motion-fast)",
        base: "var(--motion-base)",
        slow: "var(--motion-slow)",
      },
      transitionTimingFunction: {
        out: "var(--ease-out)",
        in: "var(--ease-in)",
        standard: "var(--ease-standard)",
      },
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
        toast: "oklch(var(--clr-toast-surface))",
        // Status fills (dots, counters) and the off track of a switch: semantic tokens, not palette shades.
        danger: "oklch(var(--clr-border-danger))",
        warning: "oklch(var(--clr-border-warning))",
        track: "oklch(var(--clr-border-strong))",
      },
      borderColor: {
        toast: "oklch(var(--clr-toast-border))",
        subtle: "oklch(var(--clr-border-subtle) / <alpha-value>)",
        default: "oklch(var(--clr-border-default) / <alpha-value>)",
        strong: "oklch(var(--clr-border-strong) / <alpha-value>)",
        brand: "oklch(var(--clr-border-brand) / <alpha-value>)",
        danger: "oklch(var(--clr-border-danger) / <alpha-value>)",
        warning: "oklch(var(--clr-border-warning) / <alpha-value>)",
        success: "oklch(var(--clr-border-success) / <alpha-value>)",
        disabled: "oklch(var(--clr-border-disabled) / <alpha-value>)",
        // Deprecated compatibility alias. Use border-subtle/default/strong.
        primary: "oklch(var(--clr-border-primary))",
        secondary: "oklch(var(--clr-border-secondary))",
        green: "oklch(var(--clr-border-green))",
        pink: "oklch(var(--clr-border-pink))",
        yellow: "oklch(var(--clr-border-yellow))",
        blue: "oklch(var(--clr-border-blue))",
        purple: "oklch(var(--clr-border-purple))",
      },
      textColor: {
        toast: "oklch(var(--clr-toast-text))",
        "toast-muted": "oklch(var(--clr-toast-text-muted))",
        "toast-icon": "oklch(var(--clr-toast-icon-muted))",
        "toast-success": "oklch(var(--clr-toast-success))",
        "toast-error": "oklch(var(--clr-toast-error))",
        "toast-warning": "oklch(var(--clr-toast-warning))",
        "toast-info": "oklch(var(--clr-toast-info))",
        "toast-action": "oklch(var(--clr-toast-action))",
        // Error text, readable on surfaces in both themes (the danger button's text shade).
        danger: "oklch(var(--clr-danger-btn-text))",
        100: "oklch(var(--clr-text-100))",
        200: "oklch(var(--clr-text-200))",
        300: "oklch(var(--clr-text-300))",
        green: "oklch(var(--clr-text-green))",
        pink: "oklch(var(--clr-text-pink))",
        yellow: "oklch(var(--clr-text-yellow))",
        blue: "oklch(var(--clr-text-blue))",
        purple: "oklch(var(--clr-text-purple))",
        avatar: "oklch(var(--clr-avatar-text))",
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
        primary: {
          DEFAULT: "oklch(var(--primary-500) / <alpha-value>)",
          50: "oklch(var(--primary-50) / <alpha-value>)",
          300: "oklch(var(--primary-300) / <alpha-value>)",
          400: "oklch(var(--primary-400) / <alpha-value>)",
          500: "oklch(var(--primary-500) / <alpha-value>)",
          600: "oklch(var(--primary-600) / <alpha-value>)",
          700: "oklch(var(--primary-700) / <alpha-value>)",
        },
        brand: {
          500: "oklch(var(--primary-500) / <alpha-value>)",
          600: "oklch(var(--primary-600) / <alpha-value>)",
          700: "oklch(var(--primary-700) / <alpha-value>)",
        },
        gray: {
          50: "oklch(var(--gray-50) / <alpha-value>)",
          100: "oklch(var(--gray-100) / <alpha-value>)",
          200: "oklch(var(--gray-200) / <alpha-value>)",
          300: "oklch(var(--gray-300) / <alpha-value>)",
          400: "oklch(var(--gray-400) / <alpha-value>)",
          500: "oklch(var(--gray-500) / <alpha-value>)",
          600: "oklch(var(--gray-600) / <alpha-value>)",
          700: "oklch(var(--gray-700) / <alpha-value>)",
          800: "oklch(var(--gray-800) / <alpha-value>)",
          900: "oklch(var(--gray-900) / <alpha-value>)",
          950: "oklch(var(--gray-950) / <alpha-value>)",
        },
        green: {
          50: "oklch(var(--green-50) / <alpha-value>)",
          100: "oklch(var(--green-100) / <alpha-value>)",
          200: "oklch(var(--green-200) / <alpha-value>)",
          500: "oklch(var(--green-500) / <alpha-value>)",
          600: "oklch(var(--green-600) / <alpha-value>)",
        },
        red: {
          100: "oklch(var(--red-100))",
          200: "oklch(var(--red-200))",
          500: "oklch(var(--red-500) / <alpha-value>)",
          600: "oklch(var(--red-600) / <alpha-value>)",
          700: "oklch(var(--red-700) / <alpha-value>)",
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
          success: "oklch(var(--clr-on-sucess))"
        }
      },
    },
  },
  plugins: [],
}

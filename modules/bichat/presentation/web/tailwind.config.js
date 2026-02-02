/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  darkMode: 'class',
  theme: {
    extend: {
      // Override backgroundColor to use static red instead of CSS variables
      backgroundColor: {
        red: {
          50: '#fef2f2',
          100: '#fee2e2',
          200: '#fecaca',
          300: '#fca5a5',
          400: '#f87171',
          500: '#ef4444',
          600: '#dc2626',
          700: '#b91c1c',
          800: '#991b1b',
          900: '#7f1d1d',
        },
      },
      colors: {
        // Use Tailwind's default static red colors instead of CSS variables
        red: {
          50: '#fef2f2',
          100: '#fee2e2',
          200: '#fecaca',
          300: '#fca5a5',
          400: '#f87171',
          500: '#ef4444',
          600: '#dc2626',
          700: '#b91c1c',
          800: '#991b1b',
          900: '#7f1d1d',
        },
        // Use Tailwind's default static gray colors
        gray: {
          50: '#f9fafb',
          100: '#f3f4f6',
          200: '#e5e7eb',
          300: '#d1d5db',
          400: '#9ca3af',
          500: '#6b7280',
          600: '#4b5563',
          700: '#374151',
          800: '#1f2937',
          900: '#111827',
        },
        // Custom primary colors (purple brand)
        primary: {
          50: 'oklch(var(--primary-100))',
          100: 'oklch(var(--primary-100))',
          200: 'oklch(var(--primary-200))',
          300: 'oklch(var(--primary-300))',
          400: 'oklch(var(--primary-400))',
          500: 'oklch(var(--primary-500))',
          600: 'oklch(var(--primary-600))',
          650: 'oklch(var(--primary-650))',
          700: 'oklch(var(--primary-700))',
          800: 'oklch(var(--primary-800))',
          900: 'oklch(var(--primary-900))',
        },
        // Surface colors for backgrounds
        surface: {
          100: 'oklch(var(--clr-surface-100))',
          300: 'oklch(var(--clr-surface-300))',
          400: 'oklch(var(--clr-surface-400))',
        }
      },
      fontFamily: {
        sans: ['"Gilroy"', 'system-ui', '-apple-system', 'sans-serif'],
      },
      boxShadow: {
        'sm': 'var(--shadow-sm)',
        'md': 'var(--shadow-md)',
      }
    }
  },
  plugins: [],
}

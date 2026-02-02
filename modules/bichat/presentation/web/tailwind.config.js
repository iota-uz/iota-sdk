/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
    "../../../../ui/bichat/src/**/*.{js,ts,jsx,tsx}",
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
          950: '#0d1117',
        },
        // Custom primary colors (purple brand)
        primary: {
          50: 'oklch(var(--primary-50))',
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
        // Accent colors (warm coral)
        accent: {
          400: 'oklch(var(--accent-400))',
          500: 'oklch(var(--accent-500))',
          600: 'oklch(var(--accent-600))',
        },
        // Surface colors for backgrounds
        surface: {
          100: 'oklch(var(--clr-surface-100))',
          200: 'oklch(var(--clr-surface-200))',
          300: 'oklch(var(--clr-surface-300))',
          400: 'oklch(var(--clr-surface-400))',
        }
      },
      fontFamily: {
        sans: ['"Gilroy"', 'system-ui', '-apple-system', 'sans-serif'],
      },
      boxShadow: {
        'xs': 'var(--shadow-xs)',
        'sm': 'var(--shadow-sm)',
        'md': 'var(--shadow-md)',
        'lg': 'var(--shadow-lg)',
        'xl': 'var(--shadow-xl)',
        'glow': 'var(--shadow-glow)',
        'glow-strong': 'var(--shadow-glow-strong)',
      },
      transitionTimingFunction: {
        'spring': 'var(--ease-spring)',
        'smooth': 'var(--ease-smooth)',
        'out-expo': 'var(--ease-out-expo)',
      },
      backgroundImage: {
        'gradient-primary': 'var(--gradient-primary)',
        'gradient-primary-vivid': 'var(--gradient-primary-vivid)',
        'gradient-surface': 'var(--gradient-surface)',
        'gradient-shimmer': 'var(--gradient-shimmer)',
      },
      backdropBlur: {
        'glass': 'var(--glass-blur)',
      },
      borderRadius: {
        '2xl': '1rem',
        '3xl': '1.5rem',
        '4xl': '2rem',
      },
      animation: {
        'pulse-glow': 'pulse-glow 2s ease-in-out infinite',
        'float': 'float 3s ease-in-out infinite',
        'shimmer': 'shimmer 2s infinite',
      },
    }
  },
  plugins: [],
}

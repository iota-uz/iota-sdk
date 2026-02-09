/**
 * BiChat Tailwind preset and content helper for downstream applets.
 * Use these so Tailwind generates utilities for SDK BiChat components and so
 * theme tokens stay consistent. Always import @iota-uz/sdk/bichat/styles.css
 * in your applet CSS for design tokens.
 */
import path from 'node:path'
import { createRequire } from 'node:module'

/** Tailwind v3 config preset type (minimal shape we extend). */
export type TailwindPreset = {
  darkMode?: 'class' | 'selector' | 'media'
  theme?: { extend?: Record<string, unknown> }
  plugins?: unknown[]
}

/**
 * Preset for BiChat UI: theme extensions and dark mode.
 * Stable and minimal; apps can override via their own theme or CSS variables.
 */
export const bichatTailwindPreset: TailwindPreset = {
  darkMode: 'selector',
  theme: {
    extend: {
      colors: {
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
        primary: {
          50: 'oklch(var(--primary-50, var(--bichat-color-primary-50, #eff6ff)))',
          100: 'oklch(var(--primary-100, var(--bichat-color-primary-100, #dbeafe)))',
          200: 'oklch(var(--primary-200, var(--bichat-color-primary-200, #bfdbfe)))',
          300: 'oklch(var(--primary-300, var(--bichat-color-primary-300, #93c5fd)))',
          400: 'oklch(var(--primary-400, var(--bichat-color-primary-400, #60a5fa)))',
          500: 'oklch(var(--primary-500, var(--bichat-color-primary-500, #3b82f6)))',
          600: 'oklch(var(--primary-600, var(--bichat-color-primary-600, #2563eb)))',
          700: 'oklch(var(--primary-700, var(--bichat-color-primary-700, #1d4ed8)))',
          800: 'oklch(var(--primary-800, var(--bichat-color-primary-800, #1e40af)))',
          900: 'oklch(var(--primary-900, var(--bichat-color-primary-900, #1e3a8a)))',
        },
      },
      fontFamily: {
        sans: ['"Gilroy"', 'system-ui', '-apple-system', 'sans-serif'],
      },
      boxShadow: {
        xs: 'var(--shadow-xs, 0 1px 2px 0 rgb(0 0 0 / 0.05))',
        sm: 'var(--shadow-sm, 0 1px 3px 0 rgb(0 0 0 / 0.1))',
        md: 'var(--shadow-md, 0 4px 6px -1px rgb(0 0 0 / 0.1))',
        lg: 'var(--shadow-lg, 0 10px 15px -3px rgb(0 0 0 / 0.1))',
        xl: 'var(--shadow-xl, 0 20px 25px -5px rgb(0 0 0 / 0.1))',
      },
      borderRadius: {
        '2xl': '1rem',
        '3xl': '1.5rem',
      },
    },
  },
  plugins: [],
}

/**
 * Returns absolute paths to SDK BiChat bundle files so Tailwind can scan them
 * for class names. Include the result in your Tailwind config content array.
 * Resolves via @iota-uz/sdk/package.json when available so the package root is unambiguous.
 */
export function bichatTailwindContent(): string[] {
  const require = createRequire(import.meta.url)
  let pkgDir: string
  try {
    const pkgJsonPath = require.resolve('@iota-uz/sdk/package.json')
    pkgDir = path.dirname(pkgJsonPath)
  } catch {
    const entryPath = require.resolve('@iota-uz/sdk')
    pkgDir = path.dirname(entryPath) // when "." resolves to dist/index.mjs, dirname is dist
  }
  const distBichat = pkgDir.endsWith(path.sep + 'dist') || pkgDir.endsWith('/dist')
    ? path.join(pkgDir, 'bichat')
    : path.join(pkgDir, 'dist', 'bichat')
  return [
    path.join(distBichat, 'index.mjs'),
    path.join(distBichat, 'index.cjs'),
  ]
}

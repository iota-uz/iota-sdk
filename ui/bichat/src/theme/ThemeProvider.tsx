/**
 * Theme provider component and hook
 * Manages theme state and applies CSS variables to document root
 */

import { createContext, useContext, useEffect, useMemo, ReactNode } from 'react'
import { Theme } from './types'
import { lightTheme, darkTheme } from './themes'

interface ThemeContextValue {
  theme: Theme
}

const ThemeContext = createContext<ThemeContextValue | null>(null)

export interface ThemeProviderProps {
  theme?: Theme | 'light' | 'dark' | 'system'
  children: ReactNode
}

/**
 * Detect system theme preference
 */
function getSystemTheme(): Theme {
  if (typeof window === 'undefined') {
    return lightTheme
  }

  const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches
  return prefersDark ? darkTheme : lightTheme
}

/**
 * Resolve theme prop to Theme object
 */
function resolveTheme(themeProp: Theme | 'light' | 'dark' | 'system'): Theme {
  if (typeof themeProp === 'object') {
    return themeProp
  }

  switch (themeProp) {
    case 'light':
      return lightTheme
    case 'dark':
      return darkTheme
    case 'system':
      return getSystemTheme()
    default:
      return lightTheme
  }
}

/**
 * Apply theme CSS variables to document root
 */
function applyThemeVariables(theme: Theme): void {
  if (typeof document === 'undefined') {
    return
  }

  const root = document.documentElement

  // Apply color variables
  Object.entries(theme.colors).forEach(([key, value]) => {
    root.style.setProperty(`--bichat-${key}`, value)
  })

  // Apply spacing variables
  Object.entries(theme.spacing).forEach(([key, value]) => {
    root.style.setProperty(`--bichat-spacing-${key}`, value)
  })

  // Apply border radius variables
  Object.entries(theme.borderRadius).forEach(([key, value]) => {
    root.style.setProperty(`--bichat-radius-${key}`, value)
  })
}

/**
 * Theme provider component
 * Wraps the application and provides theme context
 */
export function ThemeProvider({ theme = 'system', children }: ThemeProviderProps) {
  const resolvedTheme = useMemo(() => resolveTheme(theme), [theme])

  useEffect(() => {
    applyThemeVariables(resolvedTheme)
  }, [resolvedTheme])

  // Listen for system theme changes when using 'system'
  useEffect(() => {
    if (theme !== 'system') {
      return
    }

    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')

    const handleChange = () => {
      const newTheme = getSystemTheme()
      applyThemeVariables(newTheme)
    }

    mediaQuery.addEventListener('change', handleChange)

    return () => {
      mediaQuery.removeEventListener('change', handleChange)
    }
  }, [theme])

  const value: ThemeContextValue = {
    theme: resolvedTheme,
  }

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>
}

/**
 * Hook to access current theme
 */
export function useTheme(): Theme {
  const context = useContext(ThemeContext)

  if (!context) {
    throw new Error('useTheme must be used within ThemeProvider')
  }

  return context.theme
}

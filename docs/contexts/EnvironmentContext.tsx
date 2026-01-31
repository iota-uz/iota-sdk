'use client'

import React, { createContext, useContext, useState, useEffect, ReactNode } from 'react'
import { Environment, EnvironmentContextType, ENV_URLS } from '../types/environment'

const EnvironmentContext = createContext<EnvironmentContextType | undefined>(undefined)

const STORAGE_KEY = 'granite-docs-environment'

// ERP paths pattern - paths starting with these are considered ERP links
const ERP_PATH_PATTERNS = ['/crm', '/insurance', '/reinsurance', '/administration']

export function EnvironmentProvider({ children }: { children: ReactNode }) {
  const [environment, setEnvironmentState] = useState<Environment>('production')
  const [mounted, setMounted] = useState(false)

  // Load from localStorage on mount
  useEffect(() => {
    setMounted(true)
    const stored = localStorage.getItem(STORAGE_KEY) as Environment | null
    if (stored && ['production', 'staging', 'pre-production'].includes(stored)) {
      setEnvironmentState(stored)
    }
  }, [])

  // Persist to localStorage
  const setEnvironment = (env: Environment) => {
    setEnvironmentState(env)
    if (mounted) {
      localStorage.setItem(STORAGE_KEY, env)
    }
  }

  // Translate relative path to absolute URL
  const getUrl = (path: string, type: 'erp' | 'website' | 'auto' = 'auto'): string => {
    // External links - return as-is
    if (path.startsWith('http://') || path.startsWith('https://')) {
      return path
    }

    // Not a relative path - return as-is
    if (!path.startsWith('/')) {
      return path
    }

    // Auto-detect type based on path
    const linkType: 'erp' | 'website' =
      type === 'auto'
        ? ERP_PATH_PATTERNS.some(pattern => path.startsWith(pattern))
          ? 'erp'
          : 'website'
        : type

    // Get base URL for current environment
    const baseUrl = ENV_URLS[environment as Environment][linkType]

    // If pre-production (empty baseUrl), return # to disable
    if (!baseUrl) {
      return '#'
    }

    return `${baseUrl}${path}`
  }

  return (
    <EnvironmentContext.Provider value={{ environment, setEnvironment, getUrl }}>
      {children}
    </EnvironmentContext.Provider>
  )
}

export function useEnvironment() {
  const context = useContext(EnvironmentContext)
  if (!context) {
    throw new Error('useEnvironment must be used within EnvironmentProvider')
  }
  return context
}

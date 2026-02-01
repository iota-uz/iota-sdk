'use client'

import { useState, useEffect, useRef } from 'react'
import { useEnvironment } from '../contexts/EnvironmentContext'
import type { Environment } from '../types/environment'

const environments: { value: Environment; label: string; disabled?: boolean; tooltip?: string }[] = [
  { value: 'production', label: 'Production' },
  { value: 'staging', label: 'Staging' },
  { value: 'pre-production', label: 'Pre-production', disabled: true, tooltip: 'Coming Soon' }
]

export function EnvironmentSelector() {
  const { environment, setEnvironment } = useEnvironment()
  const [isOpen, setIsOpen] = useState(false)
  const dropdownRef = useRef<HTMLDivElement>(null)

  // Close dropdown when clicking outside or pressing Escape
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsOpen(false)
      }
    }

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setIsOpen(false)
      }
    }

    if (isOpen) {
      document.addEventListener('mousedown', handleClickOutside)
      document.addEventListener('keydown', handleKeyDown)
      return () => {
        document.removeEventListener('mousedown', handleClickOutside)
        document.removeEventListener('keydown', handleKeyDown)
      }
    }
  }, [isOpen])

  const currentEnv = environments.find(env => env.value === environment) || environments[0]

  return (
    <div className="relative hidden sm:block" ref={dropdownRef}>
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="flex items-center gap-2 rounded-lg px-3 py-1.5 text-sm font-medium text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
        aria-label="Select environment"
      >
        {/* Server icon */}
        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
            d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01"
          />
        </svg>
        <span>{currentEnv.label}</span>
        {/* Chevron icon */}
        <svg
          className={`w-4 h-4 transition-transform ${isOpen ? 'rotate-180' : ''}`}
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
        </svg>
      </button>

      {isOpen && (
        <div className="absolute right-0 mt-2 w-48 rounded-lg bg-white dark:bg-gray-900 shadow-lg border border-gray-200 dark:border-gray-700 py-1 z-50">
          {environments.map((env) => (
            <button
              key={env.value}
              onClick={() => {
                if (!env.disabled) {
                  setEnvironment(env.value)
                  setIsOpen(false)
                }
              }}
              disabled={env.disabled}
              title={env.tooltip}
              className={`
                w-full text-left px-4 py-2 text-sm flex items-center justify-between
                ${env.disabled
                  ? 'opacity-50 cursor-not-allowed text-gray-500 dark:text-gray-500'
                  : 'hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-700 dark:text-gray-200'
                }
                ${environment === env.value ? 'bg-blue-50 dark:bg-blue-950' : ''}
              `}
            >
              <span className="flex items-center gap-2">
                {env.label}
                {env.disabled && (
                  <span className="text-xs italic text-gray-400">({env.tooltip})</span>
                )}
              </span>
              {environment === env.value && !env.disabled && (
                <svg className="w-4 h-4 text-blue-500" fill="currentColor" viewBox="0 0 20 20">
                  <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
                </svg>
              )}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

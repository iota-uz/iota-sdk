'use client'

import React, { ReactNode } from 'react'

interface ChecklistCardProps {
  children: ReactNode
  title?: string
}

interface ChecklistItemProps {
  children: ReactNode
}

const ChecklistCardBase = ({ children, title }: ChecklistCardProps) => {
  return (
    <div className="rounded-lg border border-gray-200 dark:border-gray-700 p-6 bg-white dark:bg-gray-950">
      {title && (
        <h3 className="font-semibold text-lg text-gray-900 dark:text-gray-100 mb-6">{title}</h3>
      )}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">{children}</div>
    </div>
  )
}

const Required = ({ children }: ChecklistItemProps) => {
  return (
    <div className="space-y-3">
      {React.Children.toArray(children).map((child, index) => (
        <div key={index} className="flex items-start gap-3">
          <svg
            className="w-5 h-5 text-green-500 dark:text-green-400 flex-shrink-0 mt-0.5"
            fill="currentColor"
            viewBox="0 0 20 20"
          >
            <path
              fillRule="evenodd"
              d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
              clipRule="evenodd"
            />
          </svg>
          <span className="text-sm text-gray-700 dark:text-gray-300">{child}</span>
        </div>
      ))}
    </div>
  )
}

const NotRequired = ({ children }: ChecklistItemProps) => {
  return (
    <div className="space-y-3">
      {React.Children.toArray(children).map((child, index) => (
        <div key={index} className="flex items-start gap-3">
          <svg
            className="w-5 h-5 text-red-500 dark:text-red-400 flex-shrink-0 mt-0.5"
            fill="currentColor"
            viewBox="0 0 20 20"
          >
            <path
              fillRule="evenodd"
              d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z"
              clipRule="evenodd"
            />
          </svg>
          <span className="text-sm text-gray-700 dark:text-gray-300">{child}</span>
        </div>
      ))}
    </div>
  )
}

// Export sub-components separately for better SSR compatibility
export const ChecklistCardRequired = Required
export const ChecklistCardNotRequired = NotRequired

export const ChecklistCard = Object.assign(ChecklistCardBase, {
  Required,
  NotRequired,
})

'use client'

import React, { ReactNode } from 'react'

interface InfoCardProps {
  children: ReactNode
  title: string
  icon?: ReactNode
}

interface InfoCardSectionProps {
  children: ReactNode
  title: string
  nested?: boolean
}

interface InfoCardLimitProps {
  children: ReactNode
  label?: string
}

const InfoCardBase = ({ children, title, icon }: InfoCardProps) => {
  return (
    <div className="rounded-lg border border-gray-200 dark:border-gray-700 p-4 mb-4 bg-white dark:bg-gray-950">
      <div className="flex items-center gap-2 mb-4">
        {icon && <div className="text-blue-500 dark:text-blue-400">{icon}</div>}
        <h3 className="font-semibold text-gray-900 dark:text-gray-100">{title}</h3>
      </div>
      <div className="space-y-2">{children}</div>
    </div>
  )
}

const Section = ({ children, title, nested = false }: InfoCardSectionProps) => {
  const paddingClass = nested ? 'pl-4 border-l-2 border-gray-300 dark:border-gray-600' : ''

  return (
    <div className={`${paddingClass}`}>
      {title && (
        <h4 className={`font-medium mb-2 ${nested ? 'text-gray-700 dark:text-gray-300' : 'text-gray-800 dark:text-gray-200'}`}>
          {title}
        </h4>
      )}
      <div className="space-y-2">{children}</div>
    </div>
  )
}

const Limit = ({ children, label }: InfoCardLimitProps) => {
  return (
    <div className="bg-blue-50 dark:bg-blue-950 border-l-4 border-blue-500 dark:border-blue-400 p-3 rounded">
      {label && (
        <p className="text-xs font-semibold text-blue-700 dark:text-blue-300 uppercase tracking-wide mb-1">
          {label}
        </p>
      )}
      <p className="text-sm text-gray-800 dark:text-gray-200">{children}</p>
    </div>
  )
}

// Export sub-components separately for better SSR compatibility
export const InfoCardSection = Section
export const InfoCardLimit = Limit

export const InfoCard = Object.assign(InfoCardBase, {
  Section,
  Limit,
})

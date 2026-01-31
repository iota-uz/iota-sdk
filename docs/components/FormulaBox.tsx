'use client'

import React, { ReactNode } from 'react'

interface FormulaBoxProps {
  children: ReactNode
  title?: string
}

interface FormulaBoxEquationProps {
  children: ReactNode
}

interface FormulaBoxVariablesProps {
  children: ReactNode
}

interface FormulaBoxVarProps {
  name: string
  value: ReactNode
}

interface FormulaBoxResultProps {
  children: ReactNode
  label?: string
}

const FormulaBoxBase = ({ children, title }: FormulaBoxProps) => {
  return (
    <div className="rounded-lg border border-gray-200 dark:border-gray-700 p-6 bg-white dark:bg-gray-950">
      {title && (
        <h3 className="font-semibold text-lg text-gray-900 dark:text-gray-100 mb-4">{title}</h3>
      )}
      <div className="space-y-4">{children}</div>
    </div>
  )
}

const Equation = ({ children }: FormulaBoxEquationProps) => {
  return (
    <div className="bg-gray-50 dark:bg-gray-900 rounded-lg p-4 border border-gray-200 dark:border-gray-700">
      <code className="font-mono text-sm text-gray-800 dark:text-gray-200 whitespace-pre-wrap break-words">
        {children}
      </code>
    </div>
  )
}

const Variables = ({ children }: FormulaBoxVariablesProps) => {
  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
      {children}
    </div>
  )
}

const Var = ({ name, value }: FormulaBoxVarProps) => {
  return (
    <div className="flex justify-between items-start gap-4 p-3 bg-gray-50 dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-700">
      <span className="font-mono text-sm font-semibold text-gray-700 dark:text-gray-300">{name}</span>
      <span className="text-sm text-gray-800 dark:text-gray-200 text-right">{value}</span>
    </div>
  )
}

const Result = ({ children, label = 'Result' }: FormulaBoxResultProps) => {
  return (
    <div className="bg-green-50 dark:bg-green-950 border-l-4 border-green-500 dark:border-green-400 p-4 rounded-lg">
      <p className="text-xs font-semibold text-green-700 dark:text-green-300 uppercase tracking-wide mb-2">
        {label}
      </p>
      <p className="text-lg font-semibold text-gray-900 dark:text-gray-100">{children}</p>
    </div>
  )
}

// Export sub-components separately for better SSR compatibility
export const FormulaBoxEquation = Equation
export const FormulaBoxVariables = Variables
export const FormulaBoxVar = Var
export const FormulaBoxResult = Result

export const FormulaBox = Object.assign(FormulaBoxBase, {
  Equation,
  Variables,
  Var,
  Result,
})

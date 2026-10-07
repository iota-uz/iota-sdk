import { children as solidChildren, type JSX } from 'solid-js'

interface FormulaBoxProps {
  children: JSX.Element
  title?: string
}

interface FormulaBoxEquationProps {
  children: JSX.Element
}

interface FormulaBoxVariablesProps {
  children: JSX.Element
}

interface FormulaBoxVarProps {
  name: string
  value: JSX.Element
}

interface FormulaBoxResultProps {
  children: JSX.Element
  label?: string
}

const FormulaBoxBase = ({ children, title }: FormulaBoxProps) => {
  return (
    <div class="rounded-lg border border-gray-200 dark:border-gray-700 p-6 bg-white dark:bg-gray-950">
      {title && (
        <h3 class="font-semibold text-lg text-gray-900 dark:text-gray-100 mb-4">{title}</h3>
      )}
      <div class="space-y-4">{children}</div>
    </div>
  )
}

const Equation = ({ children }: FormulaBoxEquationProps) => {
  return (
    <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4 border border-gray-200 dark:border-gray-700">
      <code class="font-mono text-sm text-gray-800 dark:text-gray-200 whitespace-pre-wrap break-words">
        {children}
      </code>
    </div>
  )
}

const Variables = ({ children }: FormulaBoxVariablesProps) => {
  return (
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
      {children}
    </div>
  )
}

const Var = ({ name, value }: FormulaBoxVarProps) => {
  return (
    <div class="flex justify-between items-start gap-4 p-3 bg-gray-50 dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-700">
      <span class="font-mono text-sm font-semibold text-gray-700 dark:text-gray-300">{name}</span>
      <span class="text-sm text-gray-800 dark:text-gray-200 text-right">{value}</span>
    </div>
  )
}

const Result = ({ children, label = 'Result' }: FormulaBoxResultProps) => {
  return (
    <div class="bg-green-50 dark:bg-green-950 border-l-4 border-green-500 dark:border-green-400 p-4 rounded-lg">
      <p class="text-xs font-semibold text-green-700 dark:text-green-300 uppercase tracking-wide mb-2">
        {label}
      </p>
      <p class="text-lg font-semibold text-gray-900 dark:text-gray-100">{children}</p>
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

import { useMDXComponents as getDocsMDXComponents } from 'nextra-theme-docs'
import {
  InfoCard,
  InfoCardSection,
  InfoCardLimit,
  BarChart,
  FormulaBox,
  FormulaBoxEquation,
  FormulaBoxVariables,
  FormulaBoxVar,
  FormulaBoxResult,
  ChecklistCard,
  ChecklistCardRequired,
  ChecklistCardNotRequired
} from './components'

const docsComponents = getDocsMDXComponents()

export function useMDXComponents(components) {
  return {
    ...docsComponents,
    // Main components
    InfoCard,
    BarChart,
    FormulaBox,
    ChecklistCard,
    // Flat-named sub-components for MDX compatibility
    InfoCardSection,
    InfoCardLimit,
    FormulaBoxEquation,
    FormulaBoxVariables,
    FormulaBoxVar,
    FormulaBoxResult,
    ChecklistCardRequired,
    ChecklistCardNotRequired,
    ...components
  }
}

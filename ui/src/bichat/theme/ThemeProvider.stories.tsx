import type { Meta, StoryObj } from '@storybook/react'
import { ThemeProvider, useTheme } from './ThemeProvider'

const meta: Meta<typeof ThemeProvider> = {
  title: 'BiChat/Theme/ThemeProvider',
  component: ThemeProvider,
}

export default meta
type Story = StoryObj<typeof ThemeProvider>

const ThemeDisplay = () => {
  const theme = useTheme()
  return (
    <div className="p-6 border rounded space-y-4 bg-white dark:bg-gray-800 dark:text-white transition-colors">
      <div className="text-xl font-bold">Theme colors:</div>
      <div className="grid grid-cols-2 gap-2">
        {Object.entries(theme.colors).map(([key, value]) => (
          <div key={key} className="flex items-center gap-2">
            <div className="w-4 h-4 rounded border" style={{ backgroundColor: value }} />
            <span className="text-xs">{key}: {value}</span>
          </div>
        ))}
      </div>
      <div className="p-4 bg-primary-100 dark:bg-primary-900/30 text-primary-700 dark:text-primary-300 rounded">
        This box should use primary colors.
      </div>
    </div>
  )
}

export const Playground: Story = {
  render: () => (
    <ThemeProvider theme="light">
      <ThemeDisplay />
    </ThemeProvider>
  ),
}

export const Dark: Story = {
  render: () => (
    <ThemeProvider theme="dark">
      <ThemeDisplay />
    </ThemeProvider>
  ),
}

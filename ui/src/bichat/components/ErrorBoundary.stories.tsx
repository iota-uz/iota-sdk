import type { Meta, StoryObj } from '@storybook/react'

import { ErrorBoundary } from './ErrorBoundary'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'

const meta: Meta<typeof ErrorBoundary> = {
  title: 'BiChat/Components/ErrorBoundary',
  component: ErrorBoundary,
}

export default meta
type Story = StoryObj<typeof ErrorBoundary>

const BuggyComponent = ({ message = 'Crash!' }: { message?: string }) => {
  throw new Error(message)
}

export const Playground: Story = {
  render: () => (
    <ErrorBoundary>
      <BuggyComponent message="Controlled crash for Storybook" />
    </ErrorBoundary>
  ),
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Custom Fallback (Function)',
          content: (
            <ErrorBoundary
              fallback={(error, reset) => (
                <div className="p-4 border border-red-500 rounded bg-red-50 text-red-700">
                  <p className="font-bold">Error: {error?.message}</p>
                  <button onClick={reset} className="mt-2 underline">Retry</button>
                </div>
              )}
            >
              <BuggyComponent />
            </ErrorBoundary>
          ),
        },
        {
          name: 'Long Error Message',
          content: (
            <ErrorBoundary>
              <BuggyComponent message="This is an extremely long error message. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat." />
            </ErrorBoundary>
          ),
        },
      ]}
    />
  ),
}

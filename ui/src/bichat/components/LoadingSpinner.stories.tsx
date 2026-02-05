import type { Meta, StoryObj } from '@storybook/react'

import { LoadingSpinner } from './LoadingSpinner'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'

const meta: Meta<typeof LoadingSpinner> = {
  title: 'BiChat/Components/LoadingSpinner',
  component: LoadingSpinner,
  parameters: {
    layout: 'centered',
  },
}

export default meta
type Story = StoryObj<typeof LoadingSpinner>

export const Playground: Story = {
  args: {
    variant: 'spinner',
    size: 'md',
    message: 'Loading data...',
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Variants',
          content: (
            <div className="flex flex-col gap-8">
              <LoadingSpinner variant="spinner" message="Default Spinner" />
              <LoadingSpinner variant="dots" message="Bouncing Dots" />
              <LoadingSpinner variant="pulse" message="Pulse Ring" />
            </div>
          ),
        },
        {
          name: 'Sizes',
          content: (
            <div className="flex items-end gap-12">
              <LoadingSpinner size="sm" />
              <LoadingSpinner size="md" />
              <LoadingSpinner size="lg" />
            </div>
          ),
        },
        {
          name: 'Long Message',
          content: (
            <LoadingSpinner
              message="This is an extremely long loading message intended to test text wrapping and center alignment during long operations."
            />
          ),
        },
      ]}
    />
  ),
}

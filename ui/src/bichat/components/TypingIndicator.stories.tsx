import type { Meta, StoryObj } from '@storybook/react'

import { TypingIndicator } from './TypingIndicator'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'

const meta: Meta<typeof TypingIndicator> = {
  title: 'BiChat/Components/TypingIndicator',
  component: TypingIndicator,
  parameters: {
    layout: 'centered',
  },
}

export default meta
type Story = StoryObj<typeof TypingIndicator>

export const Playground: Story = {
  args: {
    variant: 'dots',
    size: 'md',
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Variants',
          content: (
            <div className="space-y-4">
              <div className="flex items-center gap-2">
                <span className="text-xs text-gray-400 w-12">Dots:</span>
                <TypingIndicator variant="dots" />
              </div>
              <div className="flex items-center gap-2">
                <span className="text-xs text-gray-400 w-12">Pulse:</span>
                <TypingIndicator variant="pulse" />
              </div>
              <div className="flex items-center gap-2">
                <span className="text-xs text-gray-400 w-12">Text:</span>
                <TypingIndicator variant="text" />
              </div>
            </div>
          ),
        },
        {
          name: 'Sizes (Dots)',
          content: (
            <div className="flex items-end gap-6">
              <TypingIndicator variant="dots" size="sm" />
              <TypingIndicator variant="dots" size="md" />
              <TypingIndicator variant="dots" size="lg" />
            </div>
          ),
        },
        {
          name: 'Custom Text Messages',
          content: (
            <TypingIndicator
              variant="text"
              messages={['Consulting the oracle...', 'Crunching numbers...', 'Finding the meaning of life...']}
              rotationInterval={1000}
            />
          ),
        },
      ]}
    />
  ),
}

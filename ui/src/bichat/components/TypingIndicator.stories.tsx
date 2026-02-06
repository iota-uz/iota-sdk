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
  args: {},
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Default verbs',
          content: <TypingIndicator />,
        },
        {
          name: 'Custom verbs',
          content: (
            <TypingIndicator
              verbs={['Consulting the oracle', 'Crunching numbers', 'Finding the meaning of life']}
              rotationInterval={1000}
            />
          ),
        },
        {
          name: 'Single verb',
          content: <TypingIndicator verbs={['Analyzing data']} />,
        },
      ]}
    />
  ),
}

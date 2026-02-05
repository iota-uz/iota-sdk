import type { Meta, StoryObj } from '@storybook/react'

import { UserAvatar } from './UserAvatar'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'

const meta: Meta<typeof UserAvatar> = {
  title: 'BiChat/Components/UserAvatar',
  component: UserAvatar,
  parameters: {
    layout: 'centered',
  },
}

export default meta
type Story = StoryObj<typeof UserAvatar>

export const Playground: Story = {
  args: {
    firstName: 'John',
    lastName: 'Doe',
    size: 'md',
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Sizes',
          content: (
            <div className="flex items-end gap-4">
              <UserAvatar firstName="Small" lastName="S" size="sm" />
              <UserAvatar firstName="Medium" lastName="M" size="md" />
              <UserAvatar firstName="Large" lastName="L" size="lg" />
            </div>
          ),
        },
        {
          name: 'Color Determinism (Same names = same color)',
          content: (
            <div className="flex gap-2">
              <UserAvatar firstName="Alice" lastName="Smith" />
              <UserAvatar firstName="Alice" lastName="Smith" />
              <UserAvatar firstName="Bob" lastName="Jones" />
            </div>
          ),
        },
        {
          name: 'Custom Initials',
          content: <UserAvatar firstName="Very Long" lastName="Name" initials="VL" />,
        },
      ]}
    />
  ),
}

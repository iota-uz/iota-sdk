import type { Meta, StoryObj } from '@storybook/react'

import { ActionButton } from './ActionButton'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'
import { smallText, largeText } from '@sb-helpers/textFixtures'

const meta: Meta<typeof ActionButton.Root> = {
  title: 'BiChat/Primitives/ActionButton',
  component: ActionButton.Root,
  parameters: {
    layout: 'centered',
  },
}

export default meta
type Story = StoryObj<typeof ActionButton.Root>

export const Playground: Story = {
  render: (args) => (
    <ActionButton.Root {...args}>
      <ActionButton.Icon>
        <svg
          xmlns="http://www.w3.org/2000/svg"
          width="20"
          height="20"
          viewBox="0 0 256 256"
          fill="currentColor"
        >
          <path d="M200,32H56A16,16,0,0,0,40,48V208a16,16,0,0,0,16,16H200a16,16,0,0,0,16-16V48A16,16,0,0,0,200,32Zm0,176H56V48H200V208ZM168,128a8,8,0,0,1-8,8H96a8,8,0,0,1,0-16h64A8,8,0,0,1,168,128Z" />
        </svg>
      </ActionButton.Icon>
      <ActionButton.Label srOnly>Example Action</ActionButton.Label>
      <ActionButton.Tooltip>Click me</ActionButton.Tooltip>
    </ActionButton.Root>
  ),
  args: {
    className: 'p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-500',
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Large Tooltip Text',
          content: (
            <ActionButton.Root className="p-2 border rounded">
              <ActionButton.Icon>Icon</ActionButton.Icon>
              <ActionButton.Tooltip>{largeText}</ActionButton.Tooltip>
            </ActionButton.Root>
          ),
        },
        {
          name: 'Small Tooltip Text',
          content: (
            <ActionButton.Root className="p-2 border rounded">
              <ActionButton.Icon>Icon</ActionButton.Icon>
              <ActionButton.Tooltip>{smallText}</ActionButton.Tooltip>
            </ActionButton.Root>
          ),
        },
        {
          name: 'Disabled State',
          content: (
            <ActionButton.Root disabled className="p-2 border rounded opacity-50">
              <ActionButton.Icon>Icon</ActionButton.Icon>
              <ActionButton.Tooltip>Should not show</ActionButton.Tooltip>
            </ActionButton.Root>
          ),
        },
      ]}
    />
  ),
}

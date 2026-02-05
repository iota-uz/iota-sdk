import type { Meta, StoryObj } from '@storybook/react'

import { Avatar } from './Avatar'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'
import { largeImageDataUrl } from '@sb-helpers/imageFixtures'

const meta: Meta<typeof Avatar.Root> = {
  title: 'BiChat/Primitives/Avatar',
  component: Avatar.Root,
  parameters: {
    layout: 'centered',
  },
}

export default meta
type Story = StoryObj<typeof Avatar.Root>

export const Playground: Story = {
  render: (args) => (
    <Avatar.Root {...args} className="w-10 h-10 inline-flex items-center justify-center bg-primary-600 text-white rounded-full overflow-hidden">
      <Avatar.Image src="https://github.com/shadcn.png" alt="User" />
      <Avatar.Fallback>JD</Avatar.Fallback>
    </Avatar.Root>
  ),
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Large Image',
          content: (
            <Avatar.Root className="w-12 h-12 inline-flex items-center justify-center bg-gray-200 rounded-full overflow-hidden">
              <Avatar.Image src={largeImageDataUrl} />
              <Avatar.Fallback>LI</Avatar.Fallback>
            </Avatar.Root>
          ),
        },
        {
          name: 'Invalid Image (Fallback)',
          content: (
            <Avatar.Root className="w-12 h-12 inline-flex items-center justify-center bg-gray-200 rounded-full overflow-hidden">
              <Avatar.Image src="invalid-url" />
              <Avatar.Fallback>FB</Avatar.Fallback>
            </Avatar.Root>
          ),
        },
        {
          name: 'No Image (Fallback)',
          content: (
            <Avatar.Root className="w-12 h-12 inline-flex items-center justify-center bg-primary-600 text-white rounded-full overflow-hidden">
              <Avatar.Fallback>JD</Avatar.Fallback>
            </Avatar.Root>
          ),
        },
        {
          name: 'Delayed Fallback',
          content: (
            <Avatar.Root className="w-12 h-12 inline-flex items-center justify-center bg-gray-200 rounded-full overflow-hidden">
              <Avatar.Image src="https://error.com/delay" />
              <Avatar.Fallback delayMs={1000}>DL</Avatar.Fallback>
            </Avatar.Root>
          ),
        },
      ]}
    />
  ),
}

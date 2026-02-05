import type { Meta, StoryObj } from '@storybook/react'

import { Bubble } from './Bubble'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'
import { largeText, veryLargeText } from '@sb-helpers/textFixtures'

const meta: Meta<typeof Bubble.Root> = {
  title: 'BiChat/Primitives/Bubble',
  component: Bubble.Root,
}

export default meta
type Story = StoryObj<typeof Bubble.Root>

export const Playground: Story = {
  render: (args) => (
    <Bubble.Root {...args} className="max-w-[70%] p-4 rounded-2xl bg-white border">
      <Bubble.Header className="text-xs font-semibold mb-1">Header</Bubble.Header>
      <Bubble.Content>Message content goes here.</Bubble.Content>
      <Bubble.Footer className="text-[10px] mt-2 text-gray-400">Footer</Bubble.Footer>
    </Bubble.Root>
  ),
  args: {
    variant: 'assistant',
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'User Variant (Primary)',
          content: (
            <Bubble.Root variant="user" className="bg-primary-600 text-white p-3 rounded-xl ml-auto max-w-[80%]">
              <Bubble.Content>{largeText}</Bubble.Content>
            </Bubble.Root>
          ),
        },
        {
          name: 'Very Large Content',
          content: (
            <Bubble.Root variant="assistant" className="bg-white border p-3 rounded-xl max-w-[90%]">
              <Bubble.Content className="whitespace-pre-wrap">{veryLargeText.slice(0, 1000)}</Bubble.Content>
            </Bubble.Root>
          ),
        },
        {
          name: 'System Variant',
          content: (
            <Bubble.Root variant="system" className="bg-gray-100 text-gray-500 p-2 rounded text-xs text-center mx-auto max-w-sm">
              <Bubble.Content>System message: session archived</Bubble.Content>
            </Bubble.Root>
          ),
        },
        {
          name: 'Empty Content',
          content: (
            <Bubble.Root className="bg-white border p-3 rounded-xl">
              <Bubble.Content></Bubble.Content>
            </Bubble.Root>
          ),
        },
      ]}
    />
  ),
}

import type { Meta, StoryObj } from '@storybook/react'

import { Avatar } from './Avatar'
import { ActionButton } from './ActionButton'
import { Bubble } from './Bubble'
import { Turn } from './Turn'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'
import { largeImageDataUrl } from '@sb-helpers/imageFixtures'
import { largeText, veryLargeText } from '@sb-helpers/textFixtures'

const meta: Meta = {
  title: 'BiChat/Primitives',
  parameters: { layout: 'centered' },
}

export default meta

type Story = StoryObj

export const AvatarPrimitive: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'With image',
          content: (
            <Avatar.Root className="w-12 h-12 inline-flex items-center justify-center bg-primary-600 text-white rounded-full overflow-hidden">
              <Avatar.Image src="https://github.com/shadcn.png" alt="User" />
              <Avatar.Fallback>JD</Avatar.Fallback>
            </Avatar.Root>
          ),
        },
        {
          name: 'Fallback (no image)',
          content: (
            <Avatar.Root className="w-12 h-12 inline-flex items-center justify-center bg-primary-600 text-white rounded-full overflow-hidden">
              <Avatar.Fallback>AB</Avatar.Fallback>
            </Avatar.Root>
          ),
        },
        {
          name: 'Invalid image → fallback',
          content: (
            <Avatar.Root className="w-12 h-12 inline-flex items-center justify-center bg-gray-200 rounded-full overflow-hidden">
              <Avatar.Image src="invalid-url" />
              <Avatar.Fallback>FB</Avatar.Fallback>
            </Avatar.Root>
          ),
        },
        {
          name: 'Large image',
          content: (
            <Avatar.Root className="w-12 h-12 inline-flex items-center justify-center bg-gray-200 rounded-full overflow-hidden">
              <Avatar.Image src={largeImageDataUrl} />
              <Avatar.Fallback>LI</Avatar.Fallback>
            </Avatar.Root>
          ),
        },
      ]}
    />
  ),
}

export const ActionButtonPrimitive: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'With icon and tooltip',
          content: (
            <ActionButton.Root className="p-2 rounded-lg hover:bg-gray-100 border">
              <ActionButton.Icon>
                <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 256 256" fill="currentColor">
                  <path d="M200,32H56A16,16,0,0,0,40,48V208a16,16,0,0,0,16,16H200a16,16,0,0,0,16-16V48A16,16,0,0,0,200,32Z" />
                </svg>
              </ActionButton.Icon>
              <ActionButton.Label srOnly>Example</ActionButton.Label>
              <ActionButton.Tooltip>Click me</ActionButton.Tooltip>
            </ActionButton.Root>
          ),
        },
        {
          name: 'Disabled',
          content: (
            <ActionButton.Root disabled className="p-2 border rounded opacity-50">
              <ActionButton.Icon>Icon</ActionButton.Icon>
              <ActionButton.Tooltip>Disabled</ActionButton.Tooltip>
            </ActionButton.Root>
          ),
        },
      ]}
    />
  ),
}

export const BubblePrimitive: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Assistant',
          content: (
            <Bubble.Root variant="assistant" className="max-w-[70%] p-4 rounded-2xl bg-white border">
              <Bubble.Header className="text-xs font-semibold mb-1">Header</Bubble.Header>
              <Bubble.Content>Message content.</Bubble.Content>
              <Bubble.Footer className="text-[10px] mt-2 text-gray-400">Footer</Bubble.Footer>
            </Bubble.Root>
          ),
        },
        {
          name: 'User',
          content: (
            <Bubble.Root variant="user" className="bg-primary-600 text-white p-3 rounded-xl ml-auto max-w-[80%]">
              <Bubble.Content>{largeText.slice(0, 120)}</Bubble.Content>
            </Bubble.Root>
          ),
        },
        {
          name: 'System',
          content: (
            <Bubble.Root variant="system" className="bg-gray-100 text-gray-500 p-2 rounded text-xs text-center mx-auto max-w-sm">
              <Bubble.Content>Session archived</Bubble.Content>
            </Bubble.Root>
          ),
        },
        {
          name: 'Long content',
          content: (
            <Bubble.Root variant="assistant" className="bg-white border p-3 rounded-xl max-w-[90%]">
              <Bubble.Content className="whitespace-pre-wrap">{veryLargeText.slice(0, 400)}</Bubble.Content>
            </Bubble.Root>
          ),
        },
      ]}
    />
  ),
}

export const TurnPrimitive: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'User + Assistant',
          content: (
            <Turn.Root turnId="turn-1" className="space-y-4">
              <Turn.User className="flex justify-end">
                <div className="bg-primary-600 text-white p-3 rounded-xl max-w-[80%]">User message</div>
              </Turn.User>
              <Turn.Assistant className="flex gap-3">
                <div className="bg-white border p-3 rounded-xl max-w-[80%]">
                  Assistant response
                  <Turn.Timestamp date={new Date()} className="block text-[10px] mt-2 text-gray-400" />
                </div>
              </Turn.Assistant>
            </Turn.Root>
          ),
        },
        {
          name: 'With actions',
          content: (
            <Turn.Root turnId="with-actions">
              <Turn.Assistant className="space-y-2">
                <div className="bg-white border p-3 rounded-xl">Check these actions:</div>
                <Turn.Actions className="flex gap-2">
                  <button type="button" className="text-xs border px-2 py-1 rounded">Copy</button>
                  <button type="button" className="text-xs border px-2 py-1 rounded">Regenerate</button>
                </Turn.Actions>
              </Turn.Assistant>
            </Turn.Root>
          ),
        },
      ]}
    />
  ),
}

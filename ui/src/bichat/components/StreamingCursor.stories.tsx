import type { Meta, StoryObj } from '@storybook/react'

import { StreamingCursor } from './StreamingCursor'

const meta: Meta<typeof StreamingCursor> = {
  title: 'BiChat/Components/StreamingCursor',
  component: StreamingCursor,
  parameters: {
    layout: 'centered',
  },
}

export default meta
type Story = StoryObj<typeof StreamingCursor>

export const Playground: Story = {
  render: () => (
    <div className="flex items-center gap-1 p-4 bg-white border rounded">
      <span>AI is typing</span>
      <StreamingCursor />
    </div>
  ),
}

export const Stress: Story = {
  render: () => (
    <div className="space-y-4">
      <div className="text-sm">In a sentence: This is how it looks when text is being generated<StreamingCursor /></div>
      <div className="text-2xl font-bold">Large font size<StreamingCursor /></div>
      <div className="dark p-4 bg-gray-900 text-white rounded">Dark mode support<StreamingCursor /></div>
    </div>
  ),
}

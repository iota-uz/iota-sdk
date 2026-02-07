import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react'

import { MessageInput } from './MessageInput'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'
import { largeText } from '@sb-helpers/textFixtures'

const meta: Meta<typeof MessageInput> = {
  title: 'BiChat/Components/MessageInput',
  component: MessageInput,
}

export default meta
type Story = StoryObj<typeof MessageInput>

export const Playground: Story = {
  render: (args) => {
    const [message, setMessage] = useState('')
    return (
      <div className="max-w-2xl mx-auto p-4 border rounded bg-gray-50">
        <MessageInput
          {...args}
          message={message}
          onMessageChange={setMessage}
          onSubmit={(_e, atts) => {
            alert(`Submit: "${message}" with ${atts.length} images`)
            setMessage('')
          }}
        />
      </div>
    )
  },
  args: {
    loading: false,
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Loading State',
          content: (
            <MessageInput
              message="Processing message..."
              loading={true}
              onMessageChange={() => {}}
              onSubmit={() => {}}
            />
          ),
        },
        {
          name: 'With Attachments',
          content: (
            <div className="space-y-4">
              {/* Note: Internal state managed by component, but we can see the grid via props if it were exposed. 
                  Since attachments are internal [useState], we test rendering defaults. */}
              <MessageInput
                message="Multiple images attached."
                onMessageChange={() => {}}
                onSubmit={() => {}}
                loading={false}
              />
            </div>
          ),
        },
        {
          name: 'Long Text Input',
          content: (
            <MessageInput
              message={largeText}
              onMessageChange={() => {}}
              onSubmit={() => {}}
              loading={false}
            />
          ),
        },
        {
          name: 'Disabled',
          content: (
            <MessageInput
              message=""
              disabled
              onMessageChange={() => {}}
              onSubmit={() => {}}
              loading={false}
            />
          ),
        },
      ]}
    />
  ),
}

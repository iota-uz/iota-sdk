import type { Meta, StoryObj } from '@storybook/react'
import { ChatHeader } from './ChatHeader'
import { makeSession } from '@sb-helpers/bichatFixtures'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'

const meta: Meta<typeof ChatHeader> = {
  title: 'BiChat/Components/ChatHeader',
  component: ChatHeader,
}

export default meta
type Story = StoryObj<typeof ChatHeader>

export const Playground: Story = {
  args: {
    session: makeSession({ title: 'Current Analysis' }),
    onBack: () => alert('Back clicked'),
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      columns={1}
      scenarios={[
        {
          name: 'New Chat (No Session)',
          content: <ChatHeader session={null} />,
        },
        {
          name: 'Pinned Session',
          content: <ChatHeader session={makeSession({ title: 'Pinned Analysis', pinned: true })} />,
        },
        {
          name: 'Archived Session',
          content: <ChatHeader session={makeSession({ title: 'Old Analysis', status: 'archived' })} />,
        },
        {
          name: 'Long Title',
          content: (
            <ChatHeader
              session={makeSession({
                title: 'This is an extremely long title for a chat session to test how it interacts with the actions slot and overall layout.',
              })}
            />
          ),
        },
        {
          name: 'With Custom Logo & Actions',
          content: (
            <ChatHeader
              session={makeSession({ title: 'Branded Chat' })}
              logoSlot={<div className="font-bold text-primary-600">MYCORP</div>}
              actionsSlot={<button className="text-xs bg-gray-100 px-2 py-1 rounded">Action</button>}
            />
          ),
        },
      ]}
    />
  ),
}

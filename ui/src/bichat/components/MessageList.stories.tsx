import type { Meta, StoryObj } from '@storybook/react'

import { MessageList } from './MessageList'
import { ChatSessionProvider } from '../context/ChatContext'
import { MockChatDataSource } from '@sb-helpers/mockChatDataSource'
import { turnsShort, turnsLong } from '@sb-helpers/bichatFixtures'

const meta: Meta<typeof MessageList> = {
  title: 'BiChat/Components/MessageList',
  component: MessageList,
}

export default meta
type Story = StoryObj<typeof MessageList>

export const Playground: Story = {
  render: () => (
    <ChatSessionProvider dataSource={new MockChatDataSource({ turns: turnsShort })}>
      <div className="h-[500px] border rounded bg-white dark:bg-gray-900 overflow-hidden flex flex-col">
        <MessageList />
      </div>
    </ChatSessionProvider>
  ),
}

export const Stress: Story = {
  render: () => (
    <div className="space-y-8">
      <section>
        <h3 className="text-sm font-semibold mb-2">Long Thread (20+ turns)</h3>
        <ChatSessionProvider dataSource={new MockChatDataSource({ turns: turnsLong })}>
          <div className="h-[600px] border rounded bg-white dark:bg-gray-900 overflow-hidden flex flex-col">
            <MessageList />
          </div>
        </ChatSessionProvider>
      </section>

      <section>
        <h3 className="text-sm font-semibold mb-2">Empty State</h3>
        <ChatSessionProvider dataSource={new MockChatDataSource({ turns: [] })}>
          <div className="h-[300px] border rounded bg-white dark:bg-gray-900 overflow-hidden flex flex-col">
            <MessageList />
          </div>
        </ChatSessionProvider>
      </section>
    </div>
  ),
}

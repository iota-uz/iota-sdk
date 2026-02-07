import type { Meta, StoryObj } from '@storybook/react'

import { ChatSession } from './ChatSession'
import { MockChatDataSource } from '@sb-helpers/mockChatDataSource'
import { makeSession, turnsShort, turnsLong, makePendingQuestion } from '@sb-helpers/bichatFixtures'

const meta: Meta<typeof ChatSession> = {
  title: 'BiChat/Components/ChatSession',
  component: ChatSession,
  parameters: {
    layout: 'fullscreen',
  },
}

export default meta
type Story = StoryObj<typeof ChatSession>

export const Playground: Story = {
  args: {
    dataSource: new MockChatDataSource({
      session: makeSession(),
      turns: turnsShort,
    }),
    sessionId: 'session-1',
  },
  render: (args) => (
    <div className="h-screen flex flex-col">
      <ChatSession {...args} />
    </div>
  ),
}

export const Stress: Story = {
  render: () => (
    <div className="space-y-12 p-8 bg-gray-100">
      <section className="h-[600px] border shadow-xl rounded-xl overflow-hidden flex flex-col bg-white">
        <h3 className="p-4 bg-gray-800 text-white font-mono text-xs uppercase tracking-widest">Scenario: Long Thread + Active Streaming</h3>
        <ChatSession
          dataSource={new MockChatDataSource({
            session: makeSession({ title: 'Stress Test: Long Thread' }),
            turns: turnsLong,
            streamingDelay: 50,
          })}
          sessionId="long-thread"
        />
      </section>

      <section className="h-[600px] border shadow-xl rounded-xl overflow-hidden flex flex-col bg-white">
        <h3 className="p-4 bg-gray-800 text-white font-mono text-xs uppercase tracking-widest">Scenario: New Session (Welcome Screen)</h3>
        <ChatSession
          dataSource={new MockChatDataSource({
            session: undefined,
            turns: [],
          })}
          sessionId="new"
        />
      </section>

      <section className="h-[600px] border shadow-xl rounded-xl overflow-hidden flex flex-col bg-white">
        <h3 className="p-4 bg-gray-800 text-white font-mono text-xs uppercase tracking-widest">Scenario: Pending HITL Question</h3>
        <ChatSession
          dataSource={new MockChatDataSource({
            session: makeSession({ title: 'HITL Stress' }),
            turns: turnsShort,
            pendingQuestion: makePendingQuestion(),
          })}
          sessionId="hitl"
        />
      </section>
    </div>
  ),
}

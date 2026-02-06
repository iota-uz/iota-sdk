import type { Meta, StoryObj } from '@storybook/react'

import { ChatSessionProvider, useChat } from './ChatContext'
import { MockChatDataSource } from '@sb-helpers/mockChatDataSource'
import { turnsShort } from '@sb-helpers/bichatFixtures'

const meta: Meta<typeof ChatSessionProvider> = {
  title: 'BiChat/Context/ChatContext',
  component: ChatSessionProvider,
}

export default meta
type Story = StoryObj<typeof ChatSessionProvider>

const Consumer = () => {
  const { session, turns, loading, sendMessage } = useChat()
  return (
    <div className="p-4 border rounded bg-white space-y-2">
      <div className="text-xs font-bold text-gray-400 uppercase">Chat Context State</div>
      <p>Session: {session?.title || 'None'}</p>
      <p>Turns: {turns.length}</p>
      <p>Loading: {loading ? 'Yes' : 'No'}</p>
      <button
        onClick={() => sendMessage('Hello from Storybook')}
        className="px-3 py-1 bg-primary-600 text-white rounded text-sm"
      >
        Send Mock Message
      </button>
    </div>
  )
}

export const Playground: Story = {
  render: () => (
    <ChatSessionProvider dataSource={new MockChatDataSource({ turns: turnsShort })}>
      <Consumer />
    </ChatSessionProvider>
  ),
}

export const Stress: Story = {
  render: () => (
    <div className="space-y-8">
      <section>
        <h4 className="text-sm font-semibold mb-2">Streaming State (Slow)</h4>
        <ChatSessionProvider
          dataSource={new MockChatDataSource({ turns: turnsShort, streamingDelay: 200 })}
        >
          <Consumer />
        </ChatSessionProvider>
      </section>
      <section>
        <h4 className="text-sm font-semibold mb-2">Initial Loading</h4>
        {/* We can't easily trigger the exact loading state from outside, 
            but we could mock a data source that never resolves. */}
        <ChatSessionProvider
          dataSource={{
            createSession: () => new Promise(() => {}),
            fetchSession: () => new Promise(() => {}),
            sendMessage: async function* () {},
            clearSessionHistory: async () => ({ success: true, deletedMessages: 0, deletedArtifacts: 0 }),
            compactSessionHistory: async () => ({ success: true, summary: 'Compacted', deletedMessages: 0, deletedArtifacts: 0 }),
            submitQuestionAnswers: async () => ({ success: true }),
            cancelPendingQuestion: async () => ({ success: true }),
            navigateToSession: () => {},
            listSessions: async () => ({ sessions: [], total: 0, hasMore: false }),
            archiveSession: async () => ({ id: '', title: '', status: 'archived' as const, pinned: false, createdAt: '', updatedAt: '' }),
            unarchiveSession: async () => ({ id: '', title: '', status: 'active' as const, pinned: false, createdAt: '', updatedAt: '' }),
            pinSession: async () => ({ id: '', title: '', status: 'active' as const, pinned: true, createdAt: '', updatedAt: '' }),
            unpinSession: async () => ({ id: '', title: '', status: 'active' as const, pinned: false, createdAt: '', updatedAt: '' }),
            deleteSession: async () => {},
            renameSession: async () => ({ id: '', title: '', status: 'active' as const, pinned: false, createdAt: '', updatedAt: '' }),
            regenerateSessionTitle: async () => ({ id: '', title: '', status: 'active' as const, pinned: false, createdAt: '', updatedAt: '' }),
          }}
        >
          <Consumer />
        </ChatSessionProvider>
      </section>
    </div>
  ),
}

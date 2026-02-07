import type { Meta, StoryObj } from '@storybook/react'

import { TurnBubble } from './TurnBubble'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'
import { makeConversationTurn, makeAssistantTurn, makeUserTurn } from '@sb-helpers/bichatFixtures'
import { ChatSessionProvider } from '../context/ChatContext'
import { MockChatDataSource } from '@sb-helpers/mockChatDataSource'

const meta: Meta<typeof TurnBubble> = {
  title: 'BiChat/Components/TurnBubble',
  component: TurnBubble,
  decorators: [
    (Story) => (
      <ChatSessionProvider dataSource={new MockChatDataSource()}>
        <Story />
      </ChatSessionProvider>
    ),
  ],
}

export default meta
type Story = StoryObj<typeof TurnBubble>

export const Playground: Story = {
  args: {
    turn: makeConversationTurn({
      userTurn: makeUserTurn({ content: 'What is the current status?' }),
      assistantTurn: makeAssistantTurn({ content: 'Everything is looking good.' }),
    }),
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Streaming State',
          content: (
            <TurnBubble
              isStreaming
              turn={makeConversationTurn({
                userTurn: makeUserTurn({ content: 'How about now?' }),
                assistantTurn: makeAssistantTurn({ content: 'Still loading...' }),
              })}
            />
          ),
        },
        {
          name: 'User Only (Waiting)',
          content: (
            <TurnBubble
              turn={makeConversationTurn({
                userTurn: makeUserTurn({ content: 'Waiting for response...' }),
                assistantTurn: undefined,
              })}
            />
          ),
        },
        {
          name: 'Custom Styles',
          content: (
            <TurnBubble
              classNames={{ root: 'p-4 bg-primary-50 dark:bg-primary-900/10 rounded-3xl' }}
              turn={makeConversationTurn({
                userTurn: makeUserTurn({ content: 'Custom look.' }),
                assistantTurn: makeAssistantTurn({ content: 'Indeed.' }),
              })}
            />
          ),
        },
      ]}
    />
  ),
}

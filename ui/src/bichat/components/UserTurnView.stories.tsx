import type { Meta, StoryObj } from '@storybook/react'

import { UserTurnView } from './UserTurnView'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'
import { makeConversationTurn, makeAttachment } from '@sb-helpers/bichatFixtures'
import { largeText } from '@sb-helpers/textFixtures'
import { ChatSessionProvider } from '../context/ChatContext'
import { MockChatDataSource } from '@sb-helpers/mockChatDataSource'

const meta: Meta<typeof UserTurnView> = {
  title: 'BiChat/Components/UserTurnView',
  component: UserTurnView,
  decorators: [
    (Story) => (
      <ChatSessionProvider dataSource={new MockChatDataSource()}>
        <Story />
      </ChatSessionProvider>
    ),
  ],
}

export default meta
type Story = StoryObj<typeof UserTurnView>

export const Playground: Story = {
  args: {
    turn: makeConversationTurn({
      userTurn: {
        id: 'u1',
        content: 'How many orders were placed today?',
        attachments: [],
        createdAt: new Date().toISOString(),
      },
    }),
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Large Text',
          content: (
            <UserTurnView
              turn={makeConversationTurn({
                userTurn: {
                  id: 'u2',
                  content: largeText,
                  attachments: [],
                  createdAt: new Date().toISOString(),
                },
              })}
            />
          ),
        },
        {
          name: 'With Attachments',
          content: (
            <UserTurnView
              turn={makeConversationTurn({
                userTurn: {
                  id: 'u3',
                  content: 'Check these files.',
                  attachments: [
                    makeAttachment({ filename: 'report.pdf' }),
                    makeAttachment({ filename: 'data.xlsx' }),
                  ],
                  createdAt: new Date().toISOString(),
                },
              })}
            />
          ),
        },
        {
          name: 'No Avatar & Actions',
          content: (
            <UserTurnView
              hideAvatar
              hideActions
              turn={makeConversationTurn({
                userTurn: {
                  id: 'u4',
                  content: 'Minimal view.',
                  attachments: [],
                  createdAt: new Date().toISOString(),
                },
              })}
            />
          ),
        },
      ]}
    />
  ),
}

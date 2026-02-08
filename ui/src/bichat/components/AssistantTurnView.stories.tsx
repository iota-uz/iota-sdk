import type { Meta, StoryObj } from '@storybook/react'

import { AssistantTurnView } from './AssistantTurnView'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'
import { makeAssistantTurn, makeConversationTurn } from '@sb-helpers/bichatFixtures'
import { flowingMarkdown, veryLargeText } from '@sb-helpers/textFixtures'
import { ChatSessionProvider } from '../context/ChatContext'
import { MockChatDataSource } from '@sb-helpers/mockChatDataSource'

const meta: Meta<typeof AssistantTurnView> = {
  title: 'BiChat/Components/AssistantTurnView',
  component: AssistantTurnView,
  decorators: [
    (Story) => (
      <ChatSessionProvider dataSource={new MockChatDataSource()}>
        <Story />
      </ChatSessionProvider>
    ),
  ],
}

export default meta
type Story = StoryObj<typeof AssistantTurnView>

export const Playground: Story = {
  args: {
    turn: makeConversationTurn({
      assistantTurn: makeAssistantTurn({ content: flowingMarkdown }),
    }),
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Very Large Markdown',
          content: (
            <AssistantTurnView
              turn={makeConversationTurn({
                assistantTurn: makeAssistantTurn({ content: veryLargeText.slice(0, 3000) }),
              })}
            />
          ),
        },
        {
          name: 'Streaming State',
          content: (
            <AssistantTurnView
              isStreaming
              turn={makeConversationTurn({
                assistantTurn: makeAssistantTurn({ content: 'Generating report...' }),
              })}
            />
          ),
        },
        {
          name: 'No Content (Error/Empty)',
          content: (
            <AssistantTurnView
              turn={makeConversationTurn({
                assistantTurn: makeAssistantTurn({ content: '' }),
              })}
            />
          ),
        },
      ]}
    />
  ),
}

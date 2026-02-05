import type { Meta, StoryObj } from '@storybook/react'

import { AssistantMessage } from './AssistantMessage'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'
import { makeAssistantTurn } from '@sb-helpers/bichatFixtures'
import { flowingMarkdown } from '@sb-helpers/textFixtures'

const meta: Meta<typeof AssistantMessage> = {
  title: 'BiChat/Components/AssistantMessage',
  component: AssistantMessage,
}

export default meta
type Story = StoryObj<typeof AssistantMessage>

export const Playground: Story = {
  args: {
    turn: makeAssistantTurn({ content: 'Hello! How can I assist you today?' }),
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Rich Markdown',
          content: <AssistantMessage turn={makeAssistantTurn({ content: flowingMarkdown })} />,
        },
        {
          name: 'With Actions',
          content: (
            <AssistantMessage
              turn={makeAssistantTurn({ content: 'This message has actions.' })}
              onCopy={() => {}}
              onRegenerate={() => {}}
            />
          ),
        },
        {
          name: 'Streaming Cursor',
          content: <AssistantMessage isStreaming turn={makeAssistantTurn({ content: 'Currently thinking about your request' })} />,
        },
      ]}
    />
  ),
}

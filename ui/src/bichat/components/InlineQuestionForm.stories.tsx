import type { Meta, StoryObj } from '@storybook/react'

import { InlineQuestionForm } from './InlineQuestionForm'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'
import { makePendingQuestion } from '@sb-helpers/bichatFixtures'
import { ChatSessionProvider } from '../context/ChatContext'
import { MockChatDataSource } from '@sb-helpers/mockChatDataSource'

const meta: Meta<typeof InlineQuestionForm> = {
  title: 'BiChat/Components/InlineQuestionForm',
  component: InlineQuestionForm,
  decorators: [
    (Story) => (
      <ChatSessionProvider dataSource={new MockChatDataSource()}>
        <div className="max-w-xl mx-auto p-4 border rounded bg-gray-50 dark:bg-gray-900">
          <Story />
        </div>
      </ChatSessionProvider>
    ),
  ],
}

export default meta
type Story = StoryObj<typeof InlineQuestionForm>

export const Playground: Story = {
  args: {
    pendingQuestion: makePendingQuestion(),
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      columns={1}
      scenarios={[
        {
          name: 'Multi-Step (3 questions)',
          content: (
            <InlineQuestionForm
              pendingQuestion={makePendingQuestion({
                questions: [
                  { id: 'q1', text: 'Step 1: Choice?', type: 'SINGLE_CHOICE', options: [{ id: 'o1', label: 'Yes', value: 'y' }] },
                  { id: 'q2', text: 'Step 2: Multi?', type: 'MULTIPLE_CHOICE', options: [{ id: 'o2', label: 'A', value: 'a' }] },
                  { id: 'q3', text: 'Step 3: Other?', type: 'SINGLE_CHOICE', options: [] },
                ],
              })}
            />
          ),
        },
        {
          name: 'Long Question Text',
          content: (
            <InlineQuestionForm
              pendingQuestion={makePendingQuestion({
                questions: [
                  {
                    id: 'q_long',
                    text: 'This is an extremely long question text to see how it wraps and if it pushes the navigation buttons out of view in small containers. It should still be readable and the form should remain functional.',
                    type: 'SINGLE_CHOICE',
                    options: [{ id: 'o_long', label: 'Option 1', value: '1' }],
                  },
                ],
              })}
            />
          ),
        },
      ]}
    />
  ),
}

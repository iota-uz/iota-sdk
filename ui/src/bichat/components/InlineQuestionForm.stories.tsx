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

/**
 * Multi-step wizard: 4 questions mixing single/multi-choice with realistic
 * content. Click "Next" after selecting an option to advance through the
 * progress bar and see the Back/Next navigation.
 */
export const MultiStepWizard: Story = {
  args: {
    pendingQuestion: makePendingQuestion({
      questions: [
        {
          id: 'q-region',
          text: 'Which regions should the report cover?',
          type: 'MULTIPLE_CHOICE',
          required: true,
          options: [
            { id: 'o-emea', label: 'EMEA', value: 'EMEA' },
            { id: 'o-apac', label: 'APAC', value: 'APAC' },
            { id: 'o-amer', label: 'Americas', value: 'AMER' },
          ],
        },
        {
          id: 'q-metric',
          text: 'Which metric do you want to focus on?',
          type: 'SINGLE_CHOICE',
          required: true,
          options: [
            { id: 'o-rev', label: 'Revenue', value: 'revenue' },
            { id: 'o-margin', label: 'Gross Margin', value: 'margin' },
            { id: 'o-orders', label: 'Order Count', value: 'orders' },
            { id: 'o-aov', label: 'Average Order Value', value: 'aov' },
          ],
        },
        {
          id: 'q-period',
          text: 'What time period should we compare against?',
          type: 'SINGLE_CHOICE',
          required: true,
          options: [
            { id: 'o-prev-q', label: 'Previous Quarter', value: 'prev_quarter' },
            { id: 'o-yoy', label: 'Same Quarter Last Year', value: 'yoy' },
          ],
        },
        {
          id: 'q-format',
          text: 'Any special formatting preferences?',
          type: 'MULTIPLE_CHOICE',
          required: false,
          options: [
            { id: 'o-chart', label: 'Include charts', value: 'charts' },
            { id: 'o-table', label: 'Include data table', value: 'table' },
            { id: 'o-export', label: 'Generate Excel export', value: 'excel' },
          ],
        },
      ],
    }),
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
          name: 'Single Question (no wizard)',
          description: 'Only one question — no progress bar or Back button shown',
          content: (
            <InlineQuestionForm pendingQuestion={makePendingQuestion()} />
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
        {
          name: 'Optional Question (skip allowed)',
          description: 'required=false — Submit is enabled even with no selection',
          content: (
            <InlineQuestionForm
              pendingQuestion={makePendingQuestion({
                questions: [
                  {
                    id: 'q_opt',
                    text: 'Any additional comments? (optional)',
                    type: 'SINGLE_CHOICE',
                    required: false,
                    options: [
                      { id: 'o_yes', label: 'Add a note', value: 'yes' },
                      { id: 'o_no', label: 'No thanks', value: 'no' },
                    ],
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

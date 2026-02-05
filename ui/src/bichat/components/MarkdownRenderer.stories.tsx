import type { Meta, StoryObj } from '@storybook/react'

import { MarkdownRenderer } from './MarkdownRenderer'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'
import { flowingMarkdown, veryLargeText } from '@sb-helpers/textFixtures'
import { makeCitation } from '@sb-helpers/bichatFixtures'

const meta: Meta<typeof MarkdownRenderer> = {
  title: 'BiChat/Components/MarkdownRenderer',
  component: MarkdownRenderer,
}

export default meta
type Story = StoryObj<typeof MarkdownRenderer>

export const Playground: Story = {
  args: {
    content: flowingMarkdown,
    citations: [
      makeCitation({ id: '1', title: 'Reference Source' }),
    ],
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Very Large Text',
          content: <MarkdownRenderer content={veryLargeText.slice(0, 2000)} />,
        },
        {
          name: 'Complex Markdown',
          content: (
            <MarkdownRenderer
              content={[
                '# Tables & Code',
                '',
                '| A | B | C |',
                '|---|---|---|',
                '| 1 | 2 | 3 |',
                '',
                '```javascript',
                'console.log("Hello Storybook");',
                '```',
                '',
                '> Blockquotes are styled correctly too.',
              ].join('\n')}
            />
          ),
        },
        {
          name: 'Many Citations',
          content: (
            <MarkdownRenderer
              content="This sentence has many references [1] [2] [3] [4] [5]."
              citations={Array.from({ length: 5 }).map((_, i) =>
                makeCitation({ id: String(i + 1), title: `Ref ${i + 1}` })
              )}
            />
          ),
        },
      ]}
    />
  ),
}

import type { Meta, StoryObj } from '@storybook/react'

import { SourcesPanel } from './SourcesPanel'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'
import { makeCitation } from '@sb-helpers/bichatFixtures'
import { largeText } from '@sb-helpers/textFixtures'

const meta: Meta<typeof SourcesPanel> = {
  title: 'BiChat/Components/SourcesPanel',
  component: SourcesPanel,
}

export default meta
type Story = StoryObj<typeof SourcesPanel>

export const Playground: Story = {
  args: {
    citations: [
      makeCitation({ title: 'Quarterly Report Q4', url: 'https://example.com/q4' }),
      makeCitation({ title: 'Customer Feedback Data', url: 'https://example.com/feedback' }),
    ],
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Many Sources',
          content: (
            <SourcesPanel
              citations={Array.from({ length: 12 }).map((_, i) =>
                makeCitation({ title: `Source ${i + 1}`, url: `#${i}` })
              )}
            />
          ),
        },
        {
          name: 'Long Excerpts',
          content: (
            <SourcesPanel
              citations={[
                makeCitation({
                  title: 'Long Excerpt Source',
                  excerpt: largeText,
                }),
              ]}
            />
          ),
        },
        {
          name: 'No URLs',
          content: (
            <SourcesPanel
              citations={[
                makeCitation({ title: 'Offline Document', url: '' }),
                makeCitation({ title: 'Internal Knowledge Base', url: '' }),
              ]}
            />
          ),
        },
      ]}
    />
  ),
}

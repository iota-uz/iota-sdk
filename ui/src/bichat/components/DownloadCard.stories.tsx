import type { Meta, StoryObj } from '@storybook/react'

import { DownloadCard } from './DownloadCard'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'
import { makeArtifacts } from '@sb-helpers/bichatFixtures'
import { largeText } from '@sb-helpers/textFixtures'

const meta: Meta<typeof DownloadCard> = {
  title: 'BiChat/Components/DownloadCard',
  component: DownloadCard,
}

export default meta
type Story = StoryObj<typeof DownloadCard>

export const Playground: Story = {
  args: {
    artifact: makeArtifacts()[0],
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'PDF Variant',
          content: <DownloadCard artifact={makeArtifacts()[1]} />,
        },
        {
          name: 'Long Filename & Description',
          content: (
            <DownloadCard
              artifact={{
                type: 'excel',
                filename: 'Quarterly_Regional_Revenue_Breakdown_FY2024_Final_v2_Consolidated.xlsx',
                url: '#',
                description: largeText,
                sizeReadable: '12.5 MB',
                rowCount: 15420,
              }}
            />
          ),
        },
        {
          name: 'Minimal Artifact',
          content: (
            <DownloadCard
              artifact={{
                type: 'excel',
                filename: 'data.csv',
                url: '#',
              }}
            />
          ),
        },
      ]}
    />
  ),
}

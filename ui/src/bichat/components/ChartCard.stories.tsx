import type { Meta, StoryObj } from '@storybook/react'
import { ChartCard } from './ChartCard'
import { makeChartData } from '@sb-helpers/bichatFixtures'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'

const meta: Meta<typeof ChartCard> = {
  title: 'BiChat/Components/ChartCard',
  component: ChartCard,
}

export default meta
type Story = StoryObj<typeof ChartCard>

export const Playground: Story = {
  args: {
    chartData: {
      ...makeChartData(),
      title: 'Revenue Trends',
    },
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Many Data Points',
          content: (
            <ChartCard
              chartData={{
                chartType: 'line',
                title: 'Stress Test Chart',
                series: [
                  {
                    name: 'Series 1',
                    data: Array.from({ length: 50 }).map(() => Math.random() * 100),
                  },
                ],
                labels: Array.from({ length: 50 }).map((_, i) => `Day ${i + 1}`),
              }}
            />
          ),
        },
        {
          name: 'Long Title',
          content: (
            <ChartCard
              chartData={{
                ...makeChartData(),
                title: 'This is an extremely long title for a chart card component to test its layout resilience and wrapping behavior',
              }}
            />
          ),
        },
        {
          name: 'Minimal Chart',
          content: <ChartCard chartData={makeChartData()} />,
        },
      ]}
    />
  ),
}

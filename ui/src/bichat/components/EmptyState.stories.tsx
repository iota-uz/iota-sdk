import type { Meta, StoryObj } from '@storybook/react'

import { EmptyState } from './EmptyState'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'
import { MagnifyingGlass, Ghost } from '@phosphor-icons/react'

const meta: Meta<typeof EmptyState> = {
  title: 'BiChat/Components/EmptyState',
  component: EmptyState,
}

export default meta
type Story = StoryObj<typeof EmptyState>

export const Playground: Story = {
  args: {
    title: 'No results found',
    description: 'Try adjusting your search or filters to find what you are looking for.',
    icon: <MagnifyingGlass size={48} className="text-gray-300" />,
    action: <button className="px-4 py-2 border rounded">Clear Search</button>,
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Sizes',
          content: (
            <div className="space-y-8">
              <EmptyState size="sm" title="Small Empty State" description="Fits in sidebars." />
              <EmptyState size="md" title="Medium (Default)" description="Standard page empty state." />
              <EmptyState size="lg" title="Large Hero" description="Main center-page landing empty state." />
            </div>
          ),
        },
        {
          name: 'Minimal',
          content: <EmptyState title="Just a title" />,
        },
        {
          name: 'Overflow Icon/Text',
          content: (
            <EmptyState
              title="Very long title that should wrap correctly without breaking the flex center alignment of the empty state component"
              description="This description is also quite long to test the typography scale and readability when there is a lot of content in an empty state."
              icon={<Ghost size={120} weight="fill" className="text-primary-100" />}
            />
          ),
        },
      ]}
    />
  ),
}

import type { Meta, StoryObj } from '@storybook/react'

import { Skeleton, SkeletonText, SkeletonAvatar, SkeletonCard } from './Skeleton'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'

const meta: Meta<typeof Skeleton> = {
  title: 'BiChat/Components/Skeleton',
  component: Skeleton,
}

export default meta
type Story = StoryObj<typeof Skeleton>

export const Playground: Story = {
  args: {
    variant: 'rounded',
    width: 200,
    height: 100,
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Variants',
          content: (
            <div className="space-y-4">
              <Skeleton variant="text" width="60%" />
              <Skeleton variant="circular" width={40} height={40} />
              <Skeleton variant="rounded" width="100%" height={60} />
              <Skeleton variant="rectangular" width="100%" height={40} />
            </div>
          ),
        },
        {
          name: 'Helpers',
          content: (
            <div className="space-y-6">
              <div>
                <div className="text-xs text-gray-400 mb-2">SkeletonText (3 lines):</div>
                <SkeletonText lines={3} />
              </div>
              <div className="flex items-center gap-3">
                <SkeletonAvatar size={48} />
                <div className="flex-1">
                  <SkeletonText lines={2} />
                </div>
              </div>
              <SkeletonCard height={100} />
            </div>
          ),
        },
        {
          name: 'No Animation',
          content: <Skeleton variant="rounded" width="100%" height={100} animate={false} />,
        },
      ]}
    />
  ),
}

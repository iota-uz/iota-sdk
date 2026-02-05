import type { Meta, StoryObj } from '@storybook/react'

import { ScrollToBottomButton } from './ScrollToBottomButton'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'

const meta: Meta<typeof ScrollToBottomButton> = {
  title: 'BiChat/Components/ScrollToBottomButton',
  component: ScrollToBottomButton,
  parameters: {
    layout: 'centered',
  },
}

export default meta
type Story = StoryObj<typeof ScrollToBottomButton>

export const Playground: Story = {
  args: {
    show: true,
    onClick: () => alert('Clicked'),
    unreadCount: 5,
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Unread Counts',
          content: (
            <div className="flex gap-12 h-32 items-center relative border rounded bg-gray-50 overflow-hidden">
              <ScrollToBottomButton show onClick={() => {}} unreadCount={0} />
              <div className="mx-12" />
              <ScrollToBottomButton show onClick={() => {}} unreadCount={9} />
              <div className="mx-12" />
              <ScrollToBottomButton show onClick={() => {}} unreadCount={123} />
              <div className="absolute inset-0 flex items-center justify-center text-[10px] text-gray-300 pointer-events-none">
                (Positions relative to container)
              </div>
            </div>
          ),
        },
      ]}
    />
  ),
}

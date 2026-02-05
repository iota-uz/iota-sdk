import type { Meta, StoryObj } from '@storybook/react'

import { Toast } from './Toast'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'

const meta: Meta<typeof Toast> = {
  title: 'BiChat/Components/Toast',
  component: Toast,
  parameters: {
    layout: 'centered',
  },
}

export default meta
type Story = StoryObj<typeof Toast>

export const Playground: Story = {
  args: {
    id: '1',
    type: 'success',
    message: 'Settings saved successfully.',
    onDismiss: () => {},
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'All Types',
          content: (
            <div className="flex flex-col gap-2">
              <Toast id="s" type="success" message="Success toast message." onDismiss={() => {}} />
              <Toast id="e" type="error" message="Error toast message." onDismiss={() => {}} />
              <Toast id="i" type="info" message="Info toast message." onDismiss={() => {}} />
              <Toast id="w" type="warning" message="Warning toast message." onDismiss={() => {}} />
            </div>
          ),
        },
        {
          name: 'Long Message',
          content: (
            <Toast
              id="l"
              type="error"
              message="This is an extremely long toast message that might wrap to multiple lines depending on the container width. It should still look good and remain readable."
              onDismiss={() => {}}
            />
          ),
        },
      ]}
    />
  ),
}

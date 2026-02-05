import type { Meta, StoryObj } from '@storybook/react'

import { ToastContainer } from './ToastContainer'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'

const meta: Meta<typeof ToastContainer> = {
  title: 'BiChat/Components/ToastContainer',
  component: ToastContainer,
}

export default meta
type Story = StoryObj<typeof ToastContainer>

export const Playground: Story = {
  args: {
    toasts: [
      { id: '1', type: 'success', message: 'Operation completed successfully.' },
      { id: '2', type: 'info', message: 'New updates available.' },
    ],
    onDismiss: (id) => console.log('Dismiss toast', id),
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Multiple Toasts (Stacking)',
          content: (
            <div className="h-[300px] relative border border-dashed rounded-lg overflow-hidden">
              <ToastContainer
                toasts={[
                  { id: '1', type: 'error', message: 'Failed to save changes. Please try again later.' },
                  { id: '2', type: 'warning', message: 'Your session will expire in 5 minutes.' },
                  { id: '3', type: 'success', message: 'File uploaded successfully.' },
                  { id: '4', type: 'info', message: 'Welcome back, Storybook User!' },
                ]}
                onDismiss={() => {}}
              />
              <div className="absolute inset-0 flex items-center justify-center text-gray-300 pointer-events-none">
                (Container Viewport)
              </div>
            </div>
          ),
        },
      ]}
    />
  ),
}

import type { Meta, StoryObj } from '@storybook/react'

import { Turn } from './Turn'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'

const meta: Meta<typeof Turn.Root> = {
  title: 'BiChat/Primitives/Turn',
  component: Turn.Root,
}

export default meta
type Story = StoryObj<typeof Turn.Root>

export const Playground: Story = {
  render: (args) => (
    <Turn.Root {...args} className="space-y-4">
      <Turn.User className="flex justify-end">
        <div className="bg-primary-600 text-white p-3 rounded-xl max-w-[80%]">
          User message here
        </div>
      </Turn.User>
      <Turn.Assistant className="flex gap-3">
        <div className="bg-white border p-3 rounded-xl max-w-[80%]">
          Assistant response here
          <Turn.Timestamp date={new Date()} className="block text-[10px] mt-2 text-gray-400" />
        </div>
      </Turn.Assistant>
    </Turn.Root>
  ),
  args: {
    turnId: 'turn-1',
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'User Only Turn',
          content: (
            <Turn.Root turnId="user-only">
              <Turn.User className="flex justify-end">
                <div className="bg-primary-600 text-white p-3 rounded-xl">Waiting for response...</div>
              </Turn.User>
            </Turn.Root>
          ),
        },
        {
          name: 'Turn with Actions',
          content: (
            <Turn.Root turnId="with-actions">
              <Turn.Assistant className="space-y-2">
                <div className="bg-white border p-3 rounded-xl">Check out these actions:</div>
                <Turn.Actions className="flex gap-2">
                  <button className="text-xs border px-2 py-1 rounded">Copy</button>
                  <button className="text-xs border px-2 py-1 rounded">Regenerate</button>
                </Turn.Actions>
              </Turn.Assistant>
            </Turn.Root>
          ),
        },
      ]}
    />
  ),
}

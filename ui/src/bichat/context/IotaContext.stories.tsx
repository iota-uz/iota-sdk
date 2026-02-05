import type { Meta, StoryObj } from '@storybook/react'

import { IotaContextProvider, useIotaContext } from './IotaContext'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'

const meta: Meta<typeof IotaContextProvider> = {
  title: 'BiChat/Context/IotaContext',
  component: IotaContextProvider,
  parameters: {
    // Disable the global decorator for this specific story to avoid double-providing
    bichat: { disableIotaProvider: true },
  },
}

export default meta
type Story = StoryObj<typeof IotaContextProvider>

const Consumer = () => {
  const ctx = useIotaContext()
  return (
    <div className="p-4 border rounded bg-white font-mono text-xs whitespace-pre-wrap overflow-auto max-h-[300px]">
      {JSON.stringify(ctx, null, 2)}
    </div>
  )
}

export const Playground: Story = {
  render: () => (
    <IotaContextProvider>
      <Consumer />
    </IotaContextProvider>
  ),
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Missing Context (Throws/Fallback)',
          content: (
            <div className="p-4 border border-dashed text-red-500 text-sm">
              Note: Components using useIotaContext outside of provider will throw an error.
            </div>
          ),
        },
      ]}
    />
  ),
}

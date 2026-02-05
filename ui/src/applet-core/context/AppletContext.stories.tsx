import type { Meta, StoryObj } from '@storybook/react'
import { AppletProvider, useAppletContext } from './AppletContext'

const meta: Meta<typeof AppletProvider> = {
  title: 'AppletCore/Context/AppletContext',
  component: AppletProvider,
}

export default meta
type Story = StoryObj<typeof AppletProvider>

const Consumer = () => {
  const ctx = useAppletContext()
  return (
    <div className="p-4 border rounded bg-white font-mono text-xs whitespace-pre-wrap">
      <div className="text-xs font-bold text-gray-400 uppercase mb-2">Applet Context</div>
      {JSON.stringify(ctx, null, 2)}
    </div>
  )
}

export const Playground: Story = {
  render: () => (
    <AppletProvider windowKey="__BICHAT_CONTEXT__">
      <Consumer />
    </AppletProvider>
  ),
}

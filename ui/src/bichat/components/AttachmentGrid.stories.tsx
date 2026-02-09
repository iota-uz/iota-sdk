import type { Meta, StoryObj } from '@storybook/react'
import { AttachmentGrid } from './AttachmentGrid'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'
import { makeImageAttachment } from '@sb-helpers/bichatFixtures'

const meta: Meta<typeof AttachmentGrid> = {
  title: 'BiChat/Components/AttachmentGrid',
  component: AttachmentGrid,
}

export default meta
type Story = StoryObj<typeof AttachmentGrid>

export const Playground: Story = {
  args: {
    attachments: Array.from({ length: 4 }).map((_, i) => makeImageAttachment({ filename: `image-${i}.png` })),
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: '1 Image',
          content: <AttachmentGrid attachments={[makeImageAttachment({ filename: '1.png' })]} />,
        },
        {
          name: '2 Images',
          content: <AttachmentGrid attachments={[makeImageAttachment({ filename: '1.png' }), makeImageAttachment({ filename: '2.png' })]} />,
        },
        {
          name: '3 Images',
          content: <AttachmentGrid attachments={[makeImageAttachment({ filename: '1.png' }), makeImageAttachment({ filename: '2.png' }), makeImageAttachment({ filename: '3.png' })]} />,
        },
        {
          name: '4+ Images (Overflow indicator)',
          content: (
            <AttachmentGrid
              attachments={Array.from({ length: 6 }).map((_, i) => makeImageAttachment({ filename: `img-${i}.png` }))}
            />
          ),
        },
      ]}
    />
  ),
}

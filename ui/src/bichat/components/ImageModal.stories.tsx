import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react'

import { ImageModal } from './ImageModal'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'
import { makeImageAttachment } from '@sb-helpers/bichatFixtures'
import { largeImageDataUrl, smallImageDataUrl } from '@sb-helpers/imageFixtures'

const meta: Meta<typeof ImageModal> = {
  title: 'BiChat/Components/ImageModal',
  component: ImageModal,
}

export default meta
type Story = StoryObj<typeof ImageModal>

const ModalTrigger = ({ attachment, allAttachments, currentIndex }: any) => {
  const [isOpen, setIsOpen] = useState(false)
  return (
    <div>
      <button onClick={() => setIsOpen(true)} className="px-4 py-2 bg-primary-600 text-white rounded">
        Open Image Modal
      </button>
      <ImageModal
        isOpen={isOpen}
        onClose={() => setIsOpen(false)}
        attachment={attachment}
        allAttachments={allAttachments}
        currentIndex={currentIndex}
      />
    </div>
  )
}

export const Playground: Story = {
  render: () => (
    <ModalTrigger attachment={makeImageAttachment({ preview: smallImageDataUrl })} />
  ),
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Large High-Res Image',
          content: (
            <ModalTrigger
              attachment={makeImageAttachment({
                filename: 'large-photo.jpg',
                preview: largeImageDataUrl,
              })}
            />
          ),
        },
        {
          name: 'With Gallery Navigation',
          content: (
            <ModalTrigger
              currentIndex={1}
              attachment={makeImageAttachment({ filename: 'img-2.png' })}
              allAttachments={[
                makeImageAttachment({ filename: 'img-1.png' }),
                makeImageAttachment({ filename: 'img-2.png' }),
                makeImageAttachment({ filename: 'img-3.png' }),
              ]}
            />
          ),
        },
      ]}
    />
  ),
}

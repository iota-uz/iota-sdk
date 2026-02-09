import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react'

import { ConfirmModal } from './ConfirmModal'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'

const meta: Meta<typeof ConfirmModal> = {
  title: 'BiChat/Components/ConfirmModal',
  component: ConfirmModal,
}

export default meta
type Story = StoryObj<typeof ConfirmModal>

const ModalTrigger = (props: any) => {
  const [isOpen, setIsOpen] = useState(false)
  return (
    <div>
      <button onClick={() => setIsOpen(true)} className="px-4 py-2 border rounded">
        Open {props.title}
      </button>
      <ConfirmModal {...props} isOpen={isOpen} onCancel={() => setIsOpen(false)} onConfirm={() => setIsOpen(false)} />
    </div>
  )
}

export const Playground: Story = {
  render: () => (
    <ModalTrigger
      title="Delete Session"
      message="Are you sure you want to delete this chat session? This action cannot be undone."
      confirmText="Delete"
      isDanger
    />
  ),
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Danger Variant',
          content: (
            <ModalTrigger
              title="Destructive Action"
              message="This will permanently remove all associated data from the servers."
              isDanger
              confirmText="Erase All"
            />
          ),
        },
        {
          name: 'Long Content',
          content: (
            <ModalTrigger
              title="Extremely Long Confirmation Title That Might Wrap"
              message="This message is also very long to test how the modal handle overflow. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat."
            />
          ),
        },
      ]}
    />
  ),
}

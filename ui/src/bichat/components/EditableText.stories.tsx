import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react'

import { EditableText } from './EditableText'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'
import { largeText } from '@sb-helpers/textFixtures'

const meta: Meta<typeof EditableText> = {
  title: 'BiChat/Components/EditableText',
  component: EditableText,
}

export default meta
type Story = StoryObj<typeof EditableText>

export const Playground: Story = {
  render: (args) => {
    const [value, setValue] = useState(args.value || 'Double click me')
    return <EditableText {...args} value={value} onSave={setValue} />
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Sizes',
          content: (
            <div className="space-y-4">
              <EditableText size="sm" value="Small text" onSave={() => {}} />
              <EditableText size="md" value="Medium (Default)" onSave={() => {}} />
              <EditableText size="lg" value="Large title text" onSave={() => {}} />
            </div>
          ),
        },
        {
          name: 'Loading State',
          content: <EditableText value="Saving..." isLoading onSave={() => {}} />,
        },
        {
          name: 'Long Text (Truncation)',
          content: <EditableText value={largeText} onSave={() => {}} className="max-w-[300px]" />,
        },
        {
          name: 'Empty (Placeholder)',
          content: <EditableText value="" placeholder="Untitled Document" onSave={() => {}} />,
        },
      ]}
    />
  ),
}

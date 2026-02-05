import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react'

import { SearchInput } from './SearchInput'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'

const meta: Meta<typeof SearchInput> = {
  title: 'BiChat/Components/SearchInput',
  component: SearchInput,
}

export default meta
type Story = StoryObj<typeof SearchInput>

export const Playground: Story = {
  render: (args) => {
    const [value, setValue] = useState(args.value || '')
    return <SearchInput {...args} value={value} onChange={setValue} />
  },
  args: {
    placeholder: 'Search sessions...',
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
              <SearchInput value="" onChange={() => {}} size="sm" placeholder="Small" />
              <SearchInput value="" onChange={() => {}} size="md" placeholder="Medium (Default)" />
              <SearchInput value="" onChange={() => {}} size="lg" placeholder="Large" />
            </div>
          ),
        },
        {
          name: 'States',
          content: (
            <div className="space-y-4">
              <SearchInput value="With content" onChange={() => {}} />
              <SearchInput value="Disabled input" onChange={() => {}} disabled />
            </div>
          ),
        },
        {
          name: 'Very Long Placeholder',
          content: (
            <SearchInput
              value=""
              onChange={() => {}}
              placeholder="This is an extremely long placeholder to test how it clips or wraps within the input field"
            />
          ),
        },
      ]}
    />
  ),
}

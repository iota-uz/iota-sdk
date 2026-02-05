import type { Meta, StoryObj } from '@storybook/react'

import { TableExportButton } from './TableExportButton'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'

const meta: Meta<typeof TableExportButton> = {
  title: 'BiChat/Components/TableExportButton',
  component: TableExportButton,
  parameters: {
    layout: 'centered',
  },
}

export default meta
type Story = StoryObj<typeof TableExportButton>

export const Playground: Story = {
  args: {
    onClick: () => alert('Export clicked'),
    label: 'Export to Excel',
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'States',
          content: (
            <div className="flex items-center gap-4">
              <TableExportButton onClick={() => {}} label="Default" />
              <TableExportButton onClick={() => {}} disabled label="Disabled" />
            </div>
          ),
        },
        {
          name: 'Long Label',
          content: <TableExportButton onClick={() => {}} label="Export this huge table to a very long file format name" />,
        },
      ]}
    />
  ),
}

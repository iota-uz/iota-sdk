import type { Meta, StoryObj } from '@storybook/react'

import { TableWithExport } from './TableWithExport'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'

const meta: Meta<typeof TableWithExport> = {
  title: 'BiChat/Components/TableWithExport',
  component: TableWithExport,
}

export default meta
type Story = StoryObj<typeof TableWithExport>

const ExampleTable = () => (
  <>
    <thead>
      <tr>
        <th>Product</th>
        <th>Price</th>
        <th>Stock</th>
      </tr>
    </thead>
    <tbody>
      <tr>
        <td>Widget A</td>
        <td>$10.00</td>
        <td>42</td>
      </tr>
      <tr>
        <td>Gadget B</td>
        <td>$25.50</td>
        <td>12</td>
      </tr>
    </tbody>
  </>
)

export const Playground: Story = {
  args: {
    children: <ExampleTable />,
    sendMessage: (msg) => alert(`Mock send: ${msg}`),
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Wide Table (Scroll)',
          content: (
            <TableWithExport sendMessage={() => {}}>
              <thead>
                <tr>
                  {Array.from({ length: 15 }).map((_, i) => (
                    <th key={i} className="whitespace-nowrap">Column {i + 1}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {Array.from({ length: 5 }).map((_, r) => (
                  <tr key={r}>
                    {Array.from({ length: 15 }).map((_, c) => (
                      <td key={c}>Data {r + 1}-{c + 1}</td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </TableWithExport>
          ),
        },
        {
          name: 'Disabled Export',
          content: (
            <TableWithExport disabled sendMessage={() => {}}>
              <ExampleTable />
            </TableWithExport>
          ),
        },
        {
          name: 'No Export Handler',
          content: (
            <TableWithExport>
              <ExampleTable />
            </TableWithExport>
          ),
        },
      ]}
    />
  ),
}

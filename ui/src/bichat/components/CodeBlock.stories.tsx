import type { Meta, StoryObj } from '@storybook/react'

import { CodeBlock } from './CodeBlock'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'
import { veryLargeText } from '@sb-helpers/textFixtures'

const meta: Meta<typeof CodeBlock> = {
  title: 'BiChat/Components/CodeBlock',
  component: CodeBlock,
}

export default meta
type Story = StoryObj<typeof CodeBlock>

const pyCode = `def calculate_revenue(orders):
    total = sum(order.price * order.quantity for order in orders)
    tax = total * 0.08
    return total + tax

# Example usage:
print(calculate_revenue([Order(price=10, quantity=2)]))`

export const Playground: Story = {
  args: {
    language: 'python',
    value: pyCode,
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Large File (Scroll)',
          content: (
            <div className="h-[400px] overflow-hidden border rounded">
              <CodeBlock language="typescript" value={veryLargeText.slice(0, 10000)} />
            </div>
          ),
        },
        {
          name: 'Inline Code',
          content: (
            <p>
              You can use <CodeBlock inline language="ts" value="const x = 42" /> in sentences.
            </p>
          ),
        },
        {
          name: 'Unknown Language',
          content: <CodeBlock language="foobar" value="Some weird format content" />,
        },
      ]}
    />
  ),
}

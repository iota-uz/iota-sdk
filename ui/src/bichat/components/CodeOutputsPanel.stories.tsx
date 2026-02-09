import type { Meta, StoryObj } from '@storybook/react'

import { CodeOutputsPanel } from './CodeOutputsPanel'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'
import { makeCodeOutputs } from '@sb-helpers/bichatFixtures'
import { largeImageDataUrl } from '@sb-helpers/imageFixtures'
import { veryLargeText } from '@sb-helpers/textFixtures'

const meta: Meta<typeof CodeOutputsPanel> = {
  title: 'BiChat/Components/CodeOutputsPanel',
  component: CodeOutputsPanel,
}

export default meta
type Story = StoryObj<typeof CodeOutputsPanel>

export const Playground: Story = {
  args: {
    outputs: makeCodeOutputs(),
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Large Image Output',
          content: (
            <CodeOutputsPanel
              outputs={[
                {
                  type: 'image',
                  content: largeImageDataUrl,
                  filename: 'high-res-render.png',
                  sizeBytes: 1024 * 1024 * 5,
                },
              ]}
            />
          ),
        },
        {
          name: 'Large Text Output (Scroll)',
          content: (
            <CodeOutputsPanel
              outputs={[
                {
                  type: 'text',
                  content: veryLargeText.slice(0, 5000),
                },
              ]}
            />
          ),
        },
        {
          name: 'Mixed Outputs (Text, Image, Error)',
          content: (
            <CodeOutputsPanel
              outputs={[
                { type: 'text', content: 'Processing step 1...' },
                { type: 'image', content: 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg==', filename: 'dot.png' },
                { type: 'error', content: 'Traceback (most recent call last):\n  File "script.py", line 42, in <module>\n    RuntimeError: Something went wrong.' },
              ]}
            />
          ),
        },
      ]}
    />
  ),
}

import type { Meta, StoryObj } from '@storybook/react'

import { WelcomeContent } from './WelcomeContent'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'

const meta: Meta<typeof WelcomeContent> = {
  title: 'BiChat/Components/WelcomeContent',
  component: WelcomeContent,
}

export default meta
type Story = StoryObj<typeof WelcomeContent>

export const Playground: Story = {
  args: {
    title: 'Welcome to BiChat',
    description: 'Your intelligent business analytics assistant. Ask questions about your data, generate reports, or explore insights.',
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Long Title & Description',
          content: (
            <WelcomeContent
              title="This is an intentionally extremely long title to see how it wraps on different screen sizes and if it breaks the layout of the welcome screen."
              description="This description is also very long. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat."
            />
          ),
        },
        {
          name: 'Disabled State',
          content: (
            <WelcomeContent
              disabled
              title="Welcome (Disabled)"
              description="Clicking prompts should do nothing."
            />
          ),
        },
        {
          name: 'No Description',
          content: <WelcomeContent description="" />,
        },
      ]}
    />
  ),
}

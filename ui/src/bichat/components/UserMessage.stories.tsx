import type { Meta, StoryObj } from '@storybook/react'

import { UserMessage } from './UserMessage'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'
import { makeUserTurn, makeAttachment } from '@sb-helpers/bichatFixtures'
import { largeText } from '@sb-helpers/textFixtures'

const meta: Meta<typeof UserMessage> = {
  title: 'BiChat/Components/UserMessage',
  component: UserMessage,
}

export default meta
type Story = StoryObj<typeof UserMessage>

export const Playground: Story = {
  args: {
    turn: makeUserTurn({
      content: 'Hello, I need some help with my dashboard.',
    }),
    initials: 'UK',
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Long Content',
          content: (
            <UserMessage
              turn={makeUserTurn({
                content: largeText,
              })}
            />
          ),
        },
        {
          name: 'With Image Attachments',
          content: (
            <UserMessage
              turn={makeUserTurn({
                content: 'Look at this screenshot.',
                attachments: [
                  makeAttachment({ filename: 'sc1.png', mimeType: 'image/png' }),
                  makeAttachment({ filename: 'sc2.jpg', mimeType: 'image/jpeg' }),
                ],
              })}
            />
          ),
        },
        {
          name: 'Hide Metadata',
          content: (
            <UserMessage
              hideAvatar
              hideActions
              hideTimestamp
              turn={makeUserTurn({
                content: 'Clean bubble.',
              })}
            />
          ),
        },
      ]}
    />
  ),
}

import type { Story } from '@ladle/react'
import fixture from '../fixtures/small.json'
import type { LensDocument } from './document'
import { PlaceholderPanel } from './PlaceholderPanel'
import './styles.css'

export const PlaceholderStat: Story = () => (
  <div className="lens-root">
    <PlaceholderPanel document={fixture as LensDocument} locale="en" />
  </div>
)

PlaceholderStat.storyName = 'Placeholder stat panel'

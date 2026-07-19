import type { Story } from '@ladle/react'
import fixture from '../fixtures/small.json'
import { parseDocument } from './contract'
import { PlaceholderPanel } from './PlaceholderPanel'
import './styles.css'

const document = parseDocument(fixture)

export const PlaceholderStat: Story = () => (
  <div className="lens-root">
    <PlaceholderPanel document={document} locale="en" />
  </div>
)

PlaceholderStat.storyName = 'Placeholder stat panel'

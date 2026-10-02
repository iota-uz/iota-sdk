import { describe, expect, it, vi } from 'vitest'
import { fireEvent, screen } from '@solidjs/testing-library'
import { SessionList } from './SessionList'
import { renderWithI18n } from '../../ui/test-i18n'
import type { ChatSession, SessionGroup } from '../../utils/sessionGrouping'

function makeSession(id: string, title: string): ChatSession {
  const now = new Date().toISOString()
  return { id, title, createdAt: now, updatedAt: now, pinned: false }
}

const groups: SessionGroup[] = [
  {
    name: 'Today',
    sessions: [makeSession('s1', 'Session A'), makeSession('s2', 'Session B')],
  },
]

function renderList(overrides: {
  onSelect?: (id: string) => void
  translations?: Record<string, string>
} = {}) {
  const onSelect = overrides.onSelect ?? vi.fn()
  const utils = renderWithI18n(
    () => (
      <SessionList
        groups={() => groups}
        onSelect={onSelect}
        onPin={vi.fn()}
        onRename={vi.fn()}
        onArchive={vi.fn()}
      />
    ),
    overrides.translations,
  )
  return { onSelect, ...utils }
}

describe('SessionList', () => {
  it('renders translated group headers and session items', () => {
    renderList({ translations: { 'sidebar.today': 'Bugun' } })

    expect(screen.getByText('Bugun')).toBeInTheDocument()
    expect(screen.getByText('Session A')).toBeInTheDocument()
    expect(screen.getByText('Session B')).toBeInTheDocument()
  })

  it('clicking an item calls onSelect with its id', () => {
    const onSelect = vi.fn()
    const { container } = renderList({ onSelect })

    const row = container.querySelector('[data-session-id="s1"]')
    expect(row).not.toBeNull()
    fireEvent.click(row as Element)

    expect(onSelect).toHaveBeenCalledTimes(1)
    expect(onSelect).toHaveBeenCalledWith('s1')
  })
})

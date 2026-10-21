import { describe, expect, it, vi } from 'vitest'
import { fireEvent, screen } from '@solidjs/testing-library'
import { EditableText } from './primitives'
import { renderWithI18n } from './test-i18n'

describe('EditableText', () => {
  it('commits on Enter with the trimmed value', () => {
    const onCommit = vi.fn()
    const onCancel = vi.fn()
    renderWithI18n(() => (
      <EditableText value="Draft" onCommit={onCommit} onCancel={onCancel} ariaLabel="Rename chat" />
    ))

    fireEvent.click(screen.getByRole('button', { name: 'Rename chat' }))
    const input = screen.getByRole('textbox', { name: 'Rename chat' }) as HTMLInputElement
    fireEvent.input(input, { target: { value: '  Weekly report  ' } })
    fireEvent.keyDown(input, { key: 'Enter' })

    expect(onCommit).toHaveBeenCalledTimes(1)
    expect(onCommit).toHaveBeenCalledWith('Weekly report')
    expect(onCancel).not.toHaveBeenCalled()
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
  })

  it('cancels on Escape and keeps the original value', () => {
    const onCommit = vi.fn()
    const onCancel = vi.fn()
    renderWithI18n(() => (
      <EditableText value="Original" onCommit={onCommit} onCancel={onCancel} />
    ))

    fireEvent.click(screen.getByRole('button', { name: 'Original' }))
    const input = screen.getByRole('textbox')
    fireEvent.input(input, { target: { value: 'Changed' } })
    fireEvent.keyDown(input, { key: 'Escape' })

    expect(onCommit).not.toHaveBeenCalled()
    expect(onCancel).toHaveBeenCalledTimes(1)
    expect(screen.getByRole('button', { name: 'Original' })).toBeInTheDocument()
  })

  it('commits on blur', () => {
    const onCommit = vi.fn()
    renderWithI18n(() => <EditableText value="Before" onCommit={onCommit} />)

    fireEvent.click(screen.getByRole('button', { name: 'Before' }))
    const input = screen.getByRole('textbox') as HTMLInputElement
    fireEvent.input(input, { target: { value: ' After ' } })
    fireEvent.blur(input)

    expect(onCommit).toHaveBeenCalledTimes(1)
    expect(onCommit).toHaveBeenCalledWith('After')
  })
})

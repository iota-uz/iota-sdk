import '@testing-library/jest-dom/vitest'
import { describe, expect, it, vi } from 'vitest'
import { fireEvent, screen } from '@solidjs/testing-library'
import { MessageInput } from './MessageInput'
import { renderWithI18n } from './test-i18n'

describe('MessageInput', () => {
  it('renders disabled state', () => {
    renderWithI18n(() => <MessageInput disabled onSend={() => {}} onStop={() => {}} />)
    expect(screen.getByRole('textbox')).toBeDisabled()
  })

  it('sends text on Enter and clears the field', () => {
    const onSend = vi.fn()
    renderWithI18n(() => <MessageInput onSend={onSend} onStop={() => {}} />)
    const textarea = screen.getByRole('textbox') as HTMLTextAreaElement
    fireEvent.input(textarea, { target: { value: 'hello world' } })
    fireEvent.keyDown(textarea, { key: 'Enter' })
    expect(onSend).toHaveBeenCalledWith('hello world')
    expect(textarea.value).toBe('')
  })

  it('does not send on Shift+Enter or when empty', () => {
    const onSend = vi.fn()
    renderWithI18n(() => <MessageInput onSend={onSend} onStop={() => {}} />)
    const textarea = screen.getByRole('textbox')
    fireEvent.keyDown(textarea, { key: 'Enter', shiftKey: true })
    fireEvent.keyDown(textarea, { key: 'Enter' })
    expect(onSend).not.toHaveBeenCalled()
  })

  it('sends on form submit', () => {
    const onSend = vi.fn()
    renderWithI18n(() => <MessageInput onSend={onSend} onStop={() => {}} />)
    const textarea = screen.getByRole('textbox') as HTMLTextAreaElement
    fireEvent.input(textarea, { target: { value: 'via form' } })
    fireEvent.submit(textarea.closest('form') as HTMLFormElement)
    expect(onSend).toHaveBeenCalledWith('via form')
  })

  it('shows stop button while streaming and calls onStop', () => {
    const onStop = vi.fn()
    const onSend = vi.fn()
    renderWithI18n(() => (
      <MessageInput streaming onSend={onSend} onStop={onStop} />
    ))
    const stop = screen.getByRole('button', { name: 'Stop generating' })
    expect(screen.queryByRole('button', { name: 'Send' })).toBeNull()
    fireEvent.click(stop)
    expect(onStop).toHaveBeenCalledTimes(1)
    expect(onSend).not.toHaveBeenCalled()
  })

  it('disables send while streaming', () => {
    const onSend = vi.fn()
    renderWithI18n(() => <MessageInput streaming onSend={onSend} onStop={() => {}} />)
    const textarea = screen.getByRole('textbox') as HTMLTextAreaElement
    fireEvent.input(textarea, { target: { value: 'text' } })
    expect(onSend).not.toHaveBeenCalled()
  })
})

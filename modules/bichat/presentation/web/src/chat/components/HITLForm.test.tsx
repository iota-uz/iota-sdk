import { describe, expect, it, vi } from 'vitest'
import { fireEvent, screen, waitFor } from '@solidjs/testing-library'
import type { PendingQuestion } from '../types'
import { HITLForm } from './HITLForm'
import { renderWithI18n } from './test-i18n'

function textQuestion(): PendingQuestion {
  return {
    checkpointId: 'cp-1',
    turnId: 'turn-1',
    status: 'awaiting_input',
    questions: [{ id: 'q1', text: 'Which year?', type: 'text', options: [] }],
  }
}

describe('HITLForm', () => {
  it('blocks submit for empty required answer and shows validation message', async () => {
    const onSubmit = vi.fn()
    renderWithI18n(() => (
      <HITLForm question={textQuestion()} onSubmit={onSubmit} onReject={() => {}} />
    ))
    fireEvent.click(screen.getByRole('button', { name: 'Submit' }))
    expect(await screen.findByText('This answer is required')).toBeTruthy()
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('submits answers map after filling required fields', () => {
    const onSubmit = vi.fn()
    renderWithI18n(() => (
      <HITLForm question={textQuestion()} onSubmit={onSubmit} onReject={() => {}} />
    ))
    fireEvent.input(screen.getByLabelText('Which year?'), { target: { value: '2026' } })
    fireEvent.click(screen.getByRole('button', { name: 'Submit' }))
    expect(onSubmit).toHaveBeenCalledWith({ q1: '2026' })
  })

  it('renders select options', () => {
    const question: PendingQuestion = {
      checkpointId: 'cp-2',
      turnId: 'turn-2',
      status: 'awaiting_input',
      questions: [
        {
          id: 'region',
          text: 'Pick a region',
          type: 'select',
          options: [
            { id: 'tash', label: 'Tashkent' },
            { id: 'sam', label: 'Samarkand' },
          ],
        },
      ],
    }
    renderWithI18n(() => <HITLForm question={question} onSubmit={() => {}} onReject={() => {}} />)
    const select = screen.getByLabelText('Pick a region') as HTMLSelectElement
    const labels = Array.from(select.options).map((option) => option.textContent)
    expect(labels).toEqual(['', 'Tashkent', 'Samarkand'])
  })

  it('calls onReject when reject clicked', () => {
    const onReject = vi.fn()
    renderWithI18n(() => (
      <HITLForm question={textQuestion()} onSubmit={() => {}} onReject={onReject} />
    ))
    fireEvent.click(screen.getByRole('button', { name: 'Reject' }))
    expect(onReject).toHaveBeenCalledTimes(1)
  })

  it('submits selected option id', async () => {
    const onSubmit = vi.fn()
    const question: PendingQuestion = {
      checkpointId: 'cp-3',
      turnId: 'turn-3',
      status: 'awaiting_input',
      questions: [
        {
          id: 'metric',
          text: 'Choose metric',
          type: 'select',
          options: [{ id: 'revenue', label: 'Revenue' }],
        },
      ],
    }
    renderWithI18n(() => <HITLForm question={question} onSubmit={onSubmit} onReject={() => {}} />)
    fireEvent.change(screen.getByLabelText('Choose metric'), { target: { value: 'revenue' } })
    fireEvent.click(screen.getByRole('button', { name: 'Submit' }))
    await waitFor(() => expect(onSubmit).toHaveBeenCalledWith({ metric: 'revenue' }))
  })
})

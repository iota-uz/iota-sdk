import { fireEvent, render, screen } from '@solidjs/testing-library'
import { createSignal } from 'solid-js'
import { describe, expect, it, vi } from 'vitest'
import { ScheduleEditor } from './ScheduleEditor'

const now = () => new Date('2026-11-21T10:30:00Z')

describe('ScheduleEditor', () => {
  it('emits the cron of a chosen preset, keeping the time', () => {
    const onChange = vi.fn()
    render(() => <ScheduleEditor params={{ cron: '0 20 * * *', timezone: 'UTC' }} onChange={onChange} readOnly={false} locale="en" now={now} />)
    fireEvent.click(screen.getByRole('radio', { name: 'Weekdays' }))
    expect(onChange).toHaveBeenLastCalledWith({ cron: '0 20 * * 1-5', timezone: 'UTC' })
    fireEvent.click(screen.getByRole('radio', { name: 'Weekly' }))
    expect(onChange).toHaveBeenLastCalledWith({ cron: '0 20 * * 1', timezone: 'UTC' })
  })

  it('shows an alert for an invalid custom cron', () => {
    const [params, setParams] = createSignal<Record<string, unknown>>({ cron: '*/5 * * * *' })
    render(() => <ScheduleEditor params={params()} onChange={setParams} readOnly={false} locale="ru" now={now} />)
    const input = screen.getByLabelText('Выражение cron') as HTMLInputElement
    expect(screen.queryByRole('alert')).toBeNull()
    fireEvent.input(input, { target: { value: '61 * * * *' } })
    expect(params().cron).toBe('61 * * * *')
    expect(screen.getByRole('alert').textContent).toContain('61')
    expect(screen.getByRole('radio', { name: 'Свой cron' }).getAttribute('aria-checked')).toBe('true')
  })

  it('previews the next five runs in the zone', () => {
    render(() => <ScheduleEditor params={{ cron: '0 9 * * *', timezone: 'Asia/Tashkent' }} onChange={() => {}} readOnly={false} locale="en" now={now} />)
    const items = [...document.querySelectorAll('.ae-sched-runs > li')]
    expect(items).toHaveLength(5)
    expect(items[0]!.querySelector('time')!.getAttribute('datetime')).toBe('2026-11-22T04:00:00.000Z')
  })

  it('validates missed_after and drops it when empty', () => {
    const onChange = vi.fn()
    render(() => <ScheduleEditor params={{ cron: '0 9 * * *', missed_after: '2h' }} onChange={onChange} readOnly={false} locale="en" now={now} />)
    const input = screen.getByLabelText('Skip a missed run after')
    fireEvent.input(input, { target: { value: '3x' } })
    expect(screen.getByRole('alert')).toBeTruthy()
    fireEvent.input(input, { target: { value: '' } })
    expect(onChange).toHaveBeenLastCalledWith({ cron: '0 9 * * *' })
  })

  it('disables controls when read-only', () => {
    render(() => <ScheduleEditor params={{ cron: '0 9 * * *' }} onChange={() => {}} readOnly locale="en" now={now} />)
    expect((screen.getByRole('radio', { name: 'Every day' }) as HTMLButtonElement).disabled).toBe(true)
  })
})

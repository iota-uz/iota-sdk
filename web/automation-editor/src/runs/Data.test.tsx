import { fireEvent, render } from '@solidjs/testing-library'
import { describe, expect, it } from 'vitest'
import { Value } from './Data'

describe('Value', () => {
  it('shows the first 200 entries of a long list and all of them on «Show all»', () => {
    const list = Array.from({ length: 250 }, (_, i) => `item-${i}`)
    const { container, getByRole } = render(() => <Value value={{ list }} locale="en" label="Output" />)
    const rows = () => container.querySelectorAll('.ae-json__children .ae-json__children .ae-json__row').length
    expect(rows()).toBe(200)
    fireEvent.click(getByRole('button', { name: 'Show all 250' }))
    expect(rows()).toBe(250)
  })

  it('«Load in full» shows every entry, nothing left behind a cap', () => {
    const list = Array.from({ length: 3000 }, (_, i) => 'x'.repeat(100) + i)
    const { container, getByRole } = render(() => <Value value={{ list }} locale="en" label="Output" />)
    fireEvent.click(getByRole('button', { name: 'Load in full' }))
    expect(container.querySelectorAll('.ae-json__children .ae-json__children .ae-json__row').length).toBe(3000)
    expect(container.querySelector('.ae-json__more')).toBeNull()
  })
})

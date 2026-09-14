import { afterEach, describe, expect, it } from 'vitest'
import { render } from 'solid-js/web'
import { formatPhoneDisplay, PhoneLink, PhoneText } from './Phone'

let dispose: (() => void) | undefined
const host = document.createElement('div')

afterEach(() => {
  dispose?.()
  dispose = undefined
  host.replaceChildren()
})

describe('phone display', () => {
  it('matches the canonical country formats', () => {
    expect(formatPhoneDisplay('+998993303030')).toBe('+998(99)330-30-30')
    expect(formatPhoneDisplay('+998993303030', 'dashes')).toBe('+998-99-330-30-30')
    expect(formatPhoneDisplay('+14155551234', 'spaces')).toBe('+1 415 555 1234')
    expect(formatPhoneDisplay('invalid')).toBe('invalid')
  })

  it('renders canonical link DOM, stripped href, attributes, and icon', () => {
    dispose = render(() => <PhoneLink phone="+998 (99) 330-30-30" showIcon class="font-medium" data-phone="primary" />, host)
    const link = host.querySelector('a')!
    expect(link.getAttribute('href')).toBe('tel:998993303030')
    expect(link.className).toBe('inline-flex items-center gap-1 text-blue-600 hover:text-blue-800 hover:underline font-medium')
    expect(link.textContent).toContain('+998(99)330-30-30')
    expect(link.querySelector('svg')).not.toBeNull()
    expect(link.getAttribute('data-phone')).toBe('primary')
  })

  it('renders text without an icon and renders nothing for an empty phone', () => {
    dispose = render(() => <><PhoneText phone="+14155551234" /><PhoneLink phone="" /></>, host)
    expect(host.querySelector('span')!.textContent).toBe('+1(415)555-1234')
    expect(host.querySelector('svg')).toBeNull()
    expect(host.querySelector('a')).toBeNull()
  })
})

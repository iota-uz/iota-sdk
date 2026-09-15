import { afterEach, describe, expect, it, vi } from 'vitest'
import { render } from 'solid-js/web'
import { AccentColor, EmptyTable, HandLoader } from './Illustrations'

let dispose: (() => void) | undefined
let host: HTMLDivElement
function mount(view: () => unknown) { host = document.createElement('div'); document.body.append(host); dispose = render(view as never, host); return host }
afterEach(() => { dispose?.(); dispose = undefined; document.body.replaceChildren() })

describe('visual primitives', () => {
  it('renders the canonical empty-table SVG dimensions and viewbox', () => {
    mount(() => <EmptyTable width={120} height={151} aria-label="No rows" />)
    const svg = host.querySelector('svg')!
    expect(svg).toHaveAttribute('width', '120')
    expect(svg).toHaveAttribute('height', '151')
    expect(svg).toHaveAttribute('viewBox', '0 0 100 126')
  })

  it('supports controlled accent colors and preserves the CSS variable contract', () => {
    const changed = vi.fn()
    mount(() => <AccentColor name="accent" value="violet" color="#695EFF" checked onCheckedChange={changed} />)
    const label = host.querySelector('label')!
    const input = host.querySelector('input')!
    expect(label.className).toContain('accent-color-card')
    expect(label.style.getPropertyValue('--color')).toBe('#695EFF')
    expect(input.checked).toBe(true)
    input.dispatchEvent(new Event('change', { bubbles: true }))
    expect(changed).toHaveBeenCalledWith(true, 'violet')
  })

  it('renders the hand loader structure with accessible status text', () => {
    mount(() => <HandLoader label="Загрузка" skinColor="#fff" />)
    expect(host.querySelector('[role="status"]')).toHaveAttribute('aria-label', 'Загрузка')
    expect(host.querySelectorAll('.👉')).toHaveLength(4)
    expect(host.querySelector('.🌴')).not.toBeNull()
  })

  it('preserves caller styles while adding visual CSS variables', () => {
    mount(() => <><AccentColor name="accent" value="blue" color="#00f" style={{ margin: '3px' }} /><HandLoader style="opacity:0.5" /></>)
    const label = host.querySelector('label')!
    const hand = host.querySelector<HTMLElement>('.🤚')!
    expect(label.style.margin).toBe('3px')
    expect(label.style.getPropertyValue('--color')).toBe('#00f')
    expect(hand.style.opacity).toBe('0.5')
    expect(hand.style.getPropertyValue('--tap-speed')).toBe('0.6s')
  })
})

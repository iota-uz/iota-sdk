import { afterEach, describe, expect, it } from 'vitest'
import { render } from 'solid-js/web'
import { Alert } from './Alert'
import { Badge } from './Badge'
import { Card, CardHeader } from './Card'
import { Progress } from './Progress'
import { Skeleton } from './Skeleton'
import { Spinner } from './Spinner'

let dispose: (() => void) | undefined
let host: HTMLDivElement

function mount(view: () => unknown) {
  host = document.createElement('div')
  document.body.append(host)
  dispose = render(view as never, host)
  return host
}

afterEach(() => {
  dispose?.()
  dispose = undefined
  document.body.replaceChildren()
})

describe('display primitives', () => {
  it('matches Card wrapper, header and content structure', () => {
    mount(() => <Card header={<CardHeader>Account</CardHeader>} contentClass="dense" data-card="account">Body</Card>)
    const card = host.firstElementChild as HTMLElement
    expect(card.className).toBe('bg-surface-300 rounded-lg border border-subtle')
    expect(card.dataset.card).toBe('account')
    expect(card.firstElementChild!.className).toBe('border-b border-subtle p-4')
    expect(card.lastElementChild!.className).toBe('p-4 dense')
  })

  it('renders canonical alert tones and native attributes', () => {
    mount(() => <Alert variant="success" aria-live="polite">Saved</Alert>)
    const alert = host.firstElementChild as HTMLElement
    expect(alert.className).toBe('bg-green-100 text-green-600 py-2.5 px-4 text-sm font-medium rounded-md w-full text-center')
    expect(alert.getAttribute('aria-live')).toBe('polite')
    expect(alert.getAttribute('role')).toBe('alert')
  })

  it('covers every Badge tone and both sizes', () => {
    const tones = ['pink', 'yellow', 'green', 'blue', 'purple', 'gray'] as const
    mount(() => <>{tones.map((variant, index) => <Badge variant={variant} size={index % 2 ? 'lg' : 'normal'}>{variant}</Badge>)}</>)
    const badges = [...host.children]
    expect(badges).toHaveLength(6)
    expect(badges[0]!.classList.contains('border-pink')).toBe(true)
    expect(badges[1]!.classList.contains('h-9')).toBe(true)
    expect(badges[5]!.classList.contains('bg-badge-gray')).toBe(true)
  })

  it('clamps progress geometry and exposes progress semantics', () => {
    mount(() => <Progress value={120} target={100} valueLabel="Done" targetLabel="100 jobs" />)
    const bar = host.querySelector('[role="progressbar"]') as HTMLElement
    expect(bar.style.width).toBe('100%')
    expect(bar.getAttribute('aria-valuenow')).toBe('120')
    expect(bar.textContent).toBe('Done')
    expect(host.querySelector('.shrink-0')!.textContent).toBe('100 jobs')
  })

  it('matches Spinner SVG and accessible loading label', () => {
    mount(() => <Spinner spinnerClass="small" label="Saving" />)
    expect(host.firstElementChild!.getAttribute('role')).toBe('status')
    expect(host.querySelector('svg')!.classList.contains('fill-brand-600')).toBe(true)
    expect(host.querySelector('.sr-only')!.textContent).toBe('Saving')
  })

  it('renders canonical skeleton line defaults and variants', () => {
    mount(() => <Skeleton />)
    expect(host.firstElementChild!.children).toHaveLength(3)
    dispose?.()
    host.replaceChildren()
    dispose = render(() => <Skeleton variant="table" lines={2} />, host)
    expect(host.firstElementChild!.children).toHaveLength(2)
    expect(host.firstElementChild!.getAttribute('aria-hidden')).toBe('true')
  })
})


import { afterEach, describe, expect, it, vi } from 'vitest'
import { render } from 'solid-js/web'
import { CopyableText, CopyButton } from './CopyButton'
import { buildExportURL, collectExportParams, ExportDropdown } from './ExportDropdown'
import { HelpContext, HelpHint, HelpLink, helpDocURL } from './Help'
import { LanguageSelect, supportedLanguages } from './LanguageSelect'
import { createSlotManager, Slot, SlotManagerProvider, Streamer } from './Slot'
import { Spotlight, SpotlightItemsCollapsible, spotlightBadgeClass } from './Spotlight'

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
  vi.useRealTimers()
})

describe('copy utilities', () => {
  it('copies through an injected adapter and resets the success state', async () => {
    vi.useFakeTimers()
    const copy = vi.fn(async () => undefined)
    mount(() => <CopyButton text={'quote"\nline'} showText copy={copy} resetMs={20} />)
    host.querySelector('button')!.click()
    await vi.runAllTicks()
    expect(copy).toHaveBeenCalledWith('quote"\nline')
    expect(host.querySelector('button')).toHaveTextContent('Copied!')
    await vi.advanceTimersByTimeAsync(20)
    expect(host.querySelector('button')).toHaveTextContent('Copy')
  })

  it('supports keyboard activation and the canonical empty placeholder', async () => {
    const copy = vi.fn(async () => undefined)
    mount(() => <><CopyableText text="ABC" label="Copy policy" copy={copy} /><CopyableText text="" /></>)
    const target = host.querySelector<HTMLElement>('[role="button"]')!
    target.dispatchEvent(new KeyboardEvent('keydown', { key: ' ', bubbles: true }))
    await Promise.resolve()
    expect(copy).toHaveBeenCalledWith('ABC')
    expect(host.textContent).toContain('-')
  })
})

describe('ExportDropdown', () => {
  it('merges form filters, removes pagination and builds a stable format URL', () => {
    const form = document.createElement('form')
    form.id = 'filters'
    form.innerHTML = '<input name="status" value="active"><input name="tag" value="a"><input name="tag" value="b">'
    document.body.append(form)
    const params = collectExportParams('filters', '?status=old&page=2&sort=name')
    expect(params.getAll('tag')).toEqual(['a', 'b'])
    expect(params.has('page')).toBe(false)
    expect(buildExportURL('/export?scope=all', 'csv', params)).toContain('&format=csv')
  })

  it('uses an injected callback, exposes busy state and closes the menu', async () => {
    let resolve!: () => void
    const onExport = vi.fn(() => new Promise<void>((done) => { resolve = done }))
    mount(() => <ExportDropdown formats={['excel', 'csv']} defaultOpen onExport={onExport} labels={{ preparing: 'Готовим' }} />)
    const buttons = host.querySelectorAll<HTMLButtonElement>('li button')
    buttons[0]!.click()
    expect(onExport).toHaveBeenCalledWith(expect.objectContaining({ format: 'excel' }))
    expect(host.querySelector('[role="status"]')).toHaveTextContent('Готовим')
    resolve()
    await Promise.resolve()
    await Promise.resolve()
    expect(host.querySelector('[role="status"]')).toBeNull()
  })
})

describe('language and slots', () => {
  it('renders the canonical languages and forwards native selection', () => {
    const changed = vi.fn()
    mount(() => <LanguageSelect defaultValue="uz-Cyrl" onValueChange={changed} />)
    const select = host.querySelector('select')!
    expect(select.options).toHaveLength(supportedLanguages.length)
    expect(select.value).toBe('uz-Cyrl')
    select.value = 'ru'
    select.dispatchEvent(new Event('change', { bubbles: true }))
    expect(changed).toHaveBeenCalledWith('ru')
  })

  it('replaces slot fallback with streamed chunks', async () => {
    const manager = createSlotManager()
    async function* chunks() { yield { name: 'summary', chunk: <strong>Ready</strong> } }
    mount(() => <SlotManagerProvider manager={manager}><Slot name="summary" fallback={<span>Waiting</span>} /><Streamer stream={chunks()} /></SlotManagerProvider>)
    expect(host).toHaveTextContent('Waiting')
    await Promise.resolve()
    await Promise.resolve()
    expect(host.querySelector('#summary-target')).toHaveTextContent('Ready')
    expect(host.querySelector('template')?.getAttribute('shadowrootmode')).toBe('open')
  })
})

describe('help utilities', () => {
  it('resolves internal, doc and external links', () => {
    expect(helpDocURL('', 'billing/export')).toBe('/help/doc/billing/export')
    expect(helpDocURL('/docs/', 'doc/start')).toBe('/docs/doc/start')
    expect(helpDocURL('/help', 'https://example.com/help')).toBe('https://example.com/help')
    mount(() => <HelpLink path="billing/export" newTab label="Справка" />)
    const link = host.querySelector('a')!
    expect(link.getAttribute('href')).toBe('/help/doc/billing/export')
    expect(link.rel).toBe('noopener noreferrer')
  })

  it('opens contextual help and restores focus after Escape', async () => {
    mount(() => <HelpContext title="Export" summary="How it works" sections={[{ title: 'Steps', items: ['Choose format'] }]} />)
    const button = host.querySelector('button')!
    button.click()
    expect(host.querySelector('[role="dialog"]')).toHaveTextContent('Choose format')
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await Promise.resolve()
    expect(host.querySelector('[role="dialog"]')).toBeNull()
    expect(document.activeElement).toBe(button)
  })

  it('supports a controlled hint state', () => {
    const changed = vi.fn()
    mount(() => <HelpHint title="Hint" description="Details" open onOpenChange={changed} />)
    expect(host.querySelector('[role="dialog"]')).toHaveTextContent('Details')
    host.querySelector('button')!.click()
    expect(changed).toHaveBeenCalledWith(false)
    expect(host.querySelector('[role="dialog"]')).not.toBeNull()
  })

  it('opens a hint on hover and closes it after the pointer leaves', () => {
    mount(() => <HelpHint title="Hint" description="Details" />)
    const hint = host.firstElementChild!
    hint.dispatchEvent(new MouseEvent('mouseenter'))
    expect(host.querySelector('[role="dialog"]')).toHaveTextContent('Details')
    hint.dispatchEvent(new MouseEvent('mouseleave'))
    expect(host.querySelector('[role="dialog"]')).toBeNull()
  })
})

describe('Spotlight', () => {
  it('opens from the global shortcut, searches, navigates by keyboard and activates a result', async () => {
    vi.useFakeTimers()
    const navigate = vi.fn()
    const search = vi.fn(async () => [
      { key: 'one', title: 'One', link: '/one', group: 'Pages' },
      { key: 'two', title: 'Two', link: '/two', badges: [{ label: 'Exact', tone: 'exact' }] },
    ])
    mount(() => <Spotlight search={search} onNavigate={navigate} debounceMs={10} />)
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', metaKey: true, bubbles: true }))
    const input = host.querySelector<HTMLInputElement>('[role="dialog"] input')!
    expect(input).not.toBeNull()
    input.value = 'policy'
    input.dispatchEvent(new InputEvent('input', { bubbles: true }))
    await vi.advanceTimersByTimeAsync(10)
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }))
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
    expect(navigate).toHaveBeenCalledWith(expect.objectContaining({ key: 'two' }))
    expect(host.querySelector('[data-spotlight-key="two"]')).not.toBeNull()
  })

  it('switches AI mode with Tab, closes on Escape and preserves helper classes', () => {
    mount(() => <Spotlight defaultOpen enableAI />)
    const input = host.querySelector<HTMLInputElement>('[role="dialog"] input')!
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true }))
    expect(input.placeholder).toContain('AI')
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    expect(input.placeholder).toBe('Search...')
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    expect(host.querySelector('[role="dialog"]')).toBeNull()
    expect(spotlightBadgeClass('exact')).toContain('border-emerald')
  })

  it('expands canonical collapsible results accessibly', () => {
    mount(() => <ul><SpotlightItemsCollapsible exactItems={<li>Exact</li>} moreItems={<li>More</li>} moreLabel="1 more" /></ul>)
    expect(host).not.toHaveTextContent('More')
    host.querySelector('button')!.click()
    expect(host).toHaveTextContent('More')
    expect(host.querySelector('[data-spotlight-more]')).not.toBeNull()
  })

  it('gives the global shortcut to one owner and keeps listbox options free of interactive descendants', async () => {
    mount(() => <><Spotlight results={[{ key: 'first', title: 'First', link: '/first' }]} query="x" /><Spotlight results={[{ key: 'second', title: 'Second', link: '/second' }]} query="x" /></>)
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', metaKey: true, bubbles: true }))
    await Promise.resolve()
    const dialogs = host.querySelectorAll('[role="dialog"]')
    expect(dialogs).toHaveLength(1)
    const option = dialogs[0]!.querySelector('[role="option"]')!
    expect(option.querySelector('a,button,input,select,textarea')).toBeNull()
    expect(option.querySelector('[data-spotlight-key="second"]')).not.toBeNull()
  })
})

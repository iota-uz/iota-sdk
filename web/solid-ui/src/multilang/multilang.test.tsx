import { afterEach, describe, expect, it, vi } from 'vitest'
import { render } from 'solid-js/web'
import { localizedMultiLangValue, MultiLangDetailsCompact, MultiLangDetailsView, MultiLangFormInput, MultiLangTableCell } from './MultiLang'

let dispose: (() => void) | undefined
let host: HTMLDivElement
function mount(view: () => unknown) { host = document.createElement('div'); document.body.append(host); dispose = render(view as never, host); return host }
afterEach(() => { dispose?.(); dispose = undefined; document.body.replaceChildren() })

describe('MultiLangFormInput', () => {
  it('renders all four canonical locale rows and serializes edits', () => {
    const changed = vi.fn()
    mount(() => <MultiLangFormInput name="title" label="Title" defaultValue={{ en: 'Name', ru: 'Имя' }} onValueChange={changed} />)
    expect(host.querySelectorAll('.locale-row')).toHaveLength(4)
    expect([...host.querySelectorAll<HTMLInputElement>('.locale-code')].map((input) => input.value)).toEqual(['en', 'ru', 'uz', 'uz-cyrl'])
    expect(host.querySelector<HTMLInputElement>('.locale-code')!.maxLength).toBeGreaterThanOrEqual('uz-cyrl'.length)
    const value = host.querySelectorAll<HTMLInputElement>('.locale-value')[2]!
    value.value = 'Nomi'
    value.dispatchEvent(new InputEvent('input', { bubbles: true }))
    expect(JSON.parse(host.querySelector<HTMLInputElement>('.multilang-json')!.value)).toEqual({ en: 'Name', ru: 'Имя', uz: 'Nomi' })
    expect(changed).toHaveBeenLastCalledWith(expect.objectContaining({ uz: 'Nomi' }), true)
  })

  it('validates duplicates and confirms removal of a nonempty translation', async () => {
    const confirmRemoval = vi.fn(async () => true)
    mount(() => <MultiLangFormInput name="title" label="Title" defaultValue={{ en: 'Name' }} confirmRemoval={confirmRemoval} />)
    const codes = host.querySelectorAll<HTMLInputElement>('.locale-code')
    codes[1]!.value = 'en'
    codes[1]!.dispatchEvent(new Event('change', { bubbles: true }))
    expect(host.querySelector('[role="alert"]')).toHaveTextContent('Duplicate language code')
    host.querySelector<HTMLButtonElement>('.remove-locale-btn')!.click()
    await Promise.resolve()
    expect(confirmRemoval).toHaveBeenCalledWith('en', 'Name')
    expect(host.querySelectorAll('.locale-row')).toHaveLength(3)
  })

  it('suppresses a late removal confirmation after disposal', async () => {
    let resolve!: (accepted: boolean) => void
    const changed = vi.fn()
    mount(() => <MultiLangFormInput name="title" label="Title" defaultValue={{ en: 'Name' }} confirmRemoval={() => new Promise((done) => { resolve = done })} onValueChange={changed} />)
    host.querySelector<HTMLButtonElement>('.remove-locale-btn')!.click()
    dispose?.()
    dispose = undefined
    resolve(true)
    await Promise.resolve()
    expect(changed).not.toHaveBeenCalled()
  })
})

describe('MultiLang display components', () => {
  it('uses locale-specific fallback priority and canonical display classes', () => {
    const value = { ru: 'Имя', uz: 'Nomi' }
    expect(localizedMultiLangValue(value, 'en')).toBe('Nomi')
    mount(() => <><MultiLangDetailsView value={value} /><MultiLangDetailsCompact value={value} locale="ru" /><MultiLangTableCell value={value} locale="ru" /></>)
    expect(host.querySelector('.multilang-details')).toHaveTextContent('uz:')
    expect(host.querySelector('.multilang-compact')).toHaveTextContent('+1 more')
    expect(host.querySelector('.multilang-cell')).toHaveAttribute('title', 'ru: Имя | uz: Nomi')
  })

  it('renders localized empty states', () => {
    mount(() => <><MultiLangDetailsView value={{}} labels={{ noTranslations: 'Нет переводов' }} /><MultiLangTableCell value={{}} /></>)
    expect(host).toHaveTextContent('Нет переводов')
    expect(host).toHaveTextContent('—')
  })
})

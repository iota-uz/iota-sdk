import { afterEach, describe, expect, it, vi } from 'vitest'
import { render } from 'solid-js/web'
import { DateRangeClearButton } from './DateRangeClearButton'
import { PhoneInput } from './PhoneInput'
import { Radio, RadioGroup } from './Radio'
import { Slider } from './Slider'
import { Switch } from './Switch'
import { Toggle } from './Toggle'
import { acceptsFile, UploadDropzone, UploadList, validateUploadFiles } from './Upload'

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

describe('RadioGroup and Radio', () => {
  it('owns an uncontrolled selection and preserves the canonical card DOM', () => {
    const changed = vi.fn()
    mount(() => (
      <RadioGroup name="plan" label="Plan" defaultValue="basic" orientation="horizontal" onValueChange={changed}>
        <Radio value="basic" label="Basic" />
        <Radio value="pro" label="Pro" />
      </RadioGroup>
    ))
    const inputs = [...host.querySelectorAll<HTMLInputElement>('input[type="radio"]')]
    expect(inputs[0]!.checked).toBe(true)
    expect(inputs[0]!.name).toBe('plan')
    expect(host.querySelector('[role="radiogroup"]')!.classList.contains('grid-flow-col')).toBe(true)
    inputs[1]!.click()
    expect(inputs[1]!.checked).toBe(true)
    expect(changed).toHaveBeenCalledWith('pro')
    expect(inputs[1]!.nextElementSibling!.className).toContain('peer-checked:after:bg-brand-500')
  })

  it('supports standalone uncontrolled and disabled radios', () => {
    mount(() => <Radio value="yes" defaultChecked disabled label="Yes" />)
    const input = host.querySelector('input')!
    expect(input.checked).toBe(true)
    expect(input.disabled).toBe(true)
  })

  it('associates radio-group and switch errors with their controls', () => {
    mount(() => <><RadioGroup label="Plan" error="Choose one"><Radio value="one" /></RadioGroup><Switch label="Enabled" error="Required" /></>)
    const group = host.querySelector('[role="radiogroup"]')!
    expect(group.getAttribute('aria-labelledby')).toBe(host.querySelector('h2')!.id)
    expect(group.getAttribute('aria-describedby')).toBe(host.querySelector('[role="alert"]')!.id)
    const toggle = host.querySelector<HTMLInputElement>('[role="switch"]')!
    expect(toggle.getAttribute('aria-describedby')).toBe(toggle.id + '-error')
  })
})

describe('Switch, Slider and Toggle', () => {
  it('toggles uncontrolled state, forwards events and maps all switch sizes', () => {
    const changed = vi.fn()
    mount(() => <Switch label="Enabled" size="lg" defaultChecked onChange={changed} data-native="yes" />)
    const input = host.querySelector('input')!
    expect(input.getAttribute('role')).toBe('switch')
    expect(input.checked).toBe(true)
    expect(input.dataset.native).toBe('yes')
    expect(input.nextElementSibling!.className).toContain('w-14 h-7')
    input.click()
    expect(input.checked).toBe(false)
    expect(changed).toHaveBeenCalledOnce()
  })

  it('keeps native range keyboard behavior and updates track/value text', () => {
    const changed = vi.fn()
    mount(() => <Slider label="Factor" min={0} max={10} step={0.1} defaultValue={2.5} helpText="Choose a factor" error="Review" onValueChange={changed} />)
    const input = host.querySelector('input[type="range"]') as HTMLInputElement
    expect(input.value).toBe('2.5')
    expect(input.getAttribute('aria-valuetext')).toBe('2.5')
    expect((input.previousElementSibling as HTMLElement).style.width).toBe('25%')
    expect(input.getAttribute('aria-describedby')).toContain('-help')
    expect(input.getAttribute('aria-describedby')).toContain('-error')
    input.value = '7.5'
    input.dispatchEvent(new InputEvent('input', { bubbles: true }))
    expect(changed).toHaveBeenCalledWith(7.5)
    expect((input.previousElementSibling as HTMLElement).style.width).toBe('75%')
  })

  it('updates toggle active classes, hidden form value and disabled options', () => {
    const changed = vi.fn()
    mount(() => <Toggle name="mode" size="sm" rounded="full" alignment="end" defaultValue="one" options={[
      { value: 'one', label: 'One' }, { value: 'two', label: 'Two' }, { value: 'three', label: 'Three', disabled: true },
    ]} onValueChange={changed} />)
    const wrapper = host.firstElementChild!
    expect(wrapper.className).toBe('tab-slider tabs-sm tabs-three-slots tabs-full tabs-end')
    const buttons = [...host.querySelectorAll('button')]
    expect(buttons[0]!.classList.contains('tab-active')).toBe(true)
    buttons[1]!.click()
    expect(buttons[1]!.getAttribute('aria-pressed')).toBe('true')
    expect((host.querySelector('input[type="hidden"]') as HTMLInputElement).value).toBe('two')
    buttons[2]!.click()
    expect(changed).toHaveBeenCalledTimes(1)
  })
})

describe('small advanced controls', () => {
  it('renders PhoneInput as the canonical tel field with mobile hints', () => {
    mount(() => <PhoneInput label="Phone" defaultValue="+998901234567" />)
    const input = host.querySelector('input')!
    expect(input.type).toBe('tel')
    expect(input.inputMode).toBe('tel')
    expect(input.autocomplete).toBe('tel')
    expect(input.className).toBe('form-control-input outline-none w-full')
  })

  it('only shows DateRangeClearButton for a value and dispatches the form event', () => {
    const form = document.createElement('form')
    form.id = 'filters'
    document.body.append(form)
    const listener = vi.fn()
    form.addEventListener('dateRangeChange', listener)
    mount(() => <DateRangeClearButton value="2026-01-01" formId="filters" />)
    const button = host.querySelector('button')!
    button.click()
    expect(listener).toHaveBeenCalledOnce()
    expect(button.dataset.formId).toBe('filters')

    dispose?.()
    host.replaceChildren()
    dispose = render(() => <DateRangeClearButton value="" />, host)
    expect(host.querySelector('button')).toBeNull()
  })
})

describe('upload primitives', () => {
  it('matches extension, exact MIME and wildcard accepts', () => {
    const png = new File(['image'], 'photo.PNG', { type: 'image/png' })
    expect(acceptsFile(png, '.png')).toBe(true)
    expect(acceptsFile(png, 'image/png')).toBe(true)
    expect(acceptsFile(png, 'image/*')).toBe(true)
    expect(acceptsFile(png, 'application/pdf')).toBe(false)
  })

  it('reports deterministic size and type validation copy', () => {
    const large = new File([new Uint8Array(20)], 'large.exe', { type: 'application/octet-stream' })
    const errors = validateUploadFiles([large], 'image/*', 10)
    expect(errors).toEqual(['File "large.exe" is too large (0.0MB > 0.0MB)'])
    expect(validateUploadFiles([large], 'image/*', 0)).toEqual(['File "large.exe" — unsupported format'])
  })

  it('renders uploaded metadata, image preview, hidden values and removal', () => {
    const remove = vi.fn()
    mount(() => <UploadList name="files" form="product" items={[{ id: '42', name: 'scan.png', url: '/scan.png', mimeType: 'image/png', size: '12 KB' }]} onRemove={remove} />)
    const link = host.querySelector('a')!
    expect(link.textContent).toBe('scan.png')
    expect(link.getAttribute('rel')).toBe('noopener noreferrer')
    expect(host.querySelector('img')!.alt).toBe('scan.png')
    const hidden = host.querySelector('input')!
    expect(hidden.value).toBe('42')
    expect(hidden.getAttribute('form')).toBe('product')
    ;(host.querySelector('[aria-label="Remove file"]') as HTMLButtonElement).click()
    expect(remove).toHaveBeenCalledOnce()
  })

  it('rejects invalid picker files before calling the upload transport', async () => {
    const upload = vi.fn()
    mount(() => <UploadDropzone label="Upload" accept="image/*" upload={upload} />)
    const input = host.querySelector('input[type="file"]') as HTMLInputElement
    const invalid = new File(['bad'], 'report.pdf', { type: 'application/pdf' })
    Object.defineProperty(input, 'files', { configurable: true, value: [invalid] })
    input.dispatchEvent(new Event('change', { bubbles: true }))
    await vi.waitFor(() => expect(host.querySelector('[role="alert"]')!.textContent).toContain('unsupported format'))
    expect(upload).not.toHaveBeenCalled()
  })

  it('uploads valid files, emits controlled items and clears pending state', async () => {
    const changed = vi.fn()
    const upload = vi.fn(async (file: File) => ({ id: 'new', name: file.name, mimeType: file.type }))
    mount(() => <UploadDropzone label="Upload" accept="image/*" defaultItems={[]} upload={upload} onItemsChange={changed} name="files" />)
    const input = host.querySelector('input[type="file"]') as HTMLInputElement
    const valid = new File(['ok'], 'photo.png', { type: 'image/png' })
    Object.defineProperty(input, 'files', { configurable: true, value: [valid] })
    input.dispatchEvent(new Event('change', { bubbles: true }))
    await vi.waitFor(() => expect(host.querySelector('[data-upload-id="new"]')).not.toBeNull())
    expect(changed).toHaveBeenCalledWith([{ id: 'new', name: 'photo.png', mimeType: 'image/png' }])
    expect(host.querySelector('.htmx-indicator')!.classList.contains('hidden')).toBe(true)
  })

  it('ignores an obsolete upload even when its adapter resolves after abort', async () => {
    const resolvers: Array<(item: { id: string; name: string }) => void> = []
    const changed = vi.fn()
    mount(() => <UploadDropzone label="Upload" accept="image/*" upload={(file) => new Promise((resolve) => resolvers.push((item) => resolve({ ...item, name: file.name })))} onItemsChange={changed} />)
    const input = host.querySelector('input[type="file"]') as HTMLInputElement
    const choose = (name: string) => {
      Object.defineProperty(input, 'files', { configurable: true, value: [new File(['ok'], name, { type: 'image/png' })] })
      input.dispatchEvent(new Event('change', { bubbles: true }))
    }
    choose('old.png')
    choose('new.png')
    resolvers[1]!({ id: 'new', name: 'new.png' })
    await Promise.resolve()
    resolvers[0]!({ id: 'old', name: 'old.png' })
    await Promise.resolve()
    expect(changed).toHaveBeenCalledTimes(1)
    expect(changed).toHaveBeenCalledWith([expect.objectContaining({ id: 'new' })])
  })
})

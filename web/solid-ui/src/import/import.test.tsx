import { afterEach, describe, expect, it, vi } from 'vitest'
import { render } from 'solid-js/web'
import { ExampleTable, ImportErrors, ImportForm, ImportRunProgress, ImportRunResult, type ImportPageConfig } from './Import'

let dispose: (() => void) | undefined
let host: HTMLDivElement
function mount(view: () => unknown) { host = document.createElement('div'); document.body.append(host); dispose = render(view as never, host); return host }
afterEach(() => { dispose?.(); dispose = undefined; document.body.replaceChildren(); vi.useRealTimers() })
const config: ImportPageConfig = { title: 'Import positions', description: 'Upload the spreadsheet', columns: [{ header: 'Code', required: true }, { header: 'Name' }], exampleRows: [['A1', 'Manager']], acceptedFileTypes: '.xlsx' }

describe('import presentation', () => {
  it('renders canonical error filtering and spreadsheet coordinates', () => {
    mount(() => <><ImportErrors errors={{ FileID: 'Choose file', Name: 'Invalid name' }} /><ExampleTable columns={config.columns!} rows={config.exampleRows!} /></>)
    expect(host.querySelector('[role="alert"]')).toHaveTextContent('Invalid name')
    expect(host.querySelector('[role="alert"]')).not.toHaveTextContent('Choose file')
    expect([...host.querySelectorAll('thead tr:first-child th')].map((cell) => cell.textContent)).toEqual(['', 'A', 'B'])
    expect(host.querySelector('tbody td')).toHaveTextContent('2')
  })

  it('validates the missing file before submission', () => {
    const submit = vi.fn()
    mount(() => <ImportForm config={config} upload={vi.fn()} submit={submit} />)
    host.querySelector('form')!.dispatchEvent(new SubmitEvent('submit', { bubbles: true, cancelable: true }))
    expect(submit).not.toHaveBeenCalled()
    expect(host).toHaveTextContent('Select a file to import')
  })

  it('uploads a file and submits a typed request', async () => {
    const upload = vi.fn(async (file: File) => ({ id: '77', name: file.name }))
    const submit = vi.fn(async () => ({ status: 'queued' as const, done: 0, total: 0 }))
    const started = vi.fn()
    mount(() => <ImportForm config={config} upload={upload} submit={submit} onSubmit={started} />)
    const input = host.querySelector<HTMLInputElement>('input[type="file"]')!
    const file = new File(['sheet'], 'positions.xlsx', { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' })
    Object.defineProperty(input, 'files', { configurable: true, value: [file] })
    input.dispatchEvent(new Event('change', { bubbles: true }))
    await Promise.resolve(); await Promise.resolve()
    host.querySelector('form')!.dispatchEvent(new SubmitEvent('submit', { bubbles: true, cancelable: true }))
    await Promise.resolve(); await Promise.resolve()
    expect(upload).toHaveBeenCalledWith(file, expect.any(AbortSignal))
    expect(submit).toHaveBeenCalledWith(expect.objectContaining({ file: expect.objectContaining({ id: '77' }) }), expect.any(AbortSignal))
    expect(started).toHaveBeenCalledWith(expect.objectContaining({ status: 'queued' }))
  })
})

describe('import run state machine', () => {
  it('polls progress and stops on success', async () => {
    vi.useFakeTimers()
    const loadStatus = vi.fn(async () => ({ status: 'done' as const, done: 4, total: 4, result: { counts: [{ label: 'Imported', value: 4 }] } }))
    mount(() => <ImportRunProgress initialState={{ status: 'running', phase: 'Reading', done: 1, total: 4 }} loadStatus={loadStatus} pollInterval={20} />)
    expect(host.querySelector('[role="progressbar"]')).toHaveAttribute('aria-valuenow', '1')
    await vi.advanceTimersByTimeAsync(20)
    expect(loadStatus).toHaveBeenCalledOnce()
    expect(host).toHaveTextContent('Imported')
    await vi.advanceTimersByTimeAsync(40)
    expect(loadStatus).toHaveBeenCalledOnce()
  })

  it('cancels an active run through the injected adapter', async () => {
    const cancel = vi.fn(async () => ({ status: 'cancelled' as const, done: 2, total: 10 }))
    mount(() => <ImportRunProgress initialState={{ status: 'running', done: 2, total: 10 }} cancel={cancel} />)
    host.querySelector('button')!.click()
    await Promise.resolve(); await Promise.resolve()
    expect(cancel).toHaveBeenCalledWith(expect.objectContaining({ status: 'running' }), expect.any(AbortSignal))
    expect(host).toHaveTextContent('Import cancelled')
  })

  it('confirms dry runs and offers retry after failure', async () => {
    const confirm = vi.fn(async () => ({ status: 'done' as const, done: 3, total: 3, result: { dryRun: false } }))
    const retry = vi.fn()
    const update = vi.fn()
    mount(() => <><ImportRunResult state={{ status: 'done', done: 3, total: 3, result: { dryRun: true, warnings: ['Review row 2'] } }} confirm={confirm} onStateChange={update} /><ImportRunResult state={{ status: 'failed', done: 0, total: 1, error: 'Bad file' }} retry={retry} /></>)
    expect(host).toHaveTextContent('This is a preview')
    const buttons = host.querySelectorAll<HTMLButtonElement>('button')
    buttons[0]!.click(); await Promise.resolve(); await Promise.resolve()
    expect(update).toHaveBeenCalledWith(expect.objectContaining({ status: 'done' }))
    buttons[1]!.click()
    expect(retry).toHaveBeenCalledOnce()
  })

  it('aborts confirmation and suppresses a late state update after disposal', async () => {
    let resolve!: (state: { status: 'done'; done: number; total: number }) => void
    let signal!: AbortSignal
    const update = vi.fn()
    mount(() => <ImportRunResult state={{ status: 'done', done: 1, total: 1, result: { dryRun: true } }} confirm={(_state, nextSignal) => { signal = nextSignal; return new Promise((done) => { resolve = done }) }} onStateChange={update} />)
    host.querySelector('button')!.click()
    dispose?.()
    dispose = undefined
    resolve({ status: 'done', done: 1, total: 1 })
    await Promise.resolve()
    expect(signal.aborted).toBe(true)
    expect(update).not.toHaveBeenCalled()
  })
})

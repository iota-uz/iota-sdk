import { createMemo, Show } from 'solid-js'
import { createDraft } from '../../src/draft'
import { HostError } from '../../src/errors'
import { SDK_IDENTITY } from '../../src/identity'
import { mountSolidClientRouteFromDocument, useClientHost } from '../../src/solid'
import type { ClientRouteContext } from '../../src/bootstrap'
import './styles.css'

type Product = { name: string; pricing: { factor: number; baseAmount: number } }
type PendingSave = { snapshot: Product; resolve(saved: Product): void; reject(cause: unknown): void }

const params = new URLSearchParams(location.search)
const language = params.get('locale') ?? 'en'

// Declared backend boundary: saves wait until the test answers them, so loading and stale
// states are entered through controlled gates instead of fixed delays.
const pendingSaves: PendingSave[] = []
const backend = {
  pending: () => pendingSaves.length,
  respond(): boolean {
    const save = pendingSaves.shift()
    if (!save) return false
    if (!Number.isFinite(save.snapshot.pricing.factor) || save.snapshot.pricing.factor < 1) {
      save.reject(new HostError('field_validation', 'Review the highlighted fields', { 'pricing.factor': 'Factor must be at least 1' }))
    } else save.resolve(save.snapshot)
    return true
  },
}
declare global { interface Window { productEditorBackend: typeof backend } }
window.productEditorBackend = backend

function ProductEditor(props: { route: ClientRouteContext<Product> }) {
  const host = useClientHost()
  const draft = createDraft(props.route.initial, { navigation: host.navigation })
  const projectedTotal = createMemo(() => {
    const { baseAmount, factor } = draft.value.pricing
    return Number.isFinite(baseAmount) && Number.isFinite(factor) ? Math.round(baseAmount * factor) : null
  })
  const save = async () => {
    try {
      await draft.save((snapshot, signal) => new Promise<Product>((resolve, reject) => {
        const save: PendingSave = { snapshot, resolve, reject }
        pendingSaves.push(save)
        signal.addEventListener('abort', () => {
          pendingSaves.splice(pendingSaves.indexOf(save), 1)
          reject(new DOMException('Cancelled', 'AbortError'))
        }, { once: true })
      }))
    } catch {
      document.querySelector<HTMLElement>('[data-field-error]')?.focus()
    }
  }
  return (
    <div class="editor-shell" data-component-ready="true">
      <header>
        <div><h1>Pricing configuration</h1><p>Adjust the draft. The server remains the source of saved values.</p></div>
        <span class="status" data-dirty={draft.dirty()}>{draft.dirty() ? 'Unsaved' : 'Saved'}</span>
      </header>
      <div class="editor-grid">
        <section class="form-panel" aria-label="Pricing configuration">
          <label>Name<input aria-label="Name" value={draft.value.name} onInput={(event) => draft.set('name', event.currentTarget.value)} /></label>
          <label>Factor<input aria-label="Factor" type="number" value={draft.value.pricing.factor} onInput={(event) => draft.set('pricing', 'factor', event.currentTarget.valueAsNumber)} aria-invalid={Boolean(draft.fieldErrors()['pricing.factor'])} /></label>
          <Show when={draft.fieldErrors()['pricing.factor']}><p class="field-error" data-field-error tabindex="-1">{draft.fieldErrors()['pricing.factor']}</p></Show>
          <label>Base amount<input aria-label="Base amount" type="number" value={draft.value.pricing.baseAmount} onInput={(event) => draft.set('pricing', 'baseAmount', event.currentTarget.valueAsNumber)} /></label>
          <div class="actions">
            <button class="primary" disabled={!draft.dirty() || draft.pending()} onClick={save}>{draft.pending() ? 'Saving…' : 'Save changes'}</button>
            <button disabled={!draft.dirty() || draft.pending()} onClick={draft.reset}>Reset</button>
            <button disabled={!draft.pending()} onClick={draft.cancelSave}>Cancel save</button>
          </div>
          <a href="#next" data-next>Continue</a>
        </section>
        <aside aria-label="Calculated preview" data-empty={projectedTotal() === null}>
          <span>Calculated result</span>
          <Show when={projectedTotal() !== null} fallback={<p class="empty">Enter a factor and base amount to see the result</p>}>
            <strong>{projectedTotal()!.toLocaleString(props.route.locale?.language)}</strong>
          </Show>
          <small>updates with the draft</small>
        </aside>
      </div>
    </div>
  )
}

const context = {
  bootstrapVersion: '1.0.0',
  protocolVersion: SDK_IDENTITY.protocolVersion,
  sdkReleaseVersion: SDK_IDENTITY.releaseVersion,
  sdkCommit: SDK_IDENTITY.sourceCommit,
  initial: { name: 'Standard pricing', pricing: { factor: 4, baseAmount: 2_500 } },
  theme: params.get('theme') === 'dark' ? 'dark' : 'light',
  route: { id: 'fixture.product.edit', path: '/', featureId: 'fixture-product-editor' },
  locale: { language, messages: {} },
  session: { csrf: 'fixture' },
} satisfies ClientRouteContext<Product>

document.documentElement.lang = language
document.getElementById('iota-client-context')!.textContent = JSON.stringify(context)
mountSolidClientRouteFromDocument(ProductEditor, {})

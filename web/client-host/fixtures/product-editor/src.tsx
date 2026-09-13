import { createMemo, Show } from 'solid-js'
import { createDraft } from '../../src/draft'
import { HostError } from '../../src/errors'
import { SDK_IDENTITY } from '../../src/identity'
import { mountSolidClientRouteFromDocument, useClientHost } from '../../src/solid'
import type { ClientRouteContext } from '../../src/bootstrap'
import './styles.css'

type Product = { name: string; pricing: { factor: number; baseAmount: number } }

function ProductEditor(props: { route: ClientRouteContext<Product> }) {
  const host = useClientHost()
  const draft = createDraft(props.route.initial, { navigation: host.navigation })
  const projectedTotal = createMemo(() => Math.round(draft.value.pricing.baseAmount * draft.value.pricing.factor))
  const save = async () => {
    try {
      await draft.save(async (snapshot, signal) => {
        if (!Number.isFinite(snapshot.pricing.factor) || snapshot.pricing.factor < 1) {
          throw new HostError('field_validation', 'Review the highlighted fields', { 'pricing.factor': 'Factor must be at least 1' })
        }
        await new Promise<void>((resolve, reject) => {
          const timer = window.setTimeout(resolve, 180)
          signal.addEventListener('abort', () => { window.clearTimeout(timer); reject(new DOMException('Cancelled', 'AbortError')) }, { once: true })
        })
        return snapshot
      })
    } catch {
      document.querySelector<HTMLElement>('[data-field-error]')?.focus()
    }
  }
  return (
    <div class="editor-shell">
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
        <aside aria-label="Calculated preview"><span>Calculated result</span><strong>{projectedTotal().toLocaleString()}</strong><small>updates with the draft</small></aside>
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
  theme: new URLSearchParams(location.search).get('theme') === 'dark' ? 'dark' : 'light',
  route: { id: 'fixture.product.edit', path: '/', featureId: 'fixture-product-editor' },
  locale: { language: 'en', messages: {} },
  session: { csrf: 'fixture' },
} satisfies ClientRouteContext<Product>

document.getElementById('iota-client-context')!.textContent = JSON.stringify(context)
mountSolidClientRouteFromDocument(ProductEditor, {})

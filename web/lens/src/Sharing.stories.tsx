import { onCleanup, onMount } from 'solid-js'
import fixture from '../fixtures/small.json'
import { parseDocument } from './contract'
import { DashboardPanels } from './DashboardPanels'
import { DashboardRuntimeProvider, DocumentProvider } from './runtime'
import './styles.css'

function SharingScene(props: { openPanelMenu?: boolean }) {
  const document_ = parseDocument({ ...fixture, endpoints: { ...fixture.endpoints, export: '/story/export' } })
  return (
    <div class="lens-root lens-story-shell" data-theme="light">
      <DocumentProvider initialDocument={document_}>
        <DashboardRuntimeProvider locale="en">
          {() => (
            <>
              {props.openPanelMenu ? <OpenPanelMenu /> : null}
              <DashboardPanels />
            </>
          )}
        </DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  )
}

/** Opens the first panel's export menu one frame after mount. */
function OpenPanelMenu() {
  onMount(() => {
    const frame = requestAnimationFrame(() => {
      document.querySelector<HTMLButtonElement>('button[aria-label="Export panel"]')?.click()
    })
    onCleanup(() => cancelAnimationFrame(frame))
  })
  return null
}

export const SliceLink = () => <SharingScene />
export const PanelImageFormats = () => <SharingScene openPanelMenu />

import { createContext, Show, useContext, type Component, type JSX } from 'solid-js'
import { Dynamic } from 'solid-js/web'
import type { Panel } from '../contract'
import { usePanelFrame, useTranslate, type PanelFrameState } from '../runtime'
import { PanelFrame } from './PanelFrame'

/** What a host renderer receives: the panel, its live frame state and the producer's options verbatim. */
export interface HostPanelRendererProps {
  panel: Panel
  frame: PanelFrameState
  options: unknown
}

/**
 * A host-owned body for a panel whose document names it (panel.renderer). It
 * renders only the figure: the SDK frame around it owns the title, loading,
 * empty, error and stale states, retry, export and the placement menu.
 */
export type HostPanelRenderer = Component<HostPanelRendererProps>

/** Host renderers keyed by the wire's panel.renderer.name. */
export type HostPanelRenderers = Readonly<Record<string, HostPanelRenderer>>

const HostRenderersContext = createContext<HostPanelRenderers>()

export function HostRenderersProvider(props: { renderers?: HostPanelRenderers; children: JSX.Element }) {
  return <HostRenderersContext.Provider value={props.renderers}>{props.children}</HostRenderersContext.Provider>
}

export function useHostRenderers(): (name: string | undefined) => HostPanelRenderer | undefined {
  const renderers = useContext(HostRenderersContext)
  return (name) => name && renderers && Object.hasOwn(renderers, name) ? renderers[name] : undefined
}

/**
 * A panel the producer handed to a host renderer. Without that renderer the
 * card says so instead of drawing the panel as its kind: a legacy view is not
 * replaced by a chart that merely looks similar.
 */
export function HostRenderedPanel(props: { panel: Panel }) {
  const translate = useTranslate()
  const frame = usePanelFrame(props.panel.id)
  const lookup = useHostRenderers()
  const renderer = () => lookup(props.panel.renderer?.name)
  return (
    <PanelFrame panel={props.panel} frame={frame} allowEmptyContent={!renderer()}>
      <Show when={renderer()} fallback={
        <div class="lens-panel-state lens-panel-state-compatibility" data-panel-state="compatibility" role="status">
          {translate('panel.renderer.unavailable', 'This view cannot be shown here. Its data is unchanged and can still be exported.')}
        </div>
      }>
        {(resolved) => <Dynamic component={resolved()} panel={props.panel} frame={frame} options={props.panel.renderer?.options} />}
      </Show>
    </PanelFrame>
  )
}

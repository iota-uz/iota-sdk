import { PANEL_KIND_CONFIG_FIELDS, type Panel, type PanelKind } from '../contract'

const kindConfigFields = new Set<string>(Object.values(PANEL_KIND_CONFIG_FIELDS).flat())

/** Re-project a panel into another renderer kind without leaking old props. */
export function retargetPanel(panel: Panel, kind: PanelKind): Panel {
  const result = { ...panel, kind } as Record<string, unknown>
  const allowed = new Set<string>(PANEL_KIND_CONFIG_FIELDS[kind])
  for (const field of kindConfigFields) {
    if (!allowed.has(field)) delete result[field]
  }
  return result as unknown as Panel
}

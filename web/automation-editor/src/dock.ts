// The docked panel as a URL value (D1, #1352): `dry` for Dry run, `test:<step>` for Test node of one step.
import type { EditorDock } from './types'

export function dockParam(dock: EditorDock | null): string | null {
  return dock ? (dock.kind === 'dry' ? 'dry' : `test:${dock.node}`) : null
}

/** A value that names no dock → null. */
export function parseDock(value: string | null | undefined): EditorDock | null {
  if (value === 'dry') return { kind: 'dry' }
  if (value?.startsWith('test:') && value.length > 'test:'.length) return { kind: 'test', node: value.slice('test:'.length) }
  return null
}

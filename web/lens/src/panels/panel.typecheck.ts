import type { Panel } from '../contract'

const common = {
  id: 'typecheck',
  title: 'Typecheck',
  semantics: 'series',
  frame: 'typecheck:root',
  encoding: { value: 'value' },
  format: {},
  actions: [] as Panel['actions'],
} as const

const bar: Panel = { ...common, kind: 'bar' }
void bar

// A renderer cannot accidentally receive configuration owned by another kind.
// @ts-expect-error table configuration does not belong to a bar panel
const invalidBar: Panel = { ...common, kind: 'bar', table: { searchable: true } }
void invalidBar

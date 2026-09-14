export const THEMES = ['light', 'dark'] as const
export type Theme = (typeof THEMES)[number]

export const SPECIMENS = [
  {
    id: 'gallery-shell',
    title: 'Gallery shell',
    description: 'The deterministic host used by every Solid UI parity specimen.',
    states: ['default'],
  },
  {
    id: 'buttons',
    title: 'Buttons',
    description: 'Templ button variants, sizes, loading, hover, focus, and disabled states.',
    states: ['default', 'hover', 'focus', 'disabled', 'loading'],
  },
  {
    id: 'form-controls',
    title: 'Form controls',
    description: 'Inputs, selects, textareas, and checkboxes with their validation states.',
    states: ['default', 'hover', 'focus', 'disabled', 'error', 'checked'],
  },
  {
    id: 'advanced-forms',
    title: 'Advanced forms',
    description: 'Radios, switches, sliders, segmented toggles, phone, date range, and upload controls.',
    states: ['default', 'focus', 'disabled', 'error', 'checked'],
  },
  {
    id: 'display',
    title: 'Display',
    description: 'Cards, alerts, badges, progress, loading indicators, and skeletons.',
    states: ['default'],
  },
  {
    id: 'navigation',
    title: 'Navigation',
    description: 'Breadcrumbs, line tabs, animated navigation tabs, pagination, and dropdowns.',
    states: ['default', 'focus', 'selected', 'open'],
  },
  {
    id: 'dialog',
    title: 'Dialog',
    description: 'Confirmation dialog in its closed and modal-open states.',
    states: ['default', 'open'],
  },
  {
    id: 'drawer',
    title: 'Drawer',
    description: 'Responsive view drawer in its closed and modal-open states.',
    states: ['default', 'open'],
  },
  {
    id: 'toasts',
    title: 'Toasts',
    description: 'Persistent success, warning, and error notification surfaces.',
    states: ['default', 'open'],
  },
  {
    id: 'selection',
    title: 'Selection',
    description: 'Comboboxes, async search, countries, and calendar controls in closed and expanded states.',
    states: ['default', 'focus', 'disabled', 'selected', 'open'],
  },
  {
    id: 'data',
    title: 'Data display',
    description: 'Avatars, description lists, tables, selection, sorting, and loading states.',
    states: ['default', 'focus', 'selected', 'open', 'loading'],
  },
  {
    id: 'utilities',
    title: 'Utilities',
    description: 'Copy, export, help, and language utilities with focus and expanded states.',
    states: ['default', 'focus', 'open', 'loading'],
  },
  {
    id: 'scaffold',
    title: 'Page scaffold',
    description: 'Canonical action bars, filters, and form layout building blocks.',
    states: ['default', 'focus', 'disabled', 'error', 'open'],
  },
] as const

export type SpecimenID = (typeof SPECIMENS)[number]['id']
export type SpecimenState = (typeof SPECIMENS)[number]['states'][number]

export function isTheme(value: string | null): value is Theme {
  return THEMES.some((theme) => theme === value)
}

export function isSpecimenID(value: string | null): value is SpecimenID {
  return SPECIMENS.some((specimen) => specimen.id === value)
}

export function stateFor(specimenID: SpecimenID, value: string | null): SpecimenState {
  const specimen = SPECIMENS.find((item) => item.id === specimenID)
  return specimen?.states.some((state) => state === value) ? value as SpecimenState : 'default'
}

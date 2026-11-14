// Client graph of "acme.insurance": pages, navigation, slot, cell, shell.
import { cell, navigation, page, shellRenderer, slot } from '../../src/client.ts'
import * as kit from '../kit.ts'
import * as c from './contracts.ts'

export const policies = page('acme.insurance.policies', {
  path: '/insurance/policies',
  nav: { label: { en: 'Policies', ru: 'Полисы' }, group: 'acme.insurance', icon: 'shield' },
  permission: c.ReadPolicies,
  component: () => 'Policies',
})
export const policy = page(c.PolicyRoute, { component: (p) => p.params.id })
// replaces a route of the platform/another package (from the kit): a candidate, bound at activation
export const ClientView = page(kit.ClientView, { component: (p) => p.params.id })
export const nav = navigation({
  groups: [{ id: 'acme.insurance', label: { en: 'Insurance' } }],
  patches: [{ ref: 'core.users', hide: true }],
  workspaces: [{ id: 'acme.insurance.work', label: { en: 'Insurance' } }],
  palette: [{ id: 'acme.insurance.openPolicies', label: { en: 'Open policies' }, target: 'acme.insurance.policies' }],
})
export const tierCell = cell(c.ClientsTierColumn, (p) => p.value ?? '')
export const clientBadge = slot(kit.ClientToolbar, (p) => p.client.key)
export const shell = shellRenderer((p) => p.outlet)

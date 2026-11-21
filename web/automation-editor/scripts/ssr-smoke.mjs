// Importing the subpath on the server touches no DOM: the outline and the loading skeleton render to a string.
import { renderToString } from 'solid-js/web'
import { Editor, Outline } from '../package-dist/index.server.js'

const catalog = { revision: '1', events: [{ key: 'core.user.created.v1', title: 'User created', group: 'Users', subject: 'core.user', payload: {}, trigger_params: null }], nodes: [], context_schema: {} }
const graph = { name: 'Smoke', run_as: 'owner', triggers: [{ id: 'user_created', event: 'core.user.created.v1' }], nodes: [], edges: [] }
const outline = renderToString(() => Outline({ graph, catalog, locale: 'en' }))
if (!outline.includes('User created')) throw new Error('Outline did not render on the server')
const editor = renderToString(() => Editor({ graph, catalog, layout: {}, version: 0, locale: 'en', enabled: false, api: {}, save: async () => ({ errors: [] }), reload: async () => ({ graph, version: 0, layout: {} }) }))
if (!editor.includes('ae-skeleton')) throw new Error('Editor did not render its loading skeleton on the server')
console.log('automation-editor SSR smoke: ok')

import { renderToString } from 'solid-js/web'
import { Button } from '../package-dist/index.server.js'

const html = renderToString(() => Button({ children: 'SSR smoke' }))
if (!html.includes('SSR smoke') || !html.includes('btn-primary')) {
  throw new Error('Solid UI server bundle did not render the canonical button markup')
}

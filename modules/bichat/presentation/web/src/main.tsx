import { injectMockContext } from './dev/mockIotaContext'
import { defineReactAppletElement } from '@iota-uz/sdk'
import App from './App'
import compiledStyles from '../dist/style.css?raw'

injectMockContext()

defineReactAppletElement({
  tagName: 'bi-chat-root',
  styles: compiledStyles,
  render: (host) => <App basePath={host.basePath} routerMode={host.routerMode} />,
})

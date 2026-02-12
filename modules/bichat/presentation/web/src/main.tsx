import { injectMockContext } from './dev/mockIotaContext'
import { defineReactAppletElement } from '@iota-uz/sdk'
import appletStyles from 'virtual:applet-styles'
import App from './App'

injectMockContext()

defineReactAppletElement({
  tagName: 'bi-chat-root',
  styles: appletStyles,
  render: (host) => <App basePath={host.basePath} routerMode={host.routerMode} />,
})

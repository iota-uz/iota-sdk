import { App } from './App'
import { defineSolidAppletElement } from './applet/solidAppletElement'
import { injectMockContext } from './dev/mockIotaContext'

injectMockContext()

defineSolidAppletElement('bi-chat-root', App)

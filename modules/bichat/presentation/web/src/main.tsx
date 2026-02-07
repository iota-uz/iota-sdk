import { injectMockContext } from './dev/mockIotaContext'
import { defineReactAppletElement } from '@iota-uz/sdk'
import App from './App'

injectMockContext()

async function main() {
  let compiledStyles = ''
  try {
    compiledStyles = (await import('../dist/style.css?raw')).default
  } catch {
    // CSS not built yet (e.g. fresh checkout). Dev scripts will build it.
  }

  defineReactAppletElement({
    tagName: 'bi-chat-root',
    styles: compiledStyles,
    render: (host) => <App basePath={host.basePath} routerMode={host.routerMode} />,
  })
}

void main()

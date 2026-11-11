import { SDK_IDENTITY } from '../../../../../../web/client-host/src/identity'
import { mountSolidClientRouteFromDocument } from '../../../../../../web/client-host/src/solid'
import Configurator from '../../Configurator'
import { loadMonaco } from '../../monaco'

// What the Go controller sends (configurator.Props, host context, messages).
const params = new URLSearchParams(window.location.search)
const theme = params.get('theme') === 'dark' ? 'dark' : 'light'
const permissions = (params.get('permissions') ?? 'Extensions.Read,Extensions.Author,Extensions.Review,Extensions.Publish,Extensions.Operate').split(',')
const messages: Record<string, string> = {
  'Extensions.Configurator.Title': 'Extensions',
  'Extensions.Configurator.Save': 'Save',
  'Extensions.Configurator.Check': 'Check',
  'Extensions.Configurator.Problems': 'Problems',
  'Extensions.Configurator.Files': 'Files',
  'Extensions.Configurator.Conflict': 'Conflict.',
}
document.documentElement.dataset.theme = theme
const context = {
  bootstrapVersion: '1.0.0',
  protocolVersion: SDK_IDENTITY.protocolVersion,
  sdkReleaseVersion: SDK_IDENTITY.releaseVersion,
  sdkCommit: SDK_IDENTITY.sourceCommit,
  theme,
  csrf: 'csrf-token',
  route: { id: 'extruntime.configurator', path: '/extensions', featureId: 'fixture' },
  locale: { language: 'en', messages },
  permissions,
  initial: { basePath: '/extensions', apiBase: '/_ext/manage', typesBase: '/extensions/types' },
}
document.getElementById('iota-client-context')!.textContent = JSON.stringify(context)

const dispose = mountSolidClientRouteFromDocument(Configurator, {})
const w = window as unknown as Record<string, unknown>
w.__dispose = dispose
void loadMonaco().then((m) => { w.__monaco = m })

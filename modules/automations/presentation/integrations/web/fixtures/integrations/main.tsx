import { SDK_IDENTITY } from '../../../../../../../web/client-host/src/identity'
import { mountSolidClientRouteFromDocument } from '../../../../../../../web/client-host/src/solid'
// Loaded like the application's catalog does: through the generated binding.
import Integrations from '../../Integrations.solid.generated'
import en from '../../../../locales/en.json'
import ru from '../../../../locales/ru.json'
import uz from '../../../../locales/uz.json'
import uzCyrl from '../../../../locales/uz-Cyrl.json'
import { installFake, type FakeOptions } from './fake'
import './fixture.generated.css'

// What the Go controller sends: integrations.Props, the host context and the module's messages.
const params = new URLSearchParams(window.location.search)
const theme = params.get('theme') === 'dark' ? 'dark' : 'light'
const lang = params.get('lang') ?? 'en'
const locales: Record<string, unknown> = { en, ru, uz, 'uz-Cyrl': uzCyrl }

function flatten(prefix: string, value: unknown, out: Record<string, string>) {
  if (typeof value === 'string') { out[prefix] = value; return }
  if (value && typeof value === 'object') for (const [k, v] of Object.entries(value)) flatten(prefix ? `${prefix}.${k}` : k, v, out)
}
const all: Record<string, string> = {}
flatten('', locales[lang] ?? en, all)
const messages = Object.fromEntries(Object.entries(all).filter(([k]) => k.startsWith('Automations.Integrations.')))

const fake: FakeOptions = {
  scenario: (params.get('scenario') as FakeOptions['scenario']) ?? 'default',
  delay: Number(params.get('delay') ?? '0'),
  platformFails: params.has('platformFails'),
  manage: params.get('manage') !== '0',
  testDelay: Number(params.get('testDelay') ?? '0'),
  lossy: params.has('lossy'),
  oauthFail: params.get('oauthFail') ?? '',
  testFailOnce: params.has('testFailOnce'),
  conflictOnce: params.has('conflictOnce'),
  calls: [],
}
;(window as unknown as { __calls: FakeOptions['calls'] }).__calls = fake.calls
installFake('/settings/integrations/rpc', fake)

document.documentElement.dataset.theme = theme
document.documentElement.lang = lang
if (theme === 'dark') document.documentElement.classList.add('dark')
const context = {
  bootstrapVersion: '1.0.0',
  protocolVersion: SDK_IDENTITY.protocolVersion,
  sdkReleaseVersion: SDK_IDENTITY.releaseVersion,
  sdkCommit: SDK_IDENTITY.sourceCommit,
  theme,
  route: { id: 'automations.integrations', path: '/settings/integrations', featureId: 'fixture' },
  locale: { language: lang, messages },
  permissions: ['Connections.Manage'],
  services: { rpc: '/settings/integrations/rpc' },
  initial: { basePath: '/settings/integrations', automationsPath: '/automations', configured: !params.has('unconfigured') },
}
document.getElementById('iota-client-context')!.textContent = JSON.stringify(context)
mountSolidClientRouteFromDocument(Integrations, {})

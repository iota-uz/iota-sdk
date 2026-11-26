import { SDK_IDENTITY } from '../../../../../../../web/client-host/src/identity'
import { mountSolidClientRouteFromDocument } from '../../../../../../../web/client-host/src/solid'
// Loaded like the application's catalog does: through the generated binding.
import Automations from '../../Automations.solid.generated'
import en from '../../../../locales/en.json'
import ru from '../../../../locales/ru.json'
import uz from '../../../../locales/uz.json'
import uzCyrl from '../../../../locales/uz-Cyrl.json'
import { installFake, type FakeOptions } from './fake'
import './fixture.generated.css'

// What the Go controller sends: graphs.Props, the host context and the module's messages.
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
const messages = Object.fromEntries(Object.entries(all).filter(([k]) => k.startsWith('Automations.Graphs.')))

const cannot = new Set((params.get('cannot') ?? '').split(',').filter(Boolean))
const agent = params.has('agent')
const can = { author: !cannot.has('author'), enable: !cannot.has('enable') && !agent, operate: !cannot.has('operate') }
const fake: FakeOptions = {
  scenario: (params.get('scenario') as FakeOptions['scenario']) ?? 'default',
  delay: Number(params.get('delay') ?? '0'),
  labelFails: params.get('labelFails') ?? undefined,
  delayOnly: params.get('delayOnly') ?? undefined,
  staleReads: params.has('staleReads'),
  holdOn: params.get('holdOn') ?? undefined,
  changeOnce: params.has('changeOnce'),
  layoutConflict: params.has('layoutConflict'),
  conflictOnce: params.has('conflictOnce'),
  liveFailsOnce: params.has('liveFailsOnce'),
  testLostOnce: params.has('testLostOnce'),
  statePage: Number(params.get('statePage') ?? '0'),
  reviewFailsAfterChange: params.has('reviewFailsAfterChange'),
  oldVersionDelay: Number(params.get('oldVersionDelay') ?? '0'),
  hold: params.has('hold'),
  holdFirst: params.get('hold') === 'all',
  can,
  calls: [],
  warn: params.has('warn'),
  telegram: params.has('telegram'),
  agent,
  backtests: (params.get('bt') as FakeOptions['backtests']) ?? 'default',
}
;(window as unknown as { __calls: FakeOptions['calls'] }).__calls = fake.calls
installFake('/automations/rpc', fake)

document.documentElement.dataset.theme = theme
document.documentElement.lang = lang
if (theme === 'dark') document.documentElement.classList.add('dark')
const view = (params.get('view') ?? 'list') as 'list' | 'new' | 'editor' | 'backtests'
const context = {
  bootstrapVersion: '1.0.0',
  protocolVersion: SDK_IDENTITY.protocolVersion,
  sdkReleaseVersion: SDK_IDENTITY.releaseVersion,
  sdkCommit: SDK_IDENTITY.sourceCommit,
  theme,
  route: { id: `automations.${view}`, path: '/automations', featureId: 'fixture' },
  locale: { language: lang, messages },
  permissions: [],
  services: { rpc: '/automations/rpc' },
  initial: {
    basePath: '/automations', integrationsPath: '/settings/integrations', view,
    ...(params.get('id') ? { id: params.get('id')! } : {}), ...(params.get('run') ? { run: params.get('run')! } : {}),
    can, agent, state: params.get('state') !== 'off', backtests: !params.has('nobacktests'),
  },
}
document.getElementById('iota-client-context')!.textContent = JSON.stringify(context)
mountSolidClientRouteFromDocument(Automations, {})

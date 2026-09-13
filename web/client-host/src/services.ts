import type { ClientRouteContext } from './bootstrap'
import { BrowserNavigationService, type NavigationService } from './navigation'
import { HostError } from './errors'
import { ManagedSession, type SessionService, type SessionState } from './transport'

export interface ThemeService {
  current(): 'light' | 'dark'
  set(theme: 'light' | 'dark'): void
  subscribe(listener: (theme: 'light' | 'dark') => void): () => void
}

class HostThemeService implements ThemeService {
  private theme: 'light' | 'dark'
  private readonly listeners = new Set<(theme: 'light' | 'dark') => void>()
  constructor(theme: 'light' | 'dark', private readonly root: HTMLElement) {
    this.theme = theme
    this.root.dataset.theme = theme
  }
  current(): 'light' | 'dark' { return this.theme }
  set(theme: 'light' | 'dark'): void {
    if (theme === this.theme) return
    this.theme = theme
    this.root.dataset.theme = theme
    for (const listener of this.listeners) listener(theme)
  }
  subscribe(listener: (theme: 'light' | 'dark') => void): () => void {
    this.listeners.add(listener)
    return () => this.listeners.delete(listener)
  }
}

export interface ClientHostServices {
  session: SessionService
  navigation: NavigationService
  theme: ThemeService
  locale: { language: string; t(key: string): string }
  telemetry: { emit(name: string, data?: unknown): void }
  dispose(): void
}

export function createClientHostServices(context: ClientRouteContext, owner: Window = window): ClientHostServices {
  const navigation = new BrowserNavigationService(owner)
  const initialSession: SessionState = {
    ...(context.session?.csrf || context.csrf ? { csrf: context.session?.csrf ?? context.csrf } : {}),
    ...(context.session?.expiresAt ? { expiresAt: context.session.expiresAt } : {}),
  }
  const session = new ManagedSession(initialSession, async (signal) => {
    const endpoint = context.session?.refreshEndpoint
    if (!endpoint) throw new HostError('unauthenticated', 'Session expired', { reauthUrl: context.session?.reauthUrl, returnUrl: owner.location.href })
    const response = await owner.fetch(endpoint, { method: 'POST', credentials: 'same-origin', signal })
    if (!response.ok) throw new HostError('unauthenticated', 'Session expired', { reauthUrl: context.session?.reauthUrl, returnUrl: owner.location.href })
    return await response.json() as SessionState
  })
  const messages = context.locale?.messages ?? {}
  return {
    session,
    navigation,
    theme: new HostThemeService(context.theme, owner.document.documentElement),
    locale: { language: context.locale?.language ?? 'en', t: (key) => messages[key] ?? key },
    telemetry: {
      emit(name, data) {
        if (!context.services?.telemetry) return
        owner.dispatchEvent(new CustomEvent('iota:telemetry', { detail: { endpoint: context.services.telemetry, name, data } }))
      },
    },
    dispose: () => navigation.dispose(),
  }
}

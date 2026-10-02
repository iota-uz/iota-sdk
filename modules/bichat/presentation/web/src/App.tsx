import { Route, MemoryRouter, Router, useLocation, useNavigate } from '@solidjs/router'
import { Dynamic } from 'solid-js/web'
import { createEffect, type JSX } from 'solid-js'
import { AppProvider, TouchProvider, resolveAppContext } from './context/appContext'
import { I18nProvider, createTranslator } from './i18n/i18n'
import { SessionEventProvider } from './contexts/SessionEventContext'
import { Layout } from './components/Layout'
import { HomePage } from './pages/HomePage'
import { ChatPage } from './pages/ChatPage'
import { ArchivedPage } from './pages/ArchivedPage'
import { ToastProvider } from './ui/toast'
import { AppErrorBoundary } from './ui/ErrorBoundary'

function inScope(pathname: string, basePath: string): boolean {
  const withoutBase =
    basePath && pathname.startsWith(basePath) ? pathname.slice(basePath.length) : pathname
  return (
    withoutBase === '/' ||
    withoutBase === '' ||
    withoutBase.startsWith('/session/') ||
    withoutBase === '/archived'
  )
}

function RouteGuard(props: { children: JSX.Element }): JSX.Element {
  const ctx = resolveAppContext()
  const location = useLocation()
  const navigate = useNavigate()
  createEffect(() => {
    if (!inScope(location.pathname, ctx.config.basePath ?? '')) {
      navigate('/', { replace: true })
    }
  })
  return props.children
}

function RedirectHome(): JSX.Element {
  const navigate = useNavigate()
  createEffect(() => navigate('/', { replace: true }))
  return null
}

export function App(props: { host: { basePath: string; routerMode: 'url' | 'memory' } }): JSX.Element {
  const translator = createTranslator(resolveAppContext())
  return (
    <AppProvider>
      <I18nProvider value={translator}>
        <ToastProvider>
          <SessionEventProvider>
            <TouchProvider>
            <Dynamic
              component={props.host.routerMode === 'memory' ? MemoryRouter : Router}
            >
              <Layout>
                <AppErrorBoundary>
                  <RouteGuard>
                    <Route path="/" component={HomePage} />
                    <Route path="/session/:id" component={ChatPage} />
                    <Route path="/archived" component={ArchivedPage} />
                    <Route path="*" component={RedirectHome} />
                  </RouteGuard>
                </AppErrorBoundary>
              </Layout>
            </Dynamic>
            </TouchProvider>
          </SessionEventProvider>
        </ToastProvider>
      </I18nProvider>
    </AppProvider>
  )
}

import { BrowserRouter, MemoryRouter, Routes, Route, Navigate } from 'react-router-dom'
import { IotaContextProvider } from './contexts/IotaContext'
import { ToastProvider } from './contexts/ToastContext'
import { TouchProvider } from './contexts/TouchContext'
import { SessionEventProvider } from './contexts/SessionEventContext'
import { ErrorBoundary, DefaultErrorContent } from '@iota-uz/sdk/bichat'
import Layout from './components/Layout'
import ChatPage from './pages/ChatPage'
import HomePage from './pages/HomePage'
import ArchivedPage from './pages/ArchivedPage'
import { useNavigationGuard } from './hooks/useNavigationGuard'

export interface AppProps {
  basePath: string
  routerMode: 'url' | 'memory'
}

export default function App({ basePath, routerMode }: AppProps) {
  const Router = routerMode === 'memory' ? MemoryRouter : BrowserRouter

  function GuardedRoutes() {
    useNavigationGuard()
    return (
      <Routes>
        <Route element={<Layout />}>
          <Route path="/" element={<HomePage />} />
          <Route path="/session/:id" element={<ChatPage />} />
          <Route path="/archived" element={<ArchivedPage />} />
        </Route>
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    )
  }

  return (
    <IotaContextProvider>
      <TouchProvider>
        <ToastProvider>
          <SessionEventProvider>
            <Router {...(routerMode === 'url' ? { basename: basePath } : {})}>
              <ErrorBoundary
                fallback={(error, reset) => (
                  <div className="flex h-full min-h-[50vh] items-center justify-center">
                    <div className="w-full max-w-lg">
                      <DefaultErrorContent error={error} onReset={reset} />
                      <div className="mt-4 flex items-center justify-center gap-3">
                        <button
                          type="button"
                          className="px-4 py-2 rounded-lg border border-gray-200 dark:border-gray-800 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-900 transition-colors"
                          onClick={() => {
                            if (routerMode === 'memory') {
                              window.location.reload()
                              return
                            }
                            const next = `${basePath || ''}/`.replace(/\/{2,}/g, '/')
                            window.location.assign(next)
                          }}
                        >
                          Go home
                        </button>
                      </div>
                    </div>
                  </div>
                )}
              >
                <GuardedRoutes />
              </ErrorBoundary>
            </Router>
          </SessionEventProvider>
        </ToastProvider>
      </TouchProvider>
    </IotaContextProvider>
  )
}

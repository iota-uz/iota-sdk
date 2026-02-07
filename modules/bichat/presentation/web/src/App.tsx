import { BrowserRouter, MemoryRouter, Routes, Route, Navigate } from 'react-router-dom'
import { IotaContextProvider } from './contexts/IotaContext'
import { ToastProvider } from './contexts/ToastContext'
import { TouchProvider } from './contexts/TouchContext'
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
          <Router {...(routerMode === 'url' ? { basename: basePath } : {})}>
            <GuardedRoutes />
          </Router>
        </ToastProvider>
      </TouchProvider>
    </IotaContextProvider>
  )
}

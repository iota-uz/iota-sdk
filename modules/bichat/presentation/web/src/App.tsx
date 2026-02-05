import { BrowserRouter, MemoryRouter, Routes, Route, Navigate } from 'react-router-dom'
import { IotaContextProvider } from './contexts/IotaContext'
import Layout from './components/Layout'
import ChatPage from './pages/ChatPage'
import HomePage from './pages/HomePage'

export interface AppProps {
  basePath: string
  routerMode: 'url' | 'memory'
}

export default function App({ basePath, routerMode }: AppProps) {
  const Router = routerMode === 'memory' ? MemoryRouter : BrowserRouter

  return (
    <IotaContextProvider>
      <Router {...(routerMode === 'url' ? { basename: basePath } : {})}>
        <Routes>
          <Route element={<Layout />}>
            <Route path="/" element={<HomePage />} />
            <Route path="/session/:id" element={<ChatPage />} />
          </Route>
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </Router>
    </IotaContextProvider>
  )
}

import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { IotaContextProvider } from './contexts/IotaContext'
import { GraphQLProvider } from './contexts/GraphQLContext'
import Layout from './components/Layout'
import ChatPage from './pages/ChatPage'
import HomePage from './pages/HomePage'

export default function App() {
  const basename = import.meta.env.DEV ? '' : '/bi-chat'

  return (
    <IotaContextProvider>
      <GraphQLProvider>
        <BrowserRouter basename={basename}>
          <Routes>
            <Route element={<Layout />}>
              <Route path="/" element={<HomePage />} />
              <Route path="/session/:id" element={<ChatPage />} />
            </Route>
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </BrowserRouter>
      </GraphQLProvider>
    </IotaContextProvider>
  )
}

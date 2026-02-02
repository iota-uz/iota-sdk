import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { IotaContextProvider } from './contexts/IotaContext'
import { GraphQLProvider } from './contexts/GraphQLContext'
import Layout from './components/Layout'
import ChatPage from './pages/ChatPage'
import EmptyState from './components/EmptyState'

export default function App() {
  const basename = import.meta.env.DEV ? '' : '/bi-chat'

  return (
    <IotaContextProvider>
      <GraphQLProvider>
        <BrowserRouter basename={basename}>
          <Routes>
            <Route element={<Layout />}>
              <Route
                path="/"
                element={
                  <EmptyState
                    title="No Chat Selected"
                    description="Select a chat from the sidebar or create a new one to start"
                  />
                }
              />
              <Route path="/session/:id" element={<ChatPage />} />
            </Route>
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </BrowserRouter>
      </GraphQLProvider>
    </IotaContextProvider>
  )
}

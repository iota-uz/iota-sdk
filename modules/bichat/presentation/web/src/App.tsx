import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { IotaContextProvider } from './contexts/IotaContext'
import { GraphQLProvider } from './contexts/GraphQLContext'
import ChatPage from './pages/ChatPage'
import SessionsPage from './pages/SessionsPage'

export default function App() {
  return (
    <IotaContextProvider>
      <GraphQLProvider>
        <BrowserRouter basename="/bichat">
          <Routes>
            <Route path="/" element={<SessionsPage />} />
            <Route path="/session/:id" element={<ChatPage />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </BrowserRouter>
      </GraphQLProvider>
    </IotaContextProvider>
  )
}

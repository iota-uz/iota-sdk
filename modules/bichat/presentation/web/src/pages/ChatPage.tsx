import { useCallback, useMemo } from 'react'
import { useParams, useNavigate, useLocation } from 'react-router-dom'
import { ChatSession } from '@iota-uz/sdk/bichat'
import { useBiChatDataSource } from '../data/bichatDataSource'

export default function ChatPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const location = useLocation()

  const readOnly = useMemo(() => {
    const params = new URLSearchParams(location.search)
    return params.get('readonly') === 'true'
  }, [location.search])

  const onNavigateToSession = useCallback(
    (sessionId: string) => navigate(`/session/${sessionId}`),
    [navigate]
  )
  const dataSource = useBiChatDataSource(onNavigateToSession)

  if (!id) {
    return (
      <div className="flex items-center justify-center h-screen">
        <div className="text-red-500">Session ID is required</div>
      </div>
    )
  }

  return (
    <ChatSession
      dataSource={dataSource}
      sessionId={id}
      readOnly={readOnly}
      showArtifactsPanel
      artifactsPanelDefaultExpanded={false}
      artifactsPanelStorageKey="bichat.web.artifacts-panel.expanded"
    />
  )
}

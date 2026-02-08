import { useCallback, useMemo } from 'react'
import { useParams, useNavigate, useLocation } from 'react-router-dom'
import { ChatSession, RateLimiter } from '@iota-uz/sdk/bichat'
import { useBiChatDataSource } from '../data/bichatDataSource'
import { useIotaContext } from '../contexts/IotaContext'

export default function ChatPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const location = useLocation()
  const context = useIotaContext()

  const readOnly = useMemo(() => {
    const params = new URLSearchParams(location.search)
    return params.get('readonly') === 'true'
  }, [location.search])

  const onNavigateToSession = useCallback(
    (sessionId: string) => navigate(`/session/${sessionId}`),
    [navigate]
  )
  const dataSource = useBiChatDataSource(onNavigateToSession)
  const rateLimiter = useMemo(
    () => new RateLimiter({ maxRequests: 20, windowMs: 60_000 }),
    []
  )

  if (!id) {
    return (
      <div className="flex items-center justify-center h-screen">
        <div className="text-red-500">Session ID is required</div>
      </div>
    )
  }

  const isAPIKeyConfigured = context.extensions?.llm?.apiKeyConfigured ?? true
  if (!isAPIKeyConfigured) {
    return (
      <div className="flex h-full min-h-0 flex-1 items-center justify-center px-6 py-10">
        <div className="w-full max-w-xl rounded-2xl border border-red-200 bg-red-50 p-6 text-center shadow-sm dark:border-red-900/70 dark:bg-red-950/30">
          <h1 className="text-lg font-semibold text-red-900 dark:text-red-200">
            API key is not configured
          </h1>
          <p className="mt-2 text-sm leading-relaxed text-red-800 dark:text-red-300">
            BiChat is unavailable until an LLM API key is configured on the server.
          </p>
        </div>
      </div>
    )
  }

  return (
    <ChatSession
      dataSource={dataSource}
      sessionId={id}
      readOnly={readOnly}
      rateLimiter={rateLimiter}
      showArtifactsPanel
      artifactsPanelDefaultExpanded={false}
      artifactsPanelStorageKey="bichat.web.artifacts-panel.expanded"
    />
  )
}

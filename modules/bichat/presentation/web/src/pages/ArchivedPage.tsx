import { useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { ArchivedChatList } from '@iota-uz/sdk/bichat'
import { useBiChatDataSource } from '../data/bichatDataSource'
import { useAppToast } from '../contexts/ToastContext'

export default function ArchivedPage() {
  const navigate = useNavigate()
  const toast = useAppToast()

  const onNavigateToSession = useCallback(
    (sessionId: string) => navigate(`/session/${sessionId}`),
    [navigate]
  )

  const dataSource = useBiChatDataSource(onNavigateToSession)

  return (
    <ArchivedChatList
      dataSource={dataSource}
      onBack={() => navigate('/')}
      onSessionSelect={onNavigateToSession}
      toast={toast}
    />
  )
}


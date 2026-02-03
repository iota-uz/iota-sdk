import { useParams, useNavigate } from 'react-router-dom'
import { ChatSession } from '@iotauz/bichat-ui'
import { useBiChatDataSource } from '../data/bichatDataSource'

export default function ChatPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()

  const dataSource = useBiChatDataSource((sessionId: string) => navigate(`/session/${sessionId}`))

  if (!id) {
    return (
      <div className="flex items-center justify-center h-screen">
        <div className="text-red-500">Session ID is required</div>
      </div>
    )
  }

  return <ChatSession dataSource={dataSource} sessionId={id} />
}

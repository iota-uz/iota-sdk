import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { createAppletRPCClient } from '@iota-uz/sdk'
import { useIotaContext } from '../contexts/IotaContext'

type ChatSession = {
  id: string
  title: string
  createdAt: string
  updatedAt: string
}

export default function SessionsPage() {
  const navigate = useNavigate()
  const { config } = useIotaContext()
  const rpc = useMemo(
    () => createAppletRPCClient({ endpoint: config.rpcUIEndpoint }),
    [config.rpcUIEndpoint]
  )

  const [fetching, setFetching] = useState(true)
  const [sessions, setSessions] = useState<ChatSession[]>([])

  useEffect(() => {
    let alive = true
    ;(async () => {
      setFetching(true)
      try {
        const data = await rpc.call<{ limit: number; offset: number }, { sessions: ChatSession[] }>(
          'bichat.session.list',
          { limit: 200, offset: 0 }
        )
        if (alive) setSessions(data.sessions || [])
      } finally {
        if (alive) setFetching(false)
      }
    })()
    return () => {
      alive = false
    }
  }, [rpc])

  const handleCreateSession = async () => {
    const result = await rpc.call<{ title: string }, { session: { id: string } }>('bichat.session.create', {
      title: '',
    })
    if (result.session?.id) {
      navigate(`/session/${result.session.id}`)
    }
  }

  if (fetching) return <div>Loading...</div>

  return (
    <div className="p-4">
      <div className="flex justify-between items-center mb-4">
        <h1 className="text-2xl font-bold">Sessions</h1>
        <button
          onClick={handleCreateSession}
          className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600"
        >
          New Session
        </button>
      </div>
      <div className="space-y-2">
        {sessions.map((session) => (
          <div
            key={session.id}
            onClick={() => navigate(`/session/${session.id}`)}
            className="p-4 border rounded cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800"
          >
            <h3 className="font-medium">{session.title || 'Untitled Session'}</h3>
            <p className="text-sm text-gray-500">
              {new Date(session.updatedAt).toLocaleDateString()}
            </p>
          </div>
        ))}
      </div>
    </div>
  )
}

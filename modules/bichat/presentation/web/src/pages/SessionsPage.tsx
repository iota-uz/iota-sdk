import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { createAppletRPCClient } from '@iota-uz/sdk'
import { useIotaContext } from '../contexts/IotaContext'
import { toRPCErrorDisplay, type RPCErrorDisplay } from '../utils/rpcErrors'

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
  const [loadError, setLoadError] = useState<RPCErrorDisplay | null>(null)

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
        if (alive) setLoadError(null)
      } catch (error) {
        if (alive) setLoadError(toRPCErrorDisplay(error, 'Failed to load sessions'))
      } finally {
        if (alive) setFetching(false)
      }
    })()
    return () => {
      alive = false
    }
  }, [rpc])

  const handleCreateSession = async () => {
    try {
      const result = await rpc.call<{ title: string }, { session: { id: string } }>('bichat.session.create', {
        title: '',
      })
      if (result.session?.id) {
        navigate(`/session/${result.session.id}`)
      }
      setLoadError(null)
    } catch (error) {
      setLoadError(toRPCErrorDisplay(error, 'Failed to create session'))
    }
  }

  if (fetching) return <div>Loading...</div>

  return (
    <div className="p-4">
      <div className="flex justify-between items-center mb-4">
        <h1 className="text-2xl font-bold">Sessions</h1>
        <button
          onClick={handleCreateSession}
          disabled={loadError?.isPermissionDenied}
          className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600"
        >
          New Session
        </button>
      </div>
      {loadError && (
        <div
          className={
            loadError.isPermissionDenied
              ? 'mb-4 p-3 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg'
              : 'mb-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg'
          }
        >
          <p
            className={
              loadError.isPermissionDenied
                ? 'text-sm text-amber-700 dark:text-amber-300 font-medium'
                : 'text-sm text-red-700 dark:text-red-300 font-medium'
            }
          >
            {loadError.title}
          </p>
          <p
            className={
              loadError.isPermissionDenied
                ? 'text-sm text-amber-600 dark:text-amber-400'
                : 'text-sm text-red-600 dark:text-red-400'
            }
          >
            {loadError.description}
          </p>
        </div>
      )}
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

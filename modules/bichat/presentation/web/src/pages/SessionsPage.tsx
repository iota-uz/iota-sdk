import { useQuery, useMutation } from 'urql'
import { useNavigate } from 'react-router-dom'

const SessionsQuery = `
  query Sessions {
    sessions {
      id
      title
      createdAt
      updatedAt
    }
  }
`

const CreateSessionMutation = `
  mutation CreateSession {
    createSession {
      id
      title
    }
  }
`

export default function SessionsPage() {
  const navigate = useNavigate()
  const [{ data, fetching }] = useQuery({ query: SessionsQuery })
  const [, createSession] = useMutation(CreateSessionMutation)

  const handleCreateSession = async () => {
    const result = await createSession({})
    if (result.data?.createSession) {
      navigate(`/session/${result.data.createSession.id}`)
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
        {data?.sessions.map((session: any) => (
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

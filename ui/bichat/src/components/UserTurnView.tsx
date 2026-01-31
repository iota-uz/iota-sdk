/**
 * UserTurnView component
 * Displays user messages
 */

import { Message } from '../types'
import { useChat } from '../context/ChatContext'

interface UserTurnViewProps {
  message: Message
}

export function UserTurnView({ message }: UserTurnViewProps) {
  const { handleEdit } = useChat()

  return (
    <div className="flex gap-3 justify-end group">
      <div className="flex-1 flex flex-col items-end max-w-2xl">
        <div className="rounded-2xl px-5 py-3 bg-[var(--bichat-bubble-user)] text-white">
          <div className="text-base whitespace-pre-wrap">{message.content}</div>
        </div>
        <div className="flex items-center gap-2 mt-1 px-1 opacity-0 group-hover:opacity-100 transition-opacity">
          <span className="text-xs text-gray-500">
            {new Date(message.createdAt).toLocaleTimeString()}
          </span>
          {handleEdit && (
            <button
              onClick={() => {
                const newContent = prompt('Edit message:', message.content)
                if (newContent && newContent !== message.content) {
                  handleEdit(message.id, newContent)
                }
              }}
              className="text-xs text-gray-500 hover:text-gray-700"
              aria-label="Edit message"
            >
              Edit
            </button>
          )}
        </div>
      </div>
      <div className="flex-shrink-0 w-8 h-8 rounded-full bg-gray-400 flex items-center justify-center text-white">
        U
      </div>
    </div>
  )
}

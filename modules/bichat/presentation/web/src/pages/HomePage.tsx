import { useNavigate } from '@solidjs/router'
import { Show, type JSX } from 'solid-js'
import { useSessionEvents } from '../contexts/SessionEventContext'
import { useAppContext } from '../context/appContext'
import { ChatSession, createRateLimiter, useChatSession } from '../chat/ChatSession'
import { ChatHeader } from '../chat/components/ChatHeader'
import { WelcomeContent } from '../chat/components/WelcomeContent'
import { MessageList } from '../chat/components/MessageList'
import { MessageInput } from '../chat/components/MessageInput'
import { useAppToast } from '../ui/toast'
import { useI18n } from '../i18n/i18n'

function HomeContent(): JSX.Element {
  const chat = useChatSession()
  const i18n = useI18n()

  return (
    <div class="flex min-h-0 flex-1 flex-col">
      <WelcomeContent
        visible={() => chat.turns().length === 0 && !chat.streaming()}
        onPick={(prompt) => void chat.send(prompt)}
      />
      <MessageList turns={chat.turns} loading={chat.loading} streaming={chat.streaming} onRetry={() => void chat.retry()} />
      <MessageInput
        streaming={chat.streaming}
        onSend={(content) => void chat.send(content)}
        onStop={() => void chat.cancel()}
      />
      <span class="hidden">{i18n.t('chat.new')}</span>
    </div>
  )
}

export function HomePage(): JSX.Element {
  const ctx = useAppContext()
  const i18n = useI18n()
  const navigate = useNavigate()
  const events = useSessionEvents()
  const toast = useAppToast()
  const rateLimiter = createRateLimiter(20, 60_000)

  const onSessionCreated = (sessionId: string): void => {
    events.notifySessionCreated(sessionId)
    toast.success(i18n.t('toast.sessionCreated'))
    void navigate(`/session/${sessionId}`)
  }

  const llmConfigured = () => ctx.extensions.llm?.apiKeyConfigured !== false

  return (
    <div class="flex h-full min-h-0 flex-col">
      <ChatHeader title={i18n.t('nav.newChat')} />
      <Show
        when={llmConfigured()}
        fallback={
          <div class="flex flex-1 items-center justify-center p-6">
            <p class="max-w-md text-center text-sm text-neutral-500">{i18n.t('llm.notConfigured')}</p>
          </div>
        }
      >
        <ChatSession sessionId="new" rateLimiter={rateLimiter} onSessionCreated={onSessionCreated}>
          <HomeContent />
        </ChatSession>
      </Show>
    </div>
  )
}

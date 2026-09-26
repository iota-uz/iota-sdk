import { useParams } from '@solidjs/router'
import { Show, type JSX } from 'solid-js'
import { ChatSession, useChatSession } from '../chat/ChatSession'
import { ChatHeader } from '../chat/components/ChatHeader'
import { MessageList } from '../chat/components/MessageList'
import { MessageInput } from '../chat/components/MessageInput'
import { HITLForm } from '../chat/components/HITLForm'
import { ArtifactsPanel } from '../chat/components/ArtifactsPanel'
import { SessionSkeleton } from '../components/SessionSkeleton'
import { useI18n } from '../i18n/i18n'

const ARTIFACTS_KEY = 'bichat.web.artifacts-panel.expanded'

function ChatContent(props: { readOnly: boolean }): JSX.Element {
  const chat = useChatSession()
  const i18n = useI18n()

  const artifactsExpanded = () => window.localStorage.getItem(ARTIFACTS_KEY) !== 'false'
  const toggleArtifacts = (): void => {
    const next = !artifactsExpanded()
    window.localStorage.setItem(ARTIFACTS_KEY, String(next))
    window.dispatchEvent(new CustomEvent('bichat:artifacts-panel-expanded', { detail: { expanded: next } }))
  }

  return (
    <Show when={!chat.loading()} fallback={<SessionSkeleton />}>
      <div class="flex min-h-0 flex-1">
        <div class="flex min-h-0 min-w-0 flex-1 flex-col">
          <Show when={props.readOnly}>
            <div class="bg-amber-50 px-4 py-2 text-center text-xs text-amber-700">
              {i18n.t('chat.readonly')}
            </div>
          </Show>
          <MessageList
            turns={chat.turns}
            streaming={chat.streaming}
            error={chat.error}
            onRetry={() => void chat.retry()}
          />
          <Show when={chat.pendingQuestion()}>{(question) => (
            <HITLForm
              question={question()}
              onSubmit={(answers) => void chat.submitAnswers(question().checkpointId, answers)}
              onReject={() => void chat.rejectQuestion()}
            />
          )}</Show>
          <Show when={!props.readOnly}>
            <MessageInput
              streaming={chat.streaming}
              onSend={(content) => void chat.send(content)}
              onStop={() => void chat.cancel()}
            />
          </Show>
        </div>
        <ArtifactsPanel
          artifacts={chat.artifacts}
          expanded={artifactsExpanded}
          onToggle={toggleArtifacts}
        />
      </div>
    </Show>
  )
}

export function ChatPage(): JSX.Element {
  const params = useParams()
  const i18n = useI18n()
  const readOnly = () => new URLSearchParams(window.location.search).get('readonly') === 'true'

  return (
    <div class="flex h-full min-h-0 flex-col">
      <ChatHeader title={i18n.t('nav.chats')} />
      <ChatSession sessionId={params.id} readOnly={readOnly()}>
        <ChatContent readOnly={readOnly()} />
      </ChatSession>
    </div>
  )
}

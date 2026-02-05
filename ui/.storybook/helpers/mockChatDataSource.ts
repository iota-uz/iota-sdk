import type {
  ChatDataSource,
  Session,
  ConversationTurn,
  PendingQuestion,
  StreamChunk,
  QuestionAnswers,
} from '../../src/bichat/types'
import { makeSession } from './bichatFixtures'

export class MockChatDataSource implements ChatDataSource {
  constructor(
    private options: {
      session?: Session
      turns?: ConversationTurn[]
      pendingQuestion?: PendingQuestion | null
      streamingDelay?: number
    } = {}
  ) {}

  async createSession(): Promise<Session> {
    return this.options.session ?? makeSession()
  }

  async fetchSession(id: string): Promise<{
    session: Session
    turns: ConversationTurn[]
    pendingQuestion?: PendingQuestion | null
  } | null> {
    return {
      session: this.options.session ?? makeSession({ id }),
      turns: this.options.turns ?? [],
      pendingQuestion: this.options.pendingQuestion,
    }
  }

  async *sendMessage(
    _sessionId: string,
    content: string
  ): AsyncGenerator<StreamChunk> {
    // 1. Signal user message accepted
    yield { type: 'user_message', sessionId: _sessionId }

    // 2. Simulate streaming chunks
    const response = `You said: "${content}". This is a mock response from Storybook.`
    const words = response.split(' ')

    for (const word of words) {
      if (this.options.streamingDelay) {
        await new Promise((resolve) => setTimeout(resolve, this.options.streamingDelay))
      }
      yield { type: 'chunk', content: word + ' ' }
    }

    // 3. Signal done
    yield { type: 'done', sessionId: _sessionId }
  }

  async submitQuestionAnswers(
    _sessionId: string,
    _questionId: string,
    _answers: QuestionAnswers
  ): Promise<{ success: boolean; error?: string }> {
    console.log('Mock submit answers:', _answers)
    return { success: true }
  }

  async cancelPendingQuestion(_questionId: string): Promise<{ success: boolean; error?: string }> {
    console.log('Mock cancel question:', _questionId)
    return { success: true }
  }

  navigateToSession(sessionId: string): void {
    console.log('Mock navigate to session:', sessionId)
  }
}

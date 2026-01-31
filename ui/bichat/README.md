# @iota-uz/bichat-ui

React UI components for BI-Chat - A production-ready, customizable chat interface for AI-powered business intelligence applications.

## Features

- **Modern React** - Hooks, functional components, TypeScript strict mode
- **Streaming Support** - Real-time message streaming with AsyncGenerator
- **HITL (Human-in-the-Loop)** - Built-in support for clarifying questions
- **Rich Content** - Markdown, code highlighting, charts, citations, file downloads
- **Customizable** - Render props and CSS variables for theming
- **Accessible** - ARIA labels, keyboard navigation
- **Lightweight** - No heavy UI frameworks, minimal dependencies

## Installation

```bash
npm install @iota-uz/bichat-ui react react-dom
# or
yarn add @iota-uz/bichat-ui react react-dom
# or
pnpm add @iota-uz/bichat-ui react react-dom
```

## Basic Usage

```tsx
import { ChatSession } from '@iota-uz/bichat-ui'
import '@iota-uz/bichat-ui/styles.css'

function App() {
  const dataSource = {
    async createSession() {
      const response = await fetch('/api/sessions', { method: 'POST' })
      return response.json()
    },
    async fetchSession(id: string) {
      const response = await fetch(`/api/sessions/${id}`)
      return response.json()
    },
    async *sendMessage(sessionId: string, content: string, attachments?: any[]) {
      const response = await fetch(`/api/sessions/${sessionId}/messages`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ content, attachments }),
      })

      const reader = response.body!.getReader()
      const decoder = new TextDecoder()

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        const text = decoder.decode(value)
        const lines = text.split('\n').filter(Boolean)

        for (const line of lines) {
          if (line.startsWith('data: ')) {
            const chunk = JSON.parse(line.slice(6))
            yield chunk
          }
        }
      }
    },
    async submitQuestionAnswers(sessionId: string, questionId: string, answers: any) {
      const response = await fetch(`/api/sessions/${sessionId}/questions/${questionId}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ answers }),
      })
      return response.json()
    },
    async cancelPendingQuestion(questionId: string) {
      const response = await fetch(`/api/questions/${questionId}`, { method: 'DELETE' })
      return response.json()
    },
  }

  return <ChatSession dataSource={dataSource} sessionId="new" />
}
```

## Customization

### Custom Message Rendering

```tsx
import { ChatSession, Message } from '@iota-uz/bichat-ui'

function App() {
  return (
    <ChatSession
      dataSource={dataSource}
      sessionId="new"
      renderUserMessage={(message) => (
        <div className="my-custom-user-message">
          <strong>{message.content}</strong>
        </div>
      )}
      renderAssistantMessage={(message) => (
        <div className="my-custom-assistant-message">
          <em>{message.content}</em>
        </div>
      )}
    />
  )
}
```

### Theming with CSS Variables

```css
:root {
  --bichat-primary: #3b82f6;        /* Primary color */
  --bichat-bg: #ffffff;             /* Background */
  --bichat-text: #1f2937;           /* Text color */
  --bichat-border: #e5e7eb;         /* Border color */
  --bichat-bubble-user: #3b82f6;    /* User message bubble */
  --bichat-bubble-assistant: #ffffff; /* AI message bubble */
}

/* Dark mode */
@media (prefers-color-scheme: dark) {
  :root {
    --bichat-primary: #60a5fa;
    --bichat-bg: #1f2937;
    --bichat-text: #f9fafb;
    --bichat-border: #374151;
    --bichat-bubble-assistant: #374151;
  }
}
```

### Using Context Directly

```tsx
import { ChatSessionProvider, useChat } from '@iota-uz/bichat-ui'

function CustomChatUI() {
  const {
    messages,
    message,
    setMessage,
    handleSubmit,
    loading,
    error,
  } = useChat()

  return (
    <div>
      {messages.map((msg) => (
        <div key={msg.id}>{msg.content}</div>
      ))}
      <form onSubmit={handleSubmit}>
        <input
          value={message}
          onChange={(e) => setMessage(e.target.value)}
        />
        <button disabled={loading}>Send</button>
      </form>
      {error && <div>{error}</div>}
    </div>
  )
}

function App() {
  return (
    <ChatSessionProvider dataSource={dataSource} sessionId="new">
      <CustomChatUI />
    </ChatSessionProvider>
  )
}
```

## DataSource Interface

The `ChatDataSource` interface defines how the UI communicates with your backend:

```typescript
interface ChatDataSource {
  // Create a new chat session
  createSession(): Promise<ChatSession>

  // Fetch session state (messages, pending questions)
  fetchSession(id: string): Promise<{
    session: ChatSession
    messages: Message[]
    pendingQuestion?: PendingQuestion | null
  } | null>

  // Send a message and stream the response
  sendMessage(
    sessionId: string,
    content: string,
    attachments?: Attachment[]
  ): AsyncGenerator<StreamChunk>

  // Submit answers to HITL questions
  submitQuestionAnswers(
    sessionId: string,
    questionId: string,
    answers: QuestionAnswers
  ): Promise<{ success: boolean; error?: string }>

  // Cancel pending question
  cancelPendingQuestion(questionId: string): Promise<{ success: boolean; error?: string }>

  // Optional: Navigate to a session (for SPA routing)
  navigateToSession?(sessionId: string): void
}
```

## Type Exports

All TypeScript types are exported for use in your application:

```typescript
import type {
  ChatSessionType,
  Message,
  MessageRole,
  Citation,
  ChartData,
  Artifact,
  PendingQuestion,
  ChatDataSource,
} from '@iota-uz/bichat-ui'
```

## Streaming Protocol

The `sendMessage` method should yield `StreamChunk` objects:

```typescript
interface StreamChunk {
  type: 'chunk' | 'error' | 'done' | 'user_message'
  content?: string        // For type: 'chunk'
  error?: string          // For type: 'error'
  sessionId?: string      // For type: 'user_message' | 'done'
}
```

### Example SSE Implementation

```typescript
async *sendMessage(sessionId: string, content: string) {
  const response = await fetch(`/api/sessions/${sessionId}/stream`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ content }),
  })

  const reader = response.body!.getReader()
  const decoder = new TextDecoder()

  while (true) {
    const { done, value } = await reader.read()
    if (done) break

    const text = decoder.decode(value)

    // Parse SSE format: "data: {...}\n\n"
    for (const line of text.split('\n')) {
      if (line.startsWith('data: ')) {
        yield JSON.parse(line.slice(6))
      }
    }
  }
}
```

## Components

### ChatSession

Main component that composes header, message list, and input.

**Props:**
- `dataSource: ChatDataSource` - Required data source implementation
- `sessionId?: string` - Session ID or 'new' for new session
- `isReadOnly?: boolean` - Disable input when true
- `renderUserMessage?: (message: Message) => ReactNode` - Custom user message renderer
- `renderAssistantMessage?: (message: Message) => ReactNode` - Custom assistant message renderer
- `className?: string` - Additional CSS classes

### Individual Components

All sub-components can be used standalone:

- `ChatHeader` - Session title and controls
- `MessageList` - Scrollable message container
- `MessageInput` - Input field with auto-resize
- `MarkdownRenderer` - Markdown with syntax highlighting
- `ChartCard` - Chart visualization (recharts)
- `SourcesPanel` - Citation display
- `DownloadCard` - File download UI
- `InlineQuestionForm` - HITL question form

## Hooks

### useChat

Access chat context from any child component:

```typescript
const {
  messages,           // All messages
  message,            // Current input value
  setMessage,         // Update input
  handleSubmit,       // Submit form
  loading,            // Is AI responding?
  error,              // Error message
  streamingContent,   // Current streaming text
  isStreaming,        // Is streaming active?
  pendingQuestion,    // HITL question
  handleCopy,         // Copy text to clipboard
  handleRegenerate,   // Regenerate last response
  handleEdit,         // Edit and resend message
} = useChat()
```

### useStreaming

Handle AsyncGenerator streams:

```typescript
const { content, isStreaming, error, processStream, reset } = useStreaming({
  onChunk: (content) => console.log('New content:', content),
  onError: (error) => console.error('Stream error:', error),
  onDone: () => console.log('Stream complete'),
})

// Start streaming
await processStream(dataSource.sendMessage('session-id', 'Hello'))

// Reset state
reset()
```

## Features in Detail

### Charts

Charts are automatically rendered when a message includes `chartData`:

```typescript
{
  chartData: {
    type: 'bar',  // 'bar' | 'line' | 'pie' | 'area'
    title: 'Revenue by Month',
    data: [
      { name: 'Jan', value: 4000 },
      { name: 'Feb', value: 3000 },
    ],
    xAxisKey: 'name',
    yAxisKey: 'value',
  }
}
```

### Citations

Citations are displayed in a collapsible sources panel:

```typescript
{
  citations: [
    {
      id: '1',
      source: 'Q4 Financial Report',
      url: 'https://example.com/report.pdf',
      excerpt: 'Revenue increased by 25%...',
    }
  ]
}
```

### File Downloads

Artifacts (Excel, PDF) are displayed as download cards:

```typescript
{
  artifacts: [
    {
      type: 'excel',
      filename: 'sales_report.xlsx',
      url: '/downloads/abc123',
      sizeReadable: '2.4 MB',
      rowCount: 1500,
      description: 'Sales data for Q4 2023',
    }
  ]
}
```

### HITL (Human-in-the-Loop)

When the AI needs clarification, a `PendingQuestion` is displayed:

```typescript
{
  pendingQuestion: {
    id: 'q1',
    turnId: 'turn-123',
    question: 'Which quarter would you like to analyze?',
    type: 'MULTIPLE_CHOICE',
    options: ['Q1 2023', 'Q2 2023', 'Q3 2023', 'Q4 2023'],
    status: 'PENDING',
  }
}
```

## Browser Support

- Chrome/Edge: Latest 2 versions
- Firefox: Latest 2 versions
- Safari: Latest 2 versions

## License

MIT

## Contributing

Contributions welcome! Please read our contributing guidelines first.

## Support

For issues and questions, please open a GitHub issue.

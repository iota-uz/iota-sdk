# Phase 8: React UI Components Implementation

Implement Phase 8 of the BI-Chat foundation: React UI Components in ui/bichat/

Create a complete, production-ready React component library for BI-Chat with the following structure:

## Configuration Files

1. **ui/bichat/package.json**:
   - Name: @iota-uz/bichat-ui
   - React 18.3.1 + TypeScript 5.5+ + Vite 5.4+
   - Dependencies: react, react-dom, recharts, react-markdown, react-syntax-highlighter, remark-gfm, rehype-sanitize
   - Dev dependencies: @types/react, @types/react-dom, @vitejs/plugin-react, typescript, vite
   - Scripts: dev, build, typecheck, lint
   - Type: module
   - Main: dist/index.js
   - Types: dist/index.d.ts

2. **ui/bichat/tsconfig.json**:
   - Strict mode enabled
   - JSX: react-jsx
   - Target: ES2020
   - Module: ESNext
   - Declaration: true
   - Exclude node_modules

3. **ui/bichat/vite.config.ts**:
   - Library mode (name: BIChat)
   - Entry: src/index.ts
   - External: react, react-dom
   - Output formats: es, umd
   - Rollup externals for peer dependencies

## Type Definitions (ui/bichat/src/types/index.ts)

```typescript
export interface ChatSession {
  id: string
  title: string
  status: 'active' | 'archived'
  pinned: boolean
  createdAt: string
  updatedAt: string
}

export enum MessageRole {
  User = 'user',
  Assistant = 'assistant',
  System = 'system',
  Tool = 'tool',
}

export interface Message {
  id: string
  sessionId: string
  role: MessageRole
  content: string
  createdAt: string
  toolCalls?: ToolCall[]
  citations?: Citation[]
  chartData?: ChartData
  artifacts?: Artifact[]
  explanation?: string
}

export interface ToolCall {
  id: string
  name: string
  arguments: string
}

export interface Citation {
  id: string
  source: string
  url?: string
  excerpt?: string
}

export interface Attachment {
  id: string
  filename: string
  mimeType: string
  sizeBytes: number
  base64Data?: string
}

export interface ChartData {
  type: 'bar' | 'line' | 'pie' | 'area'
  title?: string
  data: any[]
  xAxisKey?: string
  yAxisKey?: string
}

export interface Artifact {
  type: 'excel' | 'pdf'
  filename: string
  url: string
  sizeReadable?: string
  rowCount?: number
  description?: string
}

export interface PendingQuestion {
  id: string
  turnId: string
  question: string
  type: 'MULTIPLE_CHOICE' | 'FREE_TEXT'
  options?: string[]
  status: 'PENDING' | 'ANSWERED' | 'CANCELLED'
}

export type QuestionAnswers = Record<string, string>

export interface StreamChunk {
  type: 'chunk' | 'error' | 'done' | 'user_message'
  content?: string
  error?: string
  sessionId?: string
}

export interface ChatDataSource {
  createSession(): Promise<ChatSession>
  fetchSession(id: string): Promise<{
    session: ChatSession
    messages: Message[]
    pendingQuestion?: PendingQuestion | null
  } | null>
  sendMessage(
    sessionId: string,
    content: string,
    attachments?: Attachment[]
  ): AsyncGenerator<StreamChunk>
  submitQuestionAnswers(
    sessionId: string,
    questionId: string,
    answers: QuestionAnswers
  ): Promise<{ success: boolean; error?: string }>
  cancelPendingQuestion(questionId: string): Promise<{ success: boolean; error?: string }>
  navigateToSession?(sessionId: string): void
}
```

## Context Provider (ui/bichat/src/context/ChatContext.tsx)

Implement ChatSessionProvider with:
- State management for messages, loading, streaming
- Message queue support (max 5)
- Auto-submit from location state
- Rate limiting (20 msg/min)
- Optimistic UI updates
- Error handling
- HITL question handling
- Reference: /Users/diyorkhaydarov/Projects/sdk/shy-trucks/core/modules/shyona/presentation/web/src/providers/ChatSessionProvider.tsx

Export useChat hook for consuming context.

## Core Components

4. **ui/bichat/src/components/ChatSession.tsx**:
   - Main container component
   - Props: dataSource, sessionId, isReadOnly
   - Composes ChatHeader, MessageList, MessageInput
   - Render props for customization

5. **ui/bichat/src/components/ChatHeader.tsx**:
   - Session title display
   - Archive/pin controls
   - Back navigation

6. **ui/bichat/src/components/MessageList.tsx**:
   - Scrollable container
   - Auto-scroll to bottom on new messages
   - Group by date
   - Maps messages to TurnBubble

7. **ui/bichat/src/components/TurnBubble.tsx**:
   - Message container with role-based styling
   - Delegates to UserTurnView or AssistantTurnView
   - Hover actions

8. **ui/bichat/src/components/UserTurnView.tsx**:
   - User message bubble
   - Attachment display
   - Edit action

9. **ui/bichat/src/components/AssistantTurnView.tsx**:
   - Assistant message with markdown
   - Chart integration
   - Sources panel
   - Download cards
   - Explanation collapsible
   - Reference: /Users/diyorkhaydarov/Projects/sdk/shy-trucks/core/modules/shyona/presentation/web/src/components/ChatSession/AssistantTurnView.tsx

10. **ui/bichat/src/components/MarkdownRenderer.tsx**:
    - react-markdown with remark-gfm
    - Code highlighting (react-syntax-highlighter)
    - Citation inline display
    - Sanitize HTML (rehype-sanitize)

11. **ui/bichat/src/components/ChartCard.tsx**:
    - Recharts integration
    - Support bar, line, pie, area
    - Responsive sizing

12. **ui/bichat/src/components/SourcesPanel.tsx**:
    - Citation list
    - Expandable excerpts
    - Link handling

13. **ui/bichat/src/components/DownloadCard.tsx**:
    - Excel/PDF download UI
    - File size display
    - Row count badge

14. **ui/bichat/src/components/InlineQuestionForm.tsx**:
    - HITL question rendering
    - Multiple choice radio buttons
    - Free text input
    - Submit/cancel actions

15. **ui/bichat/src/components/MessageInput.tsx**:
    - Auto-resize textarea
    - Send button
    - File attachment support (optional)
    - Keyboard shortcuts (Enter to send, Shift+Enter for newline)
    - Queue indicator when loading

## Hooks

16. **ui/bichat/src/hooks/useStreaming.ts**:
    - Handle AsyncGenerator streaming
    - Accumulate chunks
    - Error handling
    - Cleanup on unmount

## Main Export (ui/bichat/src/index.ts)

Export all components, hooks, types, and context.

## Styling

Use CSS variables for theming:
```css
:root {
  --bichat-primary: #3b82f6;
  --bichat-bg: #ffffff;
  --bichat-text: #1f2937;
  --bichat-border: #e5e7eb;
  --bichat-bubble-user: #3b82f6;
  --bichat-bubble-assistant: #ffffff;
}
```

Include basic styles in ui/bichat/src/styles.css

## README.md (ui/bichat/README.md)

Create comprehensive documentation:
- Installation
- Basic usage example
- Customization via render props
- Theming guide
- DataSource implementation example
- TypeScript type exports

## Requirements:
- Modern React patterns (hooks, functional components)
- TypeScript strict mode
- Accessibility (ARIA labels, keyboard nav)
- Responsive design
- Clean, well-documented code
- No external UI libraries (headless only)
- Lightweight bundle size

Reference existing implementation at:
- /Users/diyorkhaydarov/Projects/sdk/shy-trucks/core/modules/shyona/presentation/web/src/

Mark task as complete when all files are created and tested.

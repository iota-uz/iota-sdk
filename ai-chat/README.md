# IOTA SDK AI Chat Component

A customizable React chatbot component that integrates with IOTA SDK's backend for AI-powered conversations.

## Features

- Multi-language support (EN, RU, UZ)
- Customizable chatbot interface
- Quick reply suggestions
- Callback request functionality
- Session persistence
- Mobile-responsive design
- Typing indicators
- Error handling

## Installation

```bash
# Navigate to the ai-chat directory
cd ai-chat

# Install dependencies
npm install
# or
yarn install
# or
pnpm install
```

## Quick Start

1. Set up your IOTA SDK backend endpoint in the `apiEndpoint` prop

2. Import and use the component:

```jsx
import ChatbotInterface from '@/components/chatbot-interface';

function App() {
  return (
    <ChatbotInterface
      locale="en"
      apiEndpoint="https://your-iota-sdk-server.com/website/ai-chat"
      title="Custom Title"
      subtitle="Custom Subtitle"
    />
  );
}
```

## API Reference

### `ChatbotInterface` Component

The main component that renders the chatbot UI.

#### Props

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `locale` | `string` | `"ru"` | Language code for translations ("en", "ru", "uz", "uzCyrl") |
| `apiEndpoint` | `string` | *Required* | Direct URL to your IOTA SDK chat backend |
| `faqItems` | `FAQItem[]` | `undefined` | Custom FAQ items for quick replies |
| `title` | `string` | `undefined` | Custom chatbot title (falls back to translation) |
| `subtitle` | `string` | `undefined` | Custom chatbot subtitle (falls back to translation) |

### Types

```typescript
// Quick reply item
export interface FAQItem {
  id: string;
  question: string;
}

// Message in chat
export type ChatMessage = {
  id: string;
  content: string;
  sender: "user" | "bot";
  timestamp: Date;
};
```

## Backend Integration

This component requires an IOTA SDK backend API with the following endpoints:

### Create Thread

```
POST /messages
```

Request body:
```json
{
  "message": "Initial message",
  "phone": "Phone number"
}
```

Response:
```json
{
  "thread_id": "unique-thread-id"
}
```

### Get Messages

```
GET /messages/{threadId}
```

Response:
```json
{
  "messages": [
    {
      "role": "user",
      "message": "User message content"
    },
    {
      "role": "assistant",
      "message": "Assistant response content"
    }
  ]
}
```

### Add Message

```
POST /messages/{threadId}
```

Request body:
```json
{
  "message": "New message content"
}
```

## Development

```bash
# Run the development server
npm run dev
# or
yarn dev
# or
pnpm dev
```

Visit [http://localhost:3000](http://localhost:3000) to see the demo.

## Customization

### Styling

The component uses Tailwind CSS for styling. You can customize the appearance by modifying the class names in the component files.

### Translations

Translations are stored in `lib/translations.ts`. You can add additional languages or modify existing translations as needed.

## CORS Considerations

Since the component now makes direct API calls to your IOTA SDK backend, you need to ensure that your backend server has CORS properly configured to allow requests from the domains where this component will be used.

## License

See the LICENSE file in the root directory of this project.
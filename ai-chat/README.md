# @iotauz/ai-chat

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
# Install with npm
npm install @iotauz/ai-chat

# Or with yarn
yarn add @iotauz/ai-chat

# Or with pnpm
pnpm add @iotauz/ai-chat
```

## Quick Start

1. Set up your IOTA SDK backend endpoint in the `apiEndpoint` prop

2. Import the component and CSS:

```jsx
import { ChatbotInterface } from '@iotauz/ai-chat';
// Import the pre-compiled CSS styles
import '@iotauz/ai-chat/dist/styles.css';
import { MessagesSquare } from 'lucide-react'; // or any other icon library

function App() {
  return (
    <ChatbotInterface
      locale="en"
      apiEndpoint="https://your-iota-sdk-server.com/website/ai-chat"
      title="Custom Title"
      subtitle="Custom Subtitle"
      chatIcon={<MessagesSquare size={24} className="text-white" />}
      soundOptions={{
        // Optional: customize sound effects
        submitSoundPath: "/sounds/custom-submit.mp3",
        operatorSoundPath: "/sounds/custom-operator.mp3",
        volume: 0.4,
        enabled: true
      }}
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
| `locale` | `string` | `"ru"` | Language code for translations ("en", "ru", "oz", "uz") |
| `apiEndpoint` | `string` | *Required* | Direct URL to your IOTA SDK chat backend |
| `faqItems` | `FAQItem[]` | `undefined` | Custom FAQ items for quick replies |
| `title` | `string` | `undefined` | Custom chatbot title (falls back to translation) |
| `subtitle` | `string` | `undefined` | Custom chatbot subtitle (falls back to translation) |
| `chatIcon` | `React.ReactNode` | `undefined` | Custom icon for the chat button and header |
| `soundOptions` | `SoundOptions` | `undefined` | Customize sound effects (see below) |

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

// Sound options configuration
export interface SoundOptions {
  enabled?: boolean;        // Enable/disable sound effects (default: true)
  volume?: number;          // Sound volume from 0.0 to 1.0 (default: 0.5)
  submitSoundPath?: string; // Path to submit sound file (default: '/sounds/submit.mp3')
  operatorSoundPath?: string; // Path to operator sound file (default: '/sounds/operator.mp3')
}
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

The component uses Tailwind CSS for styling. The package now includes a pre-compiled CSS file that you must import in your application:

```js
// Import the pre-compiled CSS styles
import '@iotauz/ai-chat/dist/styles.css';
```

This CSS file contains all the necessary styles for the component, including Tailwind utilities used by the chatbot interface. 

For custom styling, you can override these styles in your application's CSS files.

### Translations

Translations are stored in `lib/translations.ts`. You can add additional languages or modify existing translations as needed.

### Sound Effects

The chatbot includes sound effects for message submission and operator responses. You can:

1. Customize the sound file paths:
```jsx
<ChatbotInterface 
  apiEndpoint="/api/chat"
  soundOptions={{
    submitSoundPath: "/my-custom-sounds/submit.mp3",
    operatorSoundPath: "/my-custom-sounds/operator.mp3",
    volume: 0.5
  }}
/>
```

2. Disable sounds:
```jsx
<ChatbotInterface 
  apiEndpoint="/api/chat"
  soundOptions={{ enabled: false }}
/>
```

Make sure to place your custom sound files in the public directory of your Next.js application.

## CORS Considerations

Since the component now makes direct API calls to your IOTA SDK backend, you need to ensure that your backend server has CORS properly configured to allow requests from the domains where this component will be used.

## License

See the LICENSE file in the root directory of this project.

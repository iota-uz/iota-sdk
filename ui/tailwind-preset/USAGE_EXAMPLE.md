# Tailwind Preset Usage Example

This document demonstrates how applets should consume the `@iota-uz/tailwind-preset` package.

## Installation in an Applet

```bash
# From npm registry (when published)
npm install @iota-uz/tailwind-preset

# Or from local package for development
npm install file:../tailwind-preset
```

## Example: BiChat Web Applet

Assuming you have a React applet at `ui/bichat-web/`:

### 1. Create `tailwind.config.js`

```js
const iotaPreset = require('@iota-uz/tailwind-preset')

module.exports = {
  presets: [iotaPreset],
  content: [
    './src/**/*.{ts,tsx,js,jsx}',
    './public/index.html',
  ],
  // Applet-specific overrides (optional)
  theme: {
    extend: {
      // Custom colors specific to BiChat
      colors: {
        chat: {
          user: '#e0f2fe',
          assistant: '#f3f4f6',
        },
      },
    },
  },
}
```

### 2. Create `src/input.css`

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

/* CSS Variables - typically provided by IOTA SDK global styles */
:root {
  /* Primary brand colors */
  --color-primary: #3b82f6;
  --color-primary-50: #eff6ff;
  --color-primary-100: #dbeafe;
  --color-primary-200: #bfdbfe;
  --color-primary-300: #93c5fd;
  --color-primary-400: #60a5fa;
  --color-primary-500: #3b82f6;
  --color-primary-600: #2563eb;
  --color-primary-700: #1d4ed8;
  --color-primary-800: #1e40af;
  --color-primary-900: #1e3a8a;

  /* Background and foreground */
  --color-background: #ffffff;
  --color-foreground: #000000;

  /* Text colors */
  --color-text: #1f2937;
  --color-text-muted: #6b7280;
  --color-text-inverse: #ffffff;

  /* Semantic colors */
  --color-success: #10b981;
  --color-success-light: #d1fae5;
  --color-success-dark: #047857;

  --color-error: #ef4444;
  --color-error-light: #fee2e2;
  --color-error-dark: #b91c1c;

  --color-warning: #f59e0b;
  --color-warning-light: #fef3c7;
  --color-warning-dark: #d97706;

  --color-info: #3b82f6;
  --color-info-light: #dbeafe;
  --color-info-dark: #1d4ed8;

  /* Border colors */
  --color-border: #e5e7eb;
  --color-border-light: #f3f4f6;
  --color-border-dark: #d1d5db;

  /* Surface colors */
  --color-surface: #ffffff;
  --color-surface-elevated: #f9fafb;
  --color-surface-overlay: rgba(0, 0, 0, 0.5);
}

/* Dark mode */
[data-theme="dark"] {
  --color-background: #111827;
  --color-foreground: #ffffff;
  --color-text: #f9fafb;
  --color-text-muted: #9ca3af;
  --color-border: #374151;
  --color-surface: #1f2937;
  --color-surface-elevated: #374151;
}

/* Custom applet styles */
@layer components {
  .chat-message {
    @apply rounded-lg p-4 mb-3;
  }

  .chat-message-user {
    @apply bg-chat-user text-text;
  }

  .chat-message-assistant {
    @apply bg-chat-assistant text-text;
  }
}
```

### 3. Add Build Script to `package.json`

```json
{
  "name": "bichat-web-applet",
  "scripts": {
    "build": "vite build",
    "build:css": "tailwindcss -i ./src/input.css -o ./dist/styles.css --minify",
    "dev:css": "tailwindcss -i ./src/input.css -o ./dist/styles.css --watch"
  },
  "devDependencies": {
    "@iota-uz/tailwind-preset": "^1.0.0",
    "tailwindcss": "^3.4.0"
  }
}
```

### 4. Use Tailwind Classes in React Components

```tsx
import React from 'react'
import './styles.css' // Generated CSS

export function ChatMessage({ role, content }: { role: 'user' | 'assistant'; content: string }) {
  return (
    <div className={`chat-message ${role === 'user' ? 'chat-message-user' : 'chat-message-assistant'}`}>
      <p className="text-base font-sans">{content}</p>
    </div>
  )
}

export function ChatInput() {
  return (
    <div className="border-t border-border bg-surface p-4">
      <input
        type="text"
        className="form-input w-full rounded-md border-border focus:border-primary focus:ring-primary"
        placeholder="Type a message..."
      />
      <button className="mt-3 rounded-md bg-primary px-4 py-2 text-white font-medium hover:bg-primary-600 transition-colors">
        Send
      </button>
    </div>
  )
}

export function ErrorMessage({ message }: { message: string }) {
  return (
    <div className="rounded-md bg-error-light border border-error p-4">
      <p className="text-error-dark font-medium">{message}</p>
    </div>
  )
}
```

## Verification Checklist

After setting up the preset in your applet:

- [ ] `npm install` completes without errors
- [ ] `npm run build:css` generates CSS file
- [ ] CSS file includes Inter font family
- [ ] CSS file includes @tailwindcss/forms styles
- [ ] CSS file includes @tailwindcss/typography styles
- [ ] Tailwind classes work in components (e.g., `bg-primary`, `text-base`)
- [ ] CSS variables are used for colors (inspect compiled CSS)
- [ ] Dark mode works when `[data-theme="dark"]` is set
- [ ] Custom applet overrides work (e.g., `bg-chat-user`)

## Testing CSS Variables

Create a test component to verify theming works:

```tsx
export function ThemeTest() {
  const [theme, setTheme] = React.useState<'light' | 'dark'>('light')

  React.useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme)
  }, [theme])

  return (
    <div className="p-8 bg-background text-text">
      <button
        onClick={() => setTheme(theme === 'light' ? 'dark' : 'light')}
        className="rounded-md bg-primary px-4 py-2 text-white"
      >
        Toggle to {theme === 'light' ? 'Dark' : 'Light'} Mode
      </button>

      <div className="mt-4 space-y-3">
        <div className="rounded-md bg-surface border border-border p-4">
          <p className="text-text">Surface color adapts to theme</p>
        </div>

        <div className="rounded-md bg-success-light border border-success p-4">
          <p className="text-success-dark">Success message</p>
        </div>

        <div className="rounded-md bg-error-light border border-error p-4">
          <p className="text-error-dark">Error message</p>
        </div>
      </div>
    </div>
  )
}
```

## Publishing to NPM (When Ready)

```bash
# Login to NPM
npm login

# Publish package
cd ui/tailwind-preset
npm publish --access public

# Verify package is available
npm view @iota-uz/tailwind-preset
```

## Local Development

For local development without publishing:

```bash
# In tailwind-preset directory
npm pack

# In applet directory
npm install ../tailwind-preset/iota-uz-tailwind-preset-1.0.0.tgz

# Or use npm link
cd ui/tailwind-preset
npm link

cd ui/bichat-web
npm link @iota-uz/tailwind-preset
```

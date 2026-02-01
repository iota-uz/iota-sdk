# Example: Using @iotauz/design-tokens in Your Project

## Setup

### 1. Install the Package

```bash
npm install @iotauz/design-tokens tailwindcss@^4.0.0
```

### 2. Create Your CSS File

**app/globals.css** (or your main CSS file):

```css
/* Full import - recommended */
@import "@iotauz/design-tokens";

/* Define your content sources */
@source "./app/**/*.{js,ts,jsx,tsx}";
@source "./components/**/*.{js,ts,jsx,tsx}";
@source "./pages/**/*.{js,ts,jsx,tsx}";

/* Optional: Extend or override tokens */
@theme {
  /* Add custom colors */
  --color-accent: oklch(70% 0.15 120);
  --color-warning: oklch(75% 0.2 60);
  
  /* Override brand colors */
  --color-brand-500: oklch(60% 0.25 280);
}

/* Optional: Add project-specific styles */
.my-custom-component {
  background-color: oklch(var(--primary-500));
  padding: var(--size-3);
  border-radius: var(--size-1);
}
```

## Alternative: Partial Import

If you only need specific parts:

```css
/* Import only what you need */
@import "tailwindcss";
@import "@iotauz/design-tokens/theme.css";
@import "@iotauz/design-tokens/base.css";

/* Skip components and utilities if you don't need them */
/* @import "@iotauz/design-tokens/components.css"; */
/* @import "@iotauz/design-tokens/utilities.css"; */

@source "./app/**/*.{js,ts,jsx,tsx}";
```

## Usage Examples

### React Component Example

```jsx
// components/Button.jsx
export function Button({ children, variant = 'primary', size = 'md' }) {
  return (
    <button className={`btn btn-${variant} btn-${size}`}>
      {children}
    </button>
  );
}

// Usage
<Button variant="primary" size="md">Click me</Button>
<Button variant="secondary" size="sm">Cancel</Button>
<Button variant="danger">Delete</Button>
```

### Form Example

```jsx
// components/Form.jsx
export function Form() {
  return (
    <form className="space-y-4">
      <div className="form-control">
        <label className="form-control-label">Email</label>
        <input 
          type="email" 
          className="form-control-input" 
          placeholder="Enter your email"
        />
      </div>
      
      <div className="form-control">
        <label className="form-control-label">Password</label>
        <input 
          type="password" 
          className="form-control-input" 
          placeholder="Enter password"
        />
      </div>
      
      <button type="submit" className="btn btn-primary btn-md">
        Submit
      </button>
    </form>
  );
}
```

### Using Tailwind Utilities with Design Tokens

```jsx
// All color tokens are available as Tailwind utilities
export function Card() {
  return (
    <div className="bg-white dark:bg-black p-6 rounded-lg border border-gray-300">
      <h2 className="text-black dark:text-white text-xl font-semibold">
        Card Title
      </h2>
      <p className="text-gray-600 dark:text-gray-300 mt-2">
        This card uses design tokens as Tailwind utilities
      </p>
      <button className="btn btn-primary mt-4">
        Action
      </button>
    </div>
  );
}
```

### Dark Mode Support

```jsx
// app/layout.jsx or pages/_app.jsx
export default function RootLayout({ children }) {
  return (
    <html lang="en" className="dark"> {/* Add 'dark' class for dark mode */}
      <body>{children}</body>
    </html>
  );
}

// Or toggle dark mode dynamically
function ThemeToggle() {
  const toggleTheme = () => {
    document.documentElement.classList.toggle('dark');
  };
  
  return (
    <button onClick={toggleTheme} className="btn btn-secondary">
      Toggle Theme
    </button>
  );
}
```

### Using CSS Variables in Custom Components

```css
/* Custom component using design tokens */
.hero-section {
  background: linear-gradient(
    135deg,
    oklch(var(--primary-500)),
    oklch(var(--primary-700))
  );
  padding: var(--size-5);
  border-radius: var(--size-2);
  animation: var(--animation-slide-in-up);
}

.custom-card {
  background-color: oklch(var(--clr-surface-300));
  border: 1px solid oklch(var(--clr-border-primary));
  box-shadow: var(--shadow-100);
  transition: transform 200ms var(--ease-2);
}

.custom-card:hover {
  transform: scale(1.02);
  box-shadow: var(--shadow-200);
}
```

## Migration Guide

### From Local main.css to @iotauz/design-tokens

**Before:**
```css
/* main.css */
@import "tailwindcss";

@theme {
  --color-brand-500: oklch(58.73% 0.23 279.66);
  /* ... all other tokens ... */
}

/* ... all component styles ... */
```

**After:**
```css
/* globals.css */
@import "@iotauz/design-tokens";

@source "./app/**/*.{js,ts,jsx,tsx}";

/* Only add project-specific customizations */
@theme {
  --color-custom: oklch(50% 0.2 180);
}
```

## Benefits

1. **No File Copying**: Import directly from package
2. **Consistent Design**: Same tokens across all projects
3. **Easy Updates**: Update package version to get latest design tokens
4. **Selective Import**: Only import what you need
5. **Extendable**: Override or add custom tokens
6. **Type-Safe**: Use with TypeScript for autocomplete

## Upgrade Path

When @iotauz/design-tokens releases a new version:

```bash
npm update @iotauz/design-tokens
```

If there are breaking changes, check the changelog and migration guide in the package repository.

## Local Development

For local development (before publishing):

```json
// package.json
{
  "dependencies": {
    "@iotauz/design-tokens": "file:../iota-sdk/packages/design-tokens"
  }
}
```

Or use npm link:

```bash
cd /path/to/iota-sdk/packages/design-tokens
npm link

cd /path/to/your-project
npm link @iotauz/design-tokens
```

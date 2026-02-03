# @iotauz/tailwind-preset

Shared Tailwind CSS preset for IOTA SDK applets. Provides unified design tokens, typography, and styling across all React/Next.js applets.

## Features

- **CSS Variable-based Colors**: Runtime theming support (dark mode, tenant branding)
- **Typography Scale**: Consistent font sizes and line heights
- **Design Tokens**: Semantic color system (primary, success, error, etc.)
- **Inter Font Family**: Professional sans-serif typeface
- **Official Plugins**: `@tailwindcss/forms` and `@tailwindcss/typography`

## Installation

```bash
pnpm install @iotauz/tailwind-preset
```

## Usage

### In Your Applet's `tailwind.config.js`

```js
const iotaPreset = require('@iotauz/tailwind-preset')

module.exports = {
  presets: [iotaPreset],
  content: ['./src/**/*.{ts,tsx}'],
  // Applet-specific overrides if needed
  theme: {
    extend: {
      // Custom colors, spacing, etc.
    }
  }
}
```

### CSS Variables Required

The preset uses CSS variables for theming. Ensure your global CSS defines these variables:

```css
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

/* Dark mode (optional) */
@media (prefers-color-scheme: dark) {
  :root {
    --color-background: #111827;
    --color-foreground: #ffffff;
    --color-text: #f9fafb;
    --color-text-muted: #9ca3af;
    /* ... override other variables for dark theme */
  }
}
```

## Design Tokens

### Colors

All colors use CSS variables for runtime theming:

```jsx
// Tailwind classes
<div className="bg-primary text-white">Primary button</div>
<div className="bg-success text-success-dark">Success message</div>
<div className="border border-border">Card with border</div>
```

### Typography

```jsx
// Font families
<p className="font-sans">Inter font</p>
<code className="font-mono">Monospace code</code>

// Font sizes with matching line heights
<h1 className="text-4xl font-bold">Heading</h1>
<p className="text-base">Body text</p>
<small className="text-sm text-text-muted">Muted text</small>

// Font weights
<span className="font-medium">Medium weight</span>
<span className="font-semibold">Semibold weight</span>
```

### Spacing

```jsx
// Extended spacing scale
<div className="mt-18">72px margin top</div>
<div className="p-88">352px padding</div>
```

### Shadows

```jsx
// Custom box shadows
<div className="shadow-soft">Subtle shadow</div>
<div className="shadow-medium">Medium shadow</div>
<div className="shadow-hard">Strong shadow</div>
```

### Z-Index Layers

```jsx
// Semantic z-index values
<div className="z-dropdown">Dropdown (1000)</div>
<div className="z-modal">Modal (1050)</div>
<div className="z-tooltip">Tooltip (1070)</div>
```

## Included Plugins

### @tailwindcss/forms

Provides better default styling for form elements:

```jsx
<input type="text" className="form-input rounded-md border-gray-300" />
<select className="form-select rounded-md border-gray-300">...</select>
<textarea className="form-textarea rounded-md border-gray-300">...</textarea>
```

### @tailwindcss/typography

Beautiful typography defaults for rich content:

```jsx
<article className="prose lg:prose-xl">
  <h1>Article Title</h1>
  <p>Article content with automatic styling...</p>
</article>
```

## Dark Mode Support

The preset supports dark mode via CSS variable swap. Override the CSS variables in a `[data-theme="dark"]` selector or `@media (prefers-color-scheme: dark)`:

```css
/* Option 1: Manual theme toggle */
[data-theme="dark"] {
  --color-background: #111827;
  --color-text: #f9fafb;
  /* ... */
}

/* Option 2: System preference */
@media (prefers-color-scheme: dark) {
  :root {
    --color-background: #111827;
    --color-text: #f9fafb;
    /* ... */
  }
}
```

Then toggle dark mode in your applet:

```jsx
// Set theme attribute on root element
document.documentElement.setAttribute('data-theme', 'dark')
```

## Tenant Branding

Override CSS variables at runtime for tenant-specific branding:

```jsx
// Set custom primary color for tenant
document.documentElement.style.setProperty('--color-primary', '#8b5cf6')
document.documentElement.style.setProperty('--color-primary-600', '#7c3aed')
```

## Applet-Specific Overrides

You can extend or override the preset in your applet's Tailwind config:

```js
const iotaPreset = require('@iotauz/tailwind-preset')

module.exports = {
  presets: [iotaPreset],
  content: ['./src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      // Add custom colors
      colors: {
        brand: '#ff6b6b',
      },
      // Add custom spacing
      spacing: {
        '200': '50rem',
      },
    },
  },
  // Add applet-specific plugins
  plugins: [
    // Your custom plugins
  ],
}
```

## Package Development

```bash
# Test package locally
pnpm pack

# Install in applet
cd ../bichat
pnpm install ../tailwind-preset/iota-uz-tailwind-preset-1.0.0.tgz
```

## License

MIT

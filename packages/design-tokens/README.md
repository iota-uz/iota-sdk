# @iotauz/design-tokens

Design tokens and Tailwind CSS v4 theme for IOTA SDK. This package provides a complete design system with OKLCH color palette, typography, components, and utilities that can be used in any project.

## Installation

```bash
npm install @iotauz/design-tokens tailwindcss@^4.0.0
```

## Usage

### Full Import (Recommended)

```css
/* your-app/app/globals.css */
@import "@iotauz/design-tokens";

/* Add your content sources */
@source "./app/**/*.{js,ts,jsx,tsx}";
@source "./components/**/*.{js,ts,jsx,tsx}";

/* Optionally extend or override tokens */
@theme {
  --color-custom: oklch(50% 0.2 180);
}
```

### Partial Import

```css
/* Import only what you need */
@import "tailwindcss";
@import "@iotauz/design-tokens/theme.css";
@import "@iotauz/design-tokens/base.css";

/* Skip components/utilities if you don't need them */
```

### Extend Theme

```css
@import "@iotauz/design-tokens";

@theme {
  /* Override brand colors */
  --color-brand-500: oklch(60% 0.25 280);
  
  /* Add new colors */
  --color-accent: oklch(70% 0.15 120);
}
```

## What's Included

- **Design Tokens**: OKLCH color palette, typography, spacing
- **Base Styles**: Font faces, CSS variables, dark mode
- **Components**: `.btn`, `.form-control`, `.dialog`, `.table`, etc.
- **Utilities**: Custom utility classes and keyframes
- **Theme**: Tailwind CSS v4 @theme configuration

## Color Palette

### Brand Colors
- `brand-400`, `brand-500`, `brand-600`, `brand-650`, `brand-700`

### Semantic Colors
- `gray-50` through `gray-950`
- `red-100`, `red-200`, `red-300`, `red-500`, `red-600`, `red-700`
- `green-50`, `green-100`, `green-200`, `green-500`, `green-600`
- `pink-500`, `pink-600`
- `yellow-500`
- `blue-500`, `blue-600`
- `purple-500`

### Usage

```jsx
<div className="bg-brand-500 text-white">
  <button className="btn btn-primary">Click me</button>
</div>
```

## Dark Mode

The package includes dark mode support via `html.dark` class:

```html
<html class="dark">
  <!-- Your app -->
</html>
```

## Custom Components

### Button

```html
<button class="btn btn-primary">Primary Button</button>
<button class="btn btn-secondary">Secondary Button</button>
<button class="btn btn-danger">Danger Button</button>
<button class="btn btn-primary-outline">Outline Button</button>
<button class="btn btn-sidebar">Sidebar Button</button>
```

#### Button Sizes
```html
<button class="btn btn-primary btn-md">Medium</button>
<button class="btn btn-primary btn-sm">Small</button>
<button class="btn btn-primary btn-xs">Extra Small</button>
```

#### Button Variants
```html
<button class="btn btn-primary btn-rounded">Rounded</button>
<button class="btn btn-primary btn-with-icon">
  <svg>...</svg>
  With Icon
</button>
<button class="btn btn-primary btn-loading">
  <span class="btn-loading-indicator"></span>
  Loading
</button>
```

### Form Control

```html
<div class="form-control">
  <label class="form-control-label">Email</label>
  <input type="text" class="form-control-input" placeholder="Enter email">
</div>
```

### Dialog

```html
<dialog class="dialog dialog-rounded dialog-btt">
  <!-- Dialog content -->
</dialog>
```

#### Dialog Animations
- `dialog-btt` - Bottom to top
- `dialog-rtl` - Right to left
- `dialog-ltr` - Left to right
- `dialog-tbd` - Top to bottom

### Table

```html
<table class="table">
  <thead>
    <tr>
      <th>Header 1</th>
      <th>Header 2</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>Data 1</td>
      <td>Data 2</td>
    </tr>
  </tbody>
</table>
```

### Tabs

```html
<div class="tab-slider tabs-md tabs-rounded tabs-three-slots">
  <div class="tab-slider-inner">
    <div class="tab-slider-track">
      <button class="tab-slider-item tab-active">Tab 1</button>
      <button class="tab-slider-item">Tab 2</button>
      <button class="tab-slider-item">Tab 3</button>
      <div class="tab-slider-naver"></div>
    </div>
  </div>
  <div class="tab-content">
    <!-- Tab content -->
  </div>
</div>
```

## Custom Utilities

### Hide Scrollbar
```html
<div class="hide-scrollbar">
  <!-- Content with hidden scrollbar -->
</div>
```

### No Transition
```html
<div class="no-transition">
  <!-- Content without transitions -->
</div>
```

### Slider Thumb
```html
<input type="range" class="slider-thumb" min="0" max="100" value="50">
```

## CSS Variables

The package provides semantic CSS variables for custom components:

### Sizes
- `--size-00` through `--size-5`
- `--size-content-1`, `--size-content-2`, `--size-content-3`

### Colors (OKLCH format without `oklch()` wrapper)
- `--white`, `--black`, `--black-800`, `--black-950`
- `--primary-400` through `--primary-700`
- `--gray-50` through `--gray-950`
- `--red-100` through `--red-500`
- `--green-50` through `--green-600`
- `--pink-500`, `--pink-600`
- `--yellow-500`
- `--blue-500`, `--blue-600`
- `--purple-500`

### Component-specific Variables
- `--clr-btn-*` - Button colors
- `--clr-form-control-*` - Form control colors
- `--clr-surface-*` - Surface colors
- `--clr-text-*` - Text colors
- `--clr-border-*` - Border colors
- `--clr-badge-*` - Badge colors

### Easing Functions
- `--ease-1`, `--ease-2`, `--ease-3`
- `--ease-elastic-in-out-2`, `--ease-elastic-in-out-3`
- `--ease-squish-2`, `--ease-squish-3`

### Animations
- `--animation-slide-in-up`, `--animation-slide-in-right`, `--animation-slide-in-left`, `--animation-slide-in-down`
- `--animation-slide-out-up`, `--animation-slide-out-right`, `--animation-slide-out-left`, `--animation-slide-out-down`
- `--animation-scale-down`

## Usage with CSS Variables

```css
.custom-component {
  background-color: oklch(var(--primary-500));
  padding: var(--size-3);
  border-radius: var(--size-1);
  transition: transform 200ms var(--ease-2);
}
```

## Font Faces

The package includes font-face declarations for:
- **Gilroy** (Regular, Medium, Semibold)
- **Inter** (Variable font)

Make sure to provide the font files at:
- `/assets/fonts/Gilroy/Gilroy-Regular.woff2`
- `/assets/fonts/Gilroy/Gilroy-Medium.woff2`
- `/assets/fonts/Gilroy/Gilroy-Semibold.woff2`
- `/assets/fonts/Inter.var.woff2`

## Tailwind CSS v4

This package is designed for Tailwind CSS v4 and uses the new `@theme` directive for design tokens. Make sure you're using Tailwind CSS v4.0.0 or later.

## Migration from main.css

If you're migrating from the original IOTA SDK main.css:

1. Replace the import:
   ```diff
   - @import "path/to/main.css";
   + @import "@iotauz/design-tokens";
   ```

2. Update your `@source` directives to point to your content files
3. Any custom theme extensions should go in a separate `@theme` block after the import

## Package Structure

```
@iotauz/design-tokens/
├── index.css          # Main entry point (imports everything)
├── theme.css          # @theme block with design tokens
├── base.css           # Base styles, fonts, CSS variables
├── components.css     # Component classes (.btn, .form-control, etc.)
├── utilities.css      # Utility classes and keyframes
└── tokens/
    ├── colors.css     # Color design tokens
    ├── typography.css # Typography tokens
    └── spacing.css    # Spacing tokens
```

## Browser Support

- Modern browsers supporting OKLCH color space
- Fallbacks for older browsers not included (use postcss plugins if needed)

## License

MIT

## Repository

https://github.com/iota-uz/iota-sdk

## Issues & Contributions

Please report issues or contribute at: https://github.com/iota-uz/iota-sdk/issues

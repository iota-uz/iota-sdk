# Tailwind CSS v4 - Quick Start Guide

## Build Commands

```bash
# Compile CSS (default - minified)
make css

# Compile CSS (dev mode - non-minified)
make css dev

# Watch mode for CSS changes
make css watch

# Watch mode for both templ and CSS changes
make dev watch

# Clean generated CSS
make css clean
```

## Where Everything Lives

### Main CSS File
```
modules/core/presentation/assets/css/main.css
```

This file contains:
- `@import "tailwindcss"` - Import Tailwind
- `@source` directives - Content paths to scan
- `@theme` block - Design tokens (colors, fonts)
- `:root` - Semantic CSS variables for custom components
- `@layer components` - Custom components (.btn, .form-control, etc.)
- `@layer utilities` - Custom utilities
- Keyframes - Animations

### Output File
```
modules/core/presentation/assets/css/main.min.css
```

This is auto-generated. Never edit manually!

## Adding New Colors

Add to `@theme` block in main.css:

```css
@theme {
  --color-purple-500: oklch(52.06% 0.2042 305.37);
}
```

Use in templates:
```html
<div class="bg-purple-500 text-white">Hello</div>
```

## Adding New Custom Components

Add to `@layer components` in main.css:

```css
@layer components {
  .my-component {
    padding: 1rem;
    border-radius: 0.5rem;
    background-color: var(--color-brand-500);
  }
}
```

## Content Detection

Tailwind v4 automatically scans these files (configured in `@source` directives):

```
modules/**/templates/**/*.templ
modules/**/templates/**/*.html
modules/**/templates/**/*.js
components/**/*.templ
components/**/*.go
```

## Troubleshooting

### Utility not generating?

1. Check if the class is used in a scanned file
2. Verify `@source` paths are correct
3. Recompile: `make css`

### Build errors?

1. Check main.css syntax
2. Run: `make css dev` for non-minified output with better errors
3. Check Tailwind version: `tailwindcss --help`

### Need to rollback?

```bash
mv modules/core/presentation/assets/css/main.css.backup main.css
mv tailwind.config.js.backup tailwind.config.js
git checkout Makefile
make css
```

## Common Tasks

### Add a new utility class
Just use it in your template - it will auto-generate if it's a valid Tailwind utility!

### Add a new color
1. Add to `@theme` in main.css
2. Run `make css`
3. Use `bg-yourcolor-500`, `text-yourcolor-500`, etc.

### Add a new font
1. Add `@font-face` declaration in main.css
2. Add to `@theme`: `--font-your-font: YourFont;`
3. Use `font-your-font` in templates

### Check what classes are generated
```bash
grep "\.your-class" modules/core/presentation/assets/css/main.min.css
```

### Verify content detection
```bash
grep -r "your-class" modules/*/presentation/templates/
```

## Resources

- [Tailwind v4 Docs](https://tailwindcss.com/docs/v4-beta)
- [OKLCH Color Picker](https://oklch.com/)
- Migration docs: `TAILWIND_V4_MIGRATION_COMPLETE.md`
- Comparison: `MIGRATION_COMPARISON.md`

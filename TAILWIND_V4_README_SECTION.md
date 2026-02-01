# Add this section to your main README.md

## CSS/Styling

This project uses **Tailwind CSS v4** for styling.

### Quick Commands

```bash
# Build CSS (production)
make css

# Build CSS (development - non-minified)
make css dev

# Watch for CSS changes
make css watch

# Watch for both template and CSS changes
make dev watch
```

### Configuration

Unlike Tailwind v3, v4 uses CSS-first configuration. All configuration is in:
```
modules/core/presentation/assets/css/main.css
```

**No `tailwind.config.js` file is needed!**

### Key Features

- ✅ **CSS-First Configuration** - Everything configured in CSS using `@theme`
- ✅ **Content Detection** - Automatically scans `.templ`, `.go`, `.html`, `.js` files
- ✅ **OKLCH Colors** - Modern color system with perceptual uniformity
- ✅ **Custom Components** - Preserved custom `.btn`, `.form-control`, `.dialog`, `.table` components
- ✅ **Dark Mode** - Full dark mode support via `html.dark` selector

### Adding New Styles

#### Add a new color:
```css
/* In main.css @theme block */
@theme {
  --color-accent-500: oklch(65% 0.15 200);
}
```

Then use in templates:
```html
<div class="bg-accent-500">Content</div>
```

#### Add a custom component:
```css
/* In main.css @layer components block */
@layer components {
  .my-card {
    padding: 1rem;
    border-radius: 0.5rem;
    background-color: white;
    box-shadow: 0 1px 3px rgba(0,0,0,0.1);
  }
}
```

### Documentation

- [Quick Start Guide](TAILWIND_V4_QUICK_START.md)
- [Migration Details](TAILWIND_V4_MIGRATION_COMPLETE.md)
- [Before/After Comparison](MIGRATION_COMPARISON.md)
- [Official Tailwind v4 Docs](https://tailwindcss.com/docs/v4-beta)

### Troubleshooting

**Styles not updating?**
```bash
make css
```

**Want to see non-minified output?**
```bash
make css dev
```

**Class not generating?**
- Make sure it's used in a scanned file (`.templ`, `.go`, `.html`, `.js`)
- Check if it's a valid Tailwind utility
- Recompile: `make css`

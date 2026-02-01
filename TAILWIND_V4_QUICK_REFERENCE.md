# Tailwind CSS v4 Migration - Quick Reference

**Quick lookup guide for common migration patterns**

---

## 1. Configuration File Changes

| v3 | v4 |
|----|-----|
| `tailwind.config.js` (JavaScript) | CSS file with `@theme` directive |
| Theme in JS object | Theme in CSS variables |
| Plugins in JS array | `@plugin` directives in CSS |
| Content in JS array | `@source` directives in CSS or CLI flags |

---

## 2. CSS File Structure

### v3 Structure
```css
@tailwind base;
@tailwind components;
@tailwind utilities;

@layer base {
  :root { /* custom vars */ }
}

@layer components {
  .btn { /* custom classes */ }
}
```

### v4 Structure
```css
@import "tailwindcss";

@source "../path/to/files/**/*.ext";

@theme {
  --color-brand-500: oklch(58.73% 0.23 279.66);
  --font-family-sans: "Gilroy", sans-serif;
}

@layer base {
  :root { /* custom vars */ }
}

@layer components {
  .btn { /* custom classes */ }
}
```

---

## 3. Color Migration

### Your Current OKLCH Setup (v3)

**In CSS**:
```css
:root {
  --primary-500: 58.73% 0.23 279.66;
}
```

**In tailwind.config.js**:
```javascript
colors: {
  brand: {
    500: "oklch(var(--primary-500) / <alpha-value>)"
  }
}
```

### v4 OKLCH Setup (Simplified)

**In CSS only**:
```css
@theme {
  --color-brand-500: oklch(58.73% 0.23 279.66);
}
```

**That's it!** No config file needed. Use as `bg-brand-500` or `text-brand-500`.

---

## 4. Color Naming Convention

In v4, prefix color variables with `--color-*` for Tailwind to generate utilities:

| CSS Variable | Generated Utilities |
|--------------|---------------------|
| `--color-brand-500` | `bg-brand-500`, `text-brand-500`, `border-brand-500` |
| `--color-gray-100` | `bg-gray-100`, `text-gray-100`, `border-gray-100` |
| `--color-success` | `bg-success`, `text-success`, `border-success` |

**Pattern**: `--color-{name}-{variant}` → `{utility}-{name}-{variant}`

---

## 5. Your Color Palette Conversion

Quick mapping for your main colors:

```css
@theme {
  /* Primary/Brand */
  --color-brand-400: oklch(62.51% 0.172 283.89);
  --color-brand-500: oklch(58.73% 0.23 279.66);
  --color-brand-600: oklch(50% 0.192 279.97);
  --color-brand-650: oklch(52.52% 0.272 280.57);
  --color-brand-700: oklch(45.57% 0.171 280.34);
  
  /* Grays (copy from your :root) */
  --color-gray-50: oklch(98.5% 0.002 247.839);
  --color-gray-100: oklch(96.7% 0.003 264.542);
  --color-gray-200: oklch(92.8% 0.006 264.531);
  --color-gray-300: oklch(87.2% 0.01 258.338);
  --color-gray-400: oklch(70.7% 0.022 261.325);
  --color-gray-500: oklch(55.1% 0.027 264.364);
  --color-gray-600: oklch(44.6% 0.03 256.802);
  --color-gray-700: oklch(37.3% 0.034 259.733);
  --color-gray-800: oklch(27.8% 0.033 256.848);
  --color-gray-900: oklch(21% 0.034 264.665);
  --color-gray-950: oklch(13% 0.028 261.692);
  
  /* Semantic colors */
  --color-red-500: oklch(59.16% 0.218 0.58);
  --color-green-500: oklch(78.02% 0.1534 168.27);
  --color-yellow-500: oklch(80.13% 0.1458 73.41);
  --color-blue-500: oklch(82.1% 0.099263 240.9782);
  --color-purple-500: oklch(52.06% 0.2042 305.37);
  
  /* Black/White */
  --color-white: oklch(100% 0 0);
  --color-black: oklch(18.67% 0 0);
  --color-black-800: oklch(26% 0 0);
  --color-black-950: oklch(16.84% 0 0);
}
```

---

## 6. Semantic Tokens (Keep in :root)

Your semantic tokens (--clr-btn-bg, --clr-surface-100, etc.) should **NOT** go in `@theme`:

```css
@theme {
  /* Only design tokens that generate utilities */
}

@layer base {
  :root {
    /* Semantic/component-specific tokens */
    --clr-btn-bg: var(--color-gray-100);
    --clr-surface-100: var(--color-gray-50);
    --clr-text-100: var(--color-black);
    /* ... */
  }
  
  html.dark {
    --clr-surface-100: var(--color-black-950);
    --clr-text-100: var(--color-white);
    /* ... */
  }
}
```

**Rule of Thumb**:
- `@theme`: Generates Tailwind utilities (colors, fonts, spacing)
- `:root`: Custom CSS variables for components

---

## 7. Content/Source Configuration

### Option 1: @source in CSS (Recommended)

```css
@import "tailwindcss";
@source "../modules/**/templates/**/*.templ";
@source "../components/**/*.go";
```

### Option 2: CLI Flags

```bash
tailwindcss -i input.css -o output.css \
  --content './modules/**/*.templ' \
  --content './components/**/*.go'
```

### Option 3: Remove -c Flag from Makefile

**Before**:
```makefile
tailwindcss -c tailwind.config.js -i $(INPUT) -o $(OUTPUT)
```

**After**:
```makefile
tailwindcss -i $(INPUT) -o $(OUTPUT)
```

(All config now in CSS via `@theme` and `@source`)

---

## 8. Font Configuration

### v3
```javascript
// tailwind.config.js
theme: {
  extend: {
    fontFamily: {
      sans: ["Gilroy"]
    }
  }
}
```

### v4
```css
@theme {
  --font-family-sans: "Gilroy", system-ui, sans-serif;
}

/* @font-face declarations stay the same */
@font-face {
  font-family: "Gilroy";
  src: url(/assets/fonts/Gilroy/Gilroy-Regular.woff2) format('woff2');
  font-weight: 400;
}
```

---

## 9. Dark Mode

### Your Current Pattern (Works in v4!)

```css
@layer base {
  :root {
    --clr-surface-100: var(--color-gray-50);
  }
  
  html.dark {
    --clr-surface-100: var(--color-black-950);
  }
}
```

**No changes needed!** This pattern is fully compatible.

---

## 10. Build Commands

### Standalone CLI

**v3**:
```bash
tailwindcss -c tailwind.config.js -i input.css -o output.css --watch
```

**v4**:
```bash
# Option A: No config file (everything in CSS)
tailwindcss -i input.css -o output.css --watch

# Option B: With content flags
tailwindcss -i input.css -o output.css --watch \
  --content './modules/**/*.templ'
```

### PostCSS (ai-chat)

**v3** (`postcss.config.js`):
```javascript
module.exports = {
  plugins: {
    'tailwindcss': {},
    'autoprefixer': {},
  }
}
```

**v4** (`postcss.config.mjs`):
```javascript
export default {
  plugins: {
    "@tailwindcss/postcss": {},
  }
}
```

---

## 11. File Checklist

Files to modify:

- [ ] `tailwind.config.js` → Delete (or backup)
- [ ] `ai-chat/tailwind.config.ts` → Delete (or backup)
- [ ] `modules/core/presentation/assets/css/main.css` → Update
- [ ] `ai-chat/app/globals.css` → Update
- [ ] `Makefile` → Update CSS build commands
- [ ] `ai-chat/postcss.config.js` → Rename to `.mjs`, update plugin
- [ ] `ai-chat/package.json` → Update dependencies

---

## 12. Dependencies Update

### ai-chat package.json

**Before**:
```json
{
  "devDependencies": {
    "tailwindcss": "^3.4.17",
    "postcss": "^8",
    "autoprefixer": "^10.4.20"
  }
}
```

**After**:
```json
{
  "devDependencies": {
    "tailwindcss": "^4.0.0",
    "@tailwindcss/postcss": "^4.0.0",
    "postcss": "^8"
  }
}
```

**Note**: `autoprefixer` removed (built into v4)

---

## 13. Common Pitfalls

### ❌ Don't Do This

```css
/* DON'T: Use old @tailwind directives */
@tailwind base;
@tailwind components;
@tailwind utilities;
```

```css
/* DON'T: Put semantic tokens in @theme */
@theme {
  --clr-btn-bg: oklch(100% 0 0);  /* Wrong! */
}
```

```css
/* DON'T: Forget color prefix */
@theme {
  --brand-500: oklch(...);  /* Won't generate utilities! */
}
```

### ✅ Do This

```css
/* DO: Use @import */
@import "tailwindcss";
```

```css
/* DO: Put design tokens in @theme */
@theme {
  --color-brand-500: oklch(58.73% 0.23 279.66);
}
```

```css
/* DO: Keep semantic tokens in :root */
@layer base {
  :root {
    --clr-btn-bg: var(--color-gray-100);
  }
}
```

---

## 14. Testing Utilities

After migration, verify these work:

```html
<!-- Color utilities -->
<div class="bg-brand-500">Brand background</div>
<div class="text-gray-400">Gray text</div>
<div class="border-green-500">Green border</div>

<!-- Opacity variants -->
<div class="bg-brand-500/50">50% opacity</div>
<div class="bg-red-500/20">20% opacity</div>

<!-- Dark mode (if using html.dark) -->
<div class="dark:bg-black dark:text-white">Dark mode</div>

<!-- Font family -->
<div class="font-sans">Gilroy font</div>

<!-- Custom components (should be unchanged) -->
<button class="btn btn-primary">Button</button>
<input class="form-control" />
```

---

## 15. Migration Commands

### Quick Migration Steps

```bash
# 1. Backup
git checkout -b feature/tailwind-v4-migration
cp tailwind.config.js tailwind.config.js.backup

# 2. Try automated tool
npx @tailwindcss/upgrade

# 3. Review changes
git diff

# 4. Test build
make css

# 5. If successful, commit
git add -A
git commit -m "chore: migrate to Tailwind v4"
```

### Rollback if Needed

```bash
git reset --hard HEAD^
mv tailwind.config.js.backup tailwind.config.js
```

---

## 16. Need Help?

- **Official Docs**: https://tailwindcss.com/docs/upgrade-guide
- **Full Migration Guide**: See `TAILWIND_V4_MIGRATION.md` in this repo
- **GitHub Issues**: https://github.com/tailwindlabs/tailwindcss/issues
- **Discord**: https://tailwindcss.com/discord

---

## Quick Decision: Should I Migrate?

**✅ Migrate if**:
- Have 2-4 days available
- Want better OKLCH support
- Want faster builds
- Can test thoroughly

**⏸️ Wait if**:
- Current v3 works perfectly
- Tight deadline approaching
- Need IE11 support
- Risk-averse environment

---

**Last Updated**: 2025-02-01  
**Status**: Ready for migration

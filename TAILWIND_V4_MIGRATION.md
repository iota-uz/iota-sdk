# Tailwind CSS v4 Migration Research Report

**Date**: 2025-02-01  
**Current Setup**: Tailwind CSS v3 with standalone CLI  
**Target**: Tailwind CSS v4

---

## Executive Summary

Tailwind CSS v4 represents a **major architectural shift** from JavaScript-based configuration to a **CSS-first approach**. This migration will require significant changes to your configuration files, CSS structure, and build process, but your OKLCH color system is **fully compatible** and actually becomes **easier to manage** in v4.

**Key Impact Areas**:
- ✅ **OKLCH color system**: Fully compatible, improved syntax
- ⚠️ **Configuration files**: Complete rewrite from JS to CSS required
- ⚠️ **@tailwind directives**: Replaced with `@import "tailwindcss"`
- ⚠️ **Build pipeline**: CLI commands remain similar, but config handling changes
- ✅ **Standalone CLI**: Still supported, v4 binaries available
- ⚠️ **Content paths**: Can be handled via CSS `@source` directive or CLI flags

---

## 1. Breaking Changes Affecting Your Codebase

### 1.1 Configuration System (MAJOR CHANGE)

**Old (v3)**: JavaScript configuration in `tailwind.config.js`
```javascript
module.exports = {
  content: ["./modules/**/templates/**/*.templ", "./components/**/*.go"],
  theme: {
    extend: {
      colors: {
        brand: {
          500: "oklch(var(--primary-500) / <alpha-value>)"
        }
      }
    }
  }
}
```

**New (v4)**: CSS-first configuration using `@theme` directive
```css
@import "tailwindcss";

@source "../modules/**/templates/**/*.templ";
@source "../components/**/*.go";

@theme {
  --color-brand-500: oklch(58.73% 0.23 279.66);
  --color-brand-600: oklch(50% 0.192 279.97);
  /* Direct OKLCH values, no var() wrapper needed */
}
```

**Impact on Your Project**:
- **Both** `tailwind.config.js` (root) and `ai-chat/tailwind.config.ts` must be migrated to CSS
- All theme customizations move to CSS files
- `content` array replaced with `@source` directives in CSS

---

### 1.2 @tailwind Directives (BREAKING CHANGE)

**Old (v3)**: In `modules/core/presentation/assets/css/main.css`
```css
@tailwind base;
@tailwind components;
@tailwind utilities;
```

**New (v4)**: Single import
```css
@import "tailwindcss";
```

**Impact**: Every CSS file using `@tailwind` directives must be updated.

**Files to Update**:
- `modules/core/presentation/assets/css/main.css` ✓
- `ai-chat/app/globals.css` ✓

---

### 1.3 OKLCH Color System (COMPATIBLE - SYNTAX CHANGE)

Your current OKLCH implementation is **100% compatible** with v4, but the syntax **improves significantly**.

**Current v3 Pattern** (in `main.css`):
```css
:root {
  --primary-500: 58.73% 0.23 279.66;
}

/* In tailwind.config.js */
colors: {
  brand: {
    500: "oklch(var(--primary-500) / <alpha-value>)"
  }
}
```

**New v4 Pattern** (much simpler):
```css
@import "tailwindcss";

@theme {
  /* Direct OKLCH definition - Tailwind generates utilities automatically */
  --color-brand-500: oklch(58.73% 0.23 279.66);
  --color-brand-600: oklch(50% 0.192 279.97);
  --color-brand-700: oklch(45.57% 0.171 280.34);
  
  /* With opacity support */
  --color-primary-500: oklch(58.73% 0.23 279.66 / <alpha-value>);
  
  /* Gray scale (already OKLCH in your setup) */
  --color-gray-50: oklch(98.5% 0.002 247.839);
  --color-gray-100: oklch(96.7% 0.003 264.542);
  /* ... rest of gray palette */
}
```

**Benefits for Your Color System**:
1. **Simpler**: No double-wrapping with `var()` and `oklch()`
2. **Direct mapping**: CSS variables directly map to utility classes
3. **Better theming**: Easier to override in `.dark` selector
4. **No config file needed**: Everything in CSS

**Dark Mode Pattern** (from your `html.dark` styles):
```css
@theme {
  --color-surface-100: oklch(18.67% 0 0);  /* Light mode: black-950 */
}

html.dark {
  --color-surface-100: oklch(16.84% 0 0);  /* Dark mode: even darker */
}
```

**Migration Impact**:
- ✅ All your OKLCH values are directly compatible
- ✅ No need to change color values
- ⚠️ Need to rename variables from `--primary-500` → `--color-brand-500` (or similar)
- ⚠️ Move from `tailwind.config.js` → `@theme` in CSS
- ✅ Your complex color system with semantic tokens (--clr-btn-bg, etc.) can stay in `:root`

---

### 1.4 Content Detection & @source Directive

**Old (v3)**: In `tailwind.config.js`
```javascript
content: [
  "./modules/**/templates/**/*.{html,js,templ}",
  "./components/**/*.{html,js,templ,go}",
]
```

**New (v4)**: Two options

**Option A - CSS `@source` directive** (Recommended):
```css
@import "tailwindcss";
@source "../modules/**/templates/**/*.templ";
@source "../modules/**/templates/**/*.html";
@source "../components/**/*.go";
@source "../components/**/*.templ";
```

**Option B - CLI flags**:
```bash
tailwindcss --content './modules/**/templates/**/*.templ' \
            --content './components/**/*.go' \
            -i input.css -o output.css
```

**Current Setup Impact**:
- Your Makefile uses: `tailwindcss -c tailwind.config.js -i $(TAILWIND_INPUT) -o $(TAILWIND_OUTPUT)`
- **Option 1**: Keep CLI command, add `@source` directives to CSS
- **Option 2**: Remove `-c tailwind.config.js`, add `--content` flags
- **Option 3**: Rely on auto-detection (risky for `.templ` and `.go` files)

---

### 1.5 Font-Face Declarations (NO CHANGE REQUIRED)

Your `@font-face` declarations in `main.css` are **standard CSS** and work exactly the same in v4:
```css
@font-face {
  font-family: "Gilroy";
  font-style: normal;
  font-display: swap;
  src: url(/assets/fonts/Gilroy/Gilroy-Regular.woff2) format('woff2');
  font-weight: 400;
}
```

**Font family configuration** moves to `@theme`:
```css
@theme {
  --font-family-sans: "Gilroy", system-ui, sans-serif;
}
```

Then use as: `font-sans` utility class.

---

### 1.6 Custom Component Classes (NO CHANGE REQUIRED)

Your custom component classes (`.btn`, `.form-control`, `.dialog`, etc.) remain **100% compatible**:
```css
@layer components {
  .btn {
    /* All your custom CSS - no changes needed */
  }
}
```

However, `@layer` directives work slightly differently in v4:
- `@layer base` → Still supported
- `@layer components` → Still supported  
- `@layer utilities` → Still supported

But they must come **after** `@import "tailwindcss"`:
```css
@import "tailwindcss";

@layer base {
  /* Your base styles */
}

@layer components {
  /* Your components */
}
```

---

## 2. Migration Steps

### Phase 1: Preparation (Before Migration)

1. **Backup & Git Branch**
   ```bash
   git checkout -b feature/tailwind-v4-migration
   git add -A
   git commit -m "Pre-migration checkpoint"
   ```

2. **Identify All Tailwind Files**
   - `tailwind.config.js` (root)
   - `ai-chat/tailwind.config.ts`
   - `modules/core/presentation/assets/css/main.css`
   - `ai-chat/app/globals.css`
   - `Makefile` (CSS build commands)

3. **Document Current Color Variables**
   ```bash
   # Extract all CSS variables for reference
   grep -E "^\s*--" modules/core/presentation/assets/css/main.css > colors-inventory.txt
   ```

---

### Phase 2: Automated Migration (Recommended First Step)

**Official Upgrade Tool**:
```bash
npx @tailwindcss/upgrade
```

**What it does**:
- Updates dependencies to v4
- Converts `tailwind.config.js` → CSS `@theme` blocks
- Updates `@tailwind` directives → `@import "tailwindcss"`
- Updates utility class names (if any breaking changes)

**Known Issues**:
- ⚠️ Windows path issues (use WSL if on Windows)
- ⚠️ May not perfectly handle complex OKLCH setups
- ⚠️ Manual review always required

**For ai-chat Next.js project**:
```bash
cd ai-chat
npx @tailwindcss/upgrade
```

---

### Phase 3: Manual Migration (More Control)

#### Step 1: Update Dependencies

**For ai-chat (Next.js)**:
```json
// ai-chat/package.json
{
  "devDependencies": {
    "tailwindcss": "^4.0.0",
    "@tailwindcss/postcss": "^4.0.0",  // New in v4
    "postcss": "^8",
    "autoprefixer": "^10.4.20"  // Still needed
  }
}
```

**For main project (Standalone CLI)**:
- Download v4 binary: `https://github.com/tailwindlabs/tailwindcss/releases`
- Replace existing `tailwindcss` binary

#### Step 2: Create PostCSS Config (ai-chat only)

```javascript
// ai-chat/postcss.config.mjs
export default {
  plugins: {
    "@tailwindcss/postcss": {},
  }
}
```

**Remove**: `postcss-import`, `autoprefixer` from plugins (now built-in to v4)

#### Step 3: Migrate Main CSS File

**Before** (`modules/core/presentation/assets/css/main.css`):
```css
@tailwind base;
@tailwind components;
@tailwind utilities;

@font-face { /* ... */ }

@layer base {
  :root { /* CSS variables */ }
}

@layer components {
  .btn { /* ... */ }
}
```

**After** (v4 structure):
```css
@import "tailwindcss";

/* 1. Content sources */
@source "../../../**/templates/**/*.templ";
@source "../../../../components/**/*.go";

/* 2. Theme configuration */
@theme {
  /* Fonts */
  --font-family-sans: "Gilroy", system-ui, sans-serif;
  
  /* Base colors (from your :root variables) */
  --color-white: oklch(100% 0 0);
  --color-black: oklch(18.67% 0 0);
  --color-black-800: oklch(26% 0 0);
  --color-black-950: oklch(16.84% 0 0);
  
  /* Primary palette */
  --color-brand-400: oklch(62.51% 0.172 283.89);
  --color-brand-500: oklch(58.73% 0.23 279.66);
  --color-brand-600: oklch(50% 0.192 279.97);
  --color-brand-650: oklch(52.52% 0.272 280.57);
  --color-brand-700: oklch(45.57% 0.171 280.34);
  
  /* Gray scale */
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
  
  /* Red */
  --color-red-500: oklch(59.16% 0.218 0.58);
  --color-red-100: oklch(59.16% 0.218 0.58 / 10%);
  --color-red-200: oklch(59.16% 0.218 0.58 / 20%);
  --color-red-600: oklch(59.16% 0.218 0.58);
  --color-red-700: oklch(59.16% 0.218 0.58);
  
  /* Green */
  --color-green-50: oklch(96.3% 0.0173 168.27);
  --color-green-100: oklch(92.4% 0.0346 168.27);
  --color-green-200: oklch(84.9% 0.0692 168.27);
  --color-green-500: oklch(78.02% 0.1534 168.27);
  --color-green-600: oklch(64.33% 0.1128 171.09);
  
  /* Pink */
  --color-pink-500: oklch(59.16% 0.218 0.58);
  --color-pink-600: oklch(58.93% 0.2129 359.93);
  
  /* Yellow */
  --color-yellow-500: oklch(80.13% 0.1458 73.41);
  
  /* Blue */
  --color-blue-500: oklch(82.1% 0.099263 240.9782);
  --color-blue-600: oklch(64.12% 0.0638 240.72);
  
  /* Purple */
  --color-purple-500: oklch(52.06% 0.2042 305.37);
  
  /* Success (alias) */
  --color-success: oklch(78.02% 0.1534 168.27);
}

/* 3. Font faces */
@font-face {
  font-family: "Inter";
  font-style: normal;
  font-display: swap;
  src: url(/assets/fonts/Inter.var.woff2) format('woff2-variations');
  font-weight: 1 1000;
}

@font-face {
  font-family: "Gilroy";
  font-style: normal;
  font-display: swap;
  src: url(/assets/fonts/Gilroy/Gilroy-Regular.woff2) format('woff2');
  font-weight: 400;
}

@font-face {
  font-family: "Gilroy";
  font-style: normal;
  font-display: swap;
  src: url(/assets/fonts/Gilroy/Gilroy-Medium.woff2) format('woff2');
  font-weight: 500;
}

@font-face {
  font-family: "Gilroy";
  font-style: normal;
  font-display: swap;
  src: url(/assets/fonts/Gilroy/Gilroy-Semibold.woff2) format('woff2');
  font-weight: 600;
}

/* 4. Semantic design tokens (keep in :root, NOT in @theme) */
@layer base {
  :root {
    /* Sizes */
    --size-00: 0.25rem;
    --size-0: 0.4rem;
    --size-1: 0.5rem;
    --size-2: 0.75rem;
    --size-3: 1rem;
    --size-4: 1.25rem;
    --size-5: 1.5rem;
    --size-content-1: 20ch;
    --size-content-2: 45ch;
    --size-content-3: 60ch;
    
    /* Border */
    --border-size-1: 1px;
    
    /* Semantic button colors */
    --clr-btn-bg: oklch(0 0 0 / 0);
    --clr-btn-bg-active: var(--color-gray-200);
    --clr-btn-bg-hover: var(--color-gray-100);
    --clr-btn-text: var(--color-black);
    --crl-btn-text-hover: var(--color-black);
    /* ... rest of your semantic tokens ... */
    
    /* Shadows */
    --shadow-100: 0px 4px 8px 0px rgba(0, 0, 0, 0.16);
    --shadow-200: 0px 8px 8px 0px rgba(0, 0, 0, 0.08);
    
    /* Easing */
    --ease-1: cubic-bezier(.25, 0, .5, 1);
    --ease-2: cubic-bezier(.25, 0, .4, 1);
    /* ... */
  }
  
  /* Dark mode overrides */
  html.dark {
    --clr-surface-100: var(--color-black-950);
    --clr-text-100: var(--color-white);
    /* ... rest of dark mode tokens ... */
  }
  
  /* Other base styles */
  ::selection {
    background-color: var(--color-brand-600);
    color: var(--color-white);
  }
  
  button {
    cursor: default;
  }
  
  /* ... rest of your base layer ... */
}

/* 5. Components layer - NO CHANGES */
@layer components {
  .form-control {
    /* Exactly as before */
  }
  
  .btn {
    /* Exactly as before */
  }
  
  /* ... all your component classes ... */
}

/* 6. Utilities layer - NO CHANGES */
@layer utilities {
  .hide-scrollbar {
    scrollbar-width: none;
  }
  /* ... */
}

/* 7. Keyframes - NO CHANGES */
@keyframes slide-in-right {
  from {
    transform: translateX(100%)
  }
}
/* ... */
```

#### Step 4: Migrate ai-chat Configuration

**Before** (`ai-chat/app/globals.css`):
```css
@tailwind base;
@tailwind components;
@tailwind utilities;

:root {
  --background: 0 0% 100%;
  --primary: 214 60% 44%;
  /* ... */
}
```

**After**:
```css
@import "tailwindcss";

@theme {
  /* Convert HSL to OKLCH for consistency */
  --color-background: oklch(100% 0 0);  /* white */
  --color-foreground: oklch(20.5% 0.028 264.665);  /* ~222.2 84% 4.9% in HSL */
  
  /* Or keep HSL if preferred - v4 supports both */
  /* --color-background: hsl(0 0% 100%); */
  
  /* Primary */
  --color-primary: oklch(54% 0.14 250);  /* Approximate for 214 60% 44% */
  --color-primary-foreground: oklch(98% 0.002 247);
  
  /* ... rest of shadcn/ui colors ... */
  
  /* Radius */
  --radius: 0.5rem;
}

@layer base {
  :root {
    /* Any remaining semantic tokens */
  }
  
  .dark {
    /* Dark mode overrides */
  }
}

/* Component styles unchanged */
```

#### Step 5: Remove Old Config Files

After migration is complete and tested:
```bash
# Backup first!
mv tailwind.config.js tailwind.config.js.v3.backup
mv ai-chat/tailwind.config.ts ai-chat/tailwind.config.ts.v3.backup
```

---

### Phase 4: Update Build Pipeline

#### Update Makefile (Standalone CLI)

**Before**:
```makefile
tailwindcss -c tailwind.config.js -i $(TAILWIND_INPUT) -o $(TAILWIND_OUTPUT) --minify
```

**After (Option 1 - No config file)**:
```makefile
# All configuration in CSS via @theme and @source
tailwindcss -i $(TAILWIND_INPUT) -o $(TAILWIND_OUTPUT) --minify
```

**After (Option 2 - With content flags)**:
```makefile
# Explicit content paths
TAILWIND_CONTENT := './modules/**/templates/**/*.templ' './components/**/*.go'
tailwindcss -i $(TAILWIND_INPUT) -o $(TAILWIND_OUTPUT) --minify \
  --content $(TAILWIND_CONTENT)
```

**Recommended**: Use `@source` directives in CSS (Option 1)

#### Update ai-chat Build

If using Next.js, no changes to package.json scripts needed:
```json
{
  "scripts": {
    "dev": "next dev",    // PostCSS plugin handles it
    "build": "next build"
  }
}
```

---

## 3. Shareable/Extendable Configuration in v4

### Problem: Two Separate Configs

You currently have:
1. `tailwind.config.js` - Main Go/Templ project
2. `ai-chat/tailwind.config.ts` - React/Next.js project

### Solution 1: Shared CSS Theme File

Create a shared theme file that both projects import:

**File**: `shared-theme.css`
```css
@theme {
  /* Shared color palette */
  --color-brand-500: oklch(58.73% 0.23 279.66);
  --color-brand-600: oklch(50% 0.192 279.97);
  
  /* Shared gray scale */
  --color-gray-50: oklch(98.5% 0.002 247.839);
  --color-gray-100: oklch(96.7% 0.003 264.542);
  /* ... */
  
  /* Shared font */
  --font-family-sans: "Gilroy", system-ui, sans-serif;
}
```

**Main project** (`modules/core/presentation/assets/css/main.css`):
```css
@import "tailwindcss";
@import "./shared-theme.css";  /* Reuse shared theme */

@source "../../../**/templates/**/*.templ";
@source "../../../../components/**/*.go";

/* Project-specific theme extensions */
@theme {
  /* Add any main-project-specific tokens */
}
```

**ai-chat project** (`ai-chat/app/globals.css`):
```css
@import "tailwindcss";
@import "../../shared-theme.css";  /* Reuse shared theme */

/* ai-chat-specific theme extensions */
@theme {
  --color-sidebar-background: oklch(98% 0 0);
  /* shadcn/ui specific tokens */
}
```

### Solution 2: NPM Package for Shared Theme

For better reusability:

1. Create `packages/shared-theme/theme.css`:
```css
@theme {
  /* All shared design tokens */
}
```

2. Publish as npm package or use workspace:
```json
// package.json
{
  "name": "@iotauz/shared-theme",
  "version": "1.0.0",
  "main": "theme.css"
}
```

3. Import in both projects:
```css
@import "@iotauz/shared-theme";
```

### Solution 3: CSS Layers for Extension

Use CSS layers to allow overrides:

**shared-theme.css**:
```css
@layer theme {
  @theme {
    /* Base theme */
  }
}
```

**Project-specific**:
```css
@import "./shared-theme.css";

@layer theme {
  @theme {
    /* Override specific tokens */
    --color-brand-500: oklch(60% 0.25 280);  /* Slightly different */
  }
}
```

---

## 4. CLI Usage Changes

### v3 Standalone CLI

```bash
# v3 - Requires config file
tailwindcss -c tailwind.config.js -i input.css -o output.css --watch
```

### v4 Standalone CLI

```bash
# v4 - Config optional (use @theme in CSS instead)
tailwindcss -i input.css -o output.css --watch

# OR with explicit content paths
tailwindcss -i input.css -o output.css --watch \
  --content './modules/**/*.templ' \
  --content './components/**/*.go'
```

**Key Changes**:
- `-c tailwind.config.js` is **optional** (not required)
- Configuration now lives in CSS via `@theme` and `@source`
- Content detection can be in CSS or CLI flags
- Watch mode unchanged: `--watch`
- Minify unchanged: `--minify`

**Download v4 Binary**:
```bash
# Linux x64
curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64
chmod +x tailwindcss-linux-x64
mv tailwindcss-linux-x64 tailwindcss

# macOS ARM
curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-macos-arm64
chmod +x tailwindcss-macos-arm64
mv tailwindcss-macos-arm64 tailwindcss
```

---

## 5. OKLCH Color System Compatibility

### ✅ Full Compatibility Confirmed

Your OKLCH-based color system is **100% compatible** with Tailwind v4. In fact, **v4 makes OKLCH the default color space**, so you're ahead of the curve!

### Why v4 is Better for OKLCH

1. **Native OKLCH support**: v4's default palette uses OKLCH
2. **Simpler syntax**: No need to wrap in `oklch()` function in config
3. **Better opacity handling**: `oklch(L C H / alpha)` works natively
4. **Perceptual uniformity**: v4 embraces OKLCH benefits

### Migration Pattern

**Current v3 approach** (unnecessarily complex):
```css
/* CSS variable */
--primary-500: 58.73% 0.23 279.66;

/* Config file */
colors: {
  brand: {
    500: "oklch(var(--primary-500) / <alpha-value>)"
  }
}

/* Usage */
<div class="bg-brand-500/50">  <!-- Works -->
```

**New v4 approach** (simplified):
```css
@theme {
  /* Direct definition */
  --color-brand-500: oklch(58.73% 0.23 279.66);
}

/* Usage */
<div class="bg-brand-500/50">  <!-- Works the same -->
```

### Opacity Variants

**Built-in opacity support**:
```css
@theme {
  /* Method 1: With <alpha-value> placeholder */
  --color-brand-500: oklch(58.73% 0.23 279.66 / <alpha-value>);
  
  /* Method 2: Fixed opacity variants */
  --color-red-500: oklch(59.16% 0.218 0.58);
  --color-red-500-10: oklch(59.16% 0.218 0.58 / 10%);
  --color-red-500-20: oklch(59.16% 0.218 0.58 / 20%);
}
```

### Your Specific Color Values

All your current OKLCH values work as-is:

| Current Variable | v4 Mapping |
|------------------|------------|
| `--primary-500: 58.73% 0.23 279.66` | `--color-brand-500: oklch(58.73% 0.23 279.66)` |
| `--gray-50: 98.5% 0.002 247.839` | `--color-gray-50: oklch(98.5% 0.002 247.839)` |
| `--green-500: 78.02% 0.1534 168.27` | `--color-green-500: oklch(78.02% 0.1534 168.27)` |
| `--red-500: 59.16% 0.218 0.58` | `--color-red-500: oklch(59.16% 0.218 0.58)` |

**No value changes needed** - just move them to `@theme` block!

---

## 6. PostCSS Integration Changes (ai-chat only)

### Current Setup (v3)

```javascript
// postcss.config.js (typical v3 setup)
module.exports = {
  plugins: {
    'postcss-import': {},
    'tailwindcss': {},
    'autoprefixer': {},
  }
}
```

### New Setup (v4)

```javascript
// postcss.config.mjs
export default {
  plugins: {
    "@tailwindcss/postcss": {},  // New dedicated plugin
  }
}
```

**Key Changes**:
- Use `@tailwindcss/postcss` instead of `tailwindcss`
- Remove `postcss-import` (built into v4)
- Remove `autoprefixer` (built into v4)
- Can use `.mjs` extension for ESM

**Install**:
```bash
npm install tailwindcss@next @tailwindcss/postcss@next
```

---

## 7. Plugin Compatibility

### Official Plugins (No Plugins Currently Used)

You don't currently use any Tailwind plugins, but if you add them:

**v3 approach**:
```javascript
// tailwind.config.js
module.exports = {
  plugins: [
    require('@tailwindcss/forms'),
    require('@tailwindcss/typography'),
  ]
}
```

**v4 approach**:
```css
/* In your CSS file */
@import "tailwindcss";
@plugin "@tailwindcss/forms";
@plugin "@tailwindcss/typography";
```

**Migration**: Move from JS `plugins: []` array to CSS `@plugin` directives

---

## 8. Content Configuration Changes

### Current Setup

**tailwind.config.js**:
```javascript
content: [
  "./modules/**/templates/**/*.{html,js,templ}",
  "./components/**/*.{html,js,templ,go}",
]
```

**ai-chat/tailwind.config.ts**:
```typescript
content: [
  "./pages/**/*.{js,ts,jsx,tsx,mdx}",
  "./components/**/*.{js,ts,jsx,tsx,mdx}",
  "./app/**/*.{js,ts,jsx,tsx,mdx}",
]
```

### v4 Options

**Option 1: @source directive in CSS** (Recommended)
```css
/* main.css */
@import "tailwindcss";
@source "../modules/**/templates/**/*.templ";
@source "../components/**/*.go";

/* ai-chat/app/globals.css */
@import "tailwindcss";
@source "../pages/**/*.{js,ts,jsx,tsx,mdx}";
@source "../components/**/*.{js,ts,jsx,tsx,mdx}";
@source "../app/**/*.{js,ts,jsx,tsx,mdx}";
```

**Option 2: CLI flags**
```bash
tailwindcss -i input.css -o output.css \
  --content './modules/**/*.templ' \
  --content './components/**/*.go'
```

**Option 3: Auto-detection**
- v4 can auto-detect common files
- **NOT recommended** for custom extensions like `.templ`

### Excluding Paths (v4.1+)

```css
@source "../modules/**/*.templ";
@source not "../modules/legacy/**";  /* Exclude legacy code */
```

### Safelisting Classes

For dynamic classes that won't be detected:
```css
@source inline(btn-primary, text-red-500, bg-brand-600);
```

---

## 9. Browser Support Changes

### v3 Browser Support
- IE 11+ (with polyfills)
- Older Safari/Chrome versions

### v4 Browser Support (BREAKING)
- **Safari 16.4+**
- **Chrome 111+**
- **Firefox 128+**

**Why**: v4 uses modern CSS features:
- `@property` (CSS Houdini)
- `color-mix()`
- Native CSS cascade layers

**Impact**: If you need to support older browsers, **stay on v3** or provide fallbacks.

---

## 10. Testing & Validation Checklist

After migration:

### Visual Regression Testing
- [ ] All pages render correctly
- [ ] Color consistency (especially OKLCH colors)
- [ ] Dark mode works
- [ ] Responsive breakpoints work
- [ ] Custom components (`.btn`, `.form-control`, etc.) unchanged

### Build Testing
- [ ] `make css` produces output CSS
- [ ] `make css watch` works
- [ ] File size comparable to v3 (v4 is usually smaller)
- [ ] No console warnings

### Utility Class Testing
- [ ] All existing utility classes still work
- [ ] Opacity variants work (`bg-brand-500/50`)
- [ ] Color utilities work (`text-gray-400`, `bg-surface-100`)
- [ ] Custom utilities preserved

### Content Detection Testing
- [ ] `.templ` files scanned correctly
- [ ] `.go` files scanned correctly
- [ ] Unused classes purged in production

---

## 11. Rollback Plan

If migration fails:

```bash
# Restore v3 state
git checkout feature/tailwind-v4-migration
git reset --hard HEAD^  # Go back one commit

# Or restore from backup
mv tailwind.config.js.v3.backup tailwind.config.js
mv ai-chat/tailwind.config.ts.v3.backup ai-chat/tailwind.config.ts

# Reinstall v3 binary
curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/download/v3.4.17/tailwindcss-linux-x64
```

---

## 12. Timeline Estimate

| Phase | Duration | Risk |
|-------|----------|------|
| Research & Planning | 0.5 days | Low ✅ |
| Backup & Preparation | 0.25 days | Low ✅ |
| Automated Migration (try first) | 0.5 days | Medium ⚠️ |
| Manual Migration (if needed) | 1-2 days | Medium ⚠️ |
| Testing & Fixes | 1 day | High 🔴 |
| **Total** | **2.25-4.25 days** | |

---

## 13. Recommended Migration Strategy

### Strategy A: Incremental (Lower Risk)

1. **Phase 1**: Migrate `ai-chat` project first (smaller scope)
   - Use `npx @tailwindcss/upgrade` in ai-chat directory
   - Test thoroughly
   - Learn from any issues
   
2. **Phase 2**: Migrate main project
   - Apply lessons learned
   - Manual migration with more confidence

### Strategy B: All-at-Once (Faster, Higher Risk)

1. Run upgrade tool on both projects
2. Fix issues as they arise
3. Test everything together

**Recommendation**: Use **Strategy A** (incremental)

---

## 14. Key Resources

### Official Documentation
- **Upgrade Guide**: https://tailwindcss.com/docs/upgrade-guide
- **Theme Variables**: https://tailwindcss.com/docs/theme
- **Functions & Directives**: https://tailwindcss.com/docs/functions-and-directives
- **Standalone CLI**: https://tailwindcss.com/blog/standalone-cli

### Migration Tools
- **Upgrade Tool**: `npx @tailwindcss/upgrade`
- **AI Migration**: https://twshift.com/ (third-party)

### Community Resources
- **GitHub Discussions**: https://github.com/tailwindlabs/tailwindcss/discussions
- **Migration Examples**: Search for "tailwind v4 migration" on GitHub

---

## 15. Decision Matrix: Should You Migrate?

### ✅ Reasons to Migrate

1. **Better OKLCH support** - Your color system becomes simpler
2. **Faster builds** - Rust-based engine (Oxide) is significantly faster
3. **Smaller CSS output** - Better tree-shaking
4. **Modern CSS features** - Native cascade layers, better theming
5. **Future-proof** - v3 will eventually be deprecated
6. **Simpler config** - CSS-first is more intuitive

### ⚠️ Reasons to Wait

1. **Stable production app** - If no issues with v3, wait for v4 to mature
2. **Time constraints** - 2-4 days of developer time needed
3. **Browser support** - Need IE11 or Safari <16.4 support
4. **Plugin dependencies** - Using third-party plugins not yet v4 compatible
5. **Risk tolerance** - v4 is stable but still relatively new

### Recommendation

**Migrate when**:
- You have 1 week of development time available
- You can thoroughly test in staging
- You don't need old browser support
- You want to take advantage of improved OKLCH handling

**Wait if**:
- Current setup works perfectly
- Time is limited
- Major feature deadline approaching
- Need older browser support

---

## 16. Immediate Next Steps

If proceeding with migration:

1. **Create migration branch**
   ```bash
   git checkout -b feature/tailwind-v4-migration
   ```

2. **Backup current configs**
   ```bash
   cp tailwind.config.js tailwind.config.js.v3.backup
   cp ai-chat/tailwind.config.ts ai-chat/tailwind.config.ts.v3.backup
   cp modules/core/presentation/assets/css/main.css main.css.v3.backup
   ```

3. **Try automated tool first** (ai-chat)
   ```bash
   cd ai-chat
   npx @tailwindcss/upgrade
   npm install
   npm run dev  # Test
   ```

4. **Review generated changes**
   ```bash
   git diff
   ```

5. **If satisfied, commit**
   ```bash
   git add -A
   git commit -m "chore: migrate ai-chat to Tailwind v4"
   ```

6. **Repeat for main project** or do manual migration

---

## Summary

**TL;DR**:
- Tailwind v4 is a major architectural change (JS config → CSS config)
- Your OKLCH color system is **fully compatible** and **improves** in v4
- Migration involves: updating `@tailwind` directives, moving config to CSS `@theme`, and updating build commands
- Estimated effort: 2-4 days with testing
- Standalone CLI still supported
- **Biggest benefit for you**: Simpler OKLCH color management
- **Biggest challenge**: Rewriting two config files to CSS-first approach

**Confidence Level**: High ✅  
**Source Quality**: Official documentation + community validation  
**Recommendation**: Proceed with migration using incremental strategy, starting with ai-chat project

# Migration: main.css → @iotauz/design-tokens

This document shows the transformation from a monolithic `main.css` to a shareable package.

## Before: Monolithic main.css

**File**: `modules/core/presentation/assets/css/main.css`  
**Size**: ~40 KB (1172 lines)  
**Structure**: Single file with everything

```css
@import "tailwindcss";

@source "../../../../../modules/**/templates/**/*.templ";
/* ... more @source directives ... */

@font-face { /* Gilroy fonts */ }
@font-face { /* Inter fonts */ }

@theme {
  /* All 39 color tokens inline */
  --color-brand-500: oklch(58.73% 0.23 279.66);
  /* ... */
}

:root {
  /* All 170+ CSS variables inline */
  --primary-500: 58.73% 0.23 279.66;
  /* ... */
}

html.dark {
  /* Dark mode overrides */
}

@layer components {
  .btn { /* All button styles */ }
  .form-control { /* All form styles */ }
  /* ... all components ... */
}

@layer utilities {
  /* Custom utilities */
}

/* Keyframes */
@keyframes slide-in-right { /* ... */ }
```

### Problems with Monolithic Approach

❌ **Not Shareable**: Each project copies the entire file  
❌ **No Versioning**: Hard to track changes across projects  
❌ **Difficult Updates**: Must manually sync changes  
❌ **No Modularity**: Can't import parts separately  
❌ **Large File**: 40 KB, hard to navigate  
❌ **Tight Coupling**: Design tokens mixed with implementation  

## After: Shareable Package

**Package**: `@iotauz/design-tokens`  
**Size**: ~30 KB total (across 8 modular files)  
**Structure**: Organized, modular, shareable

### Package Structure

```
@iotauz/design-tokens/
├── index.css (210 B)           # Entry point
├── theme.css (189 B)           # Theme wrapper
├── base.css (9.9 KB)           # Fonts, variables, base styles
├── components.css (15 KB)      # Component classes
├── utilities.css (2.4 KB)      # Utilities, keyframes
└── tokens/
    ├── colors.css (2.1 KB)     # Color tokens
    ├── typography.css (73 B)   # Typography tokens
    └── spacing.css (168 B)     # Spacing tokens
```

### Consumer Usage

**Simple Import:**
```css
@import "@iotauz/design-tokens";
@source "./app/**/*.{js,ts,jsx,tsx}";
```

**Partial Import:**
```css
@import "tailwindcss";
@import "@iotauz/design-tokens/theme.css";
@import "@iotauz/design-tokens/base.css";
```

**With Extension:**
```css
@import "@iotauz/design-tokens";

@theme {
  --color-custom: oklch(50% 0.2 180);
}
```

### Benefits of Package Approach

✅ **Shareable**: `npm install @iotauz/design-tokens`  
✅ **Versioned**: Semantic versioning (1.0.0, 1.1.0, etc.)  
✅ **Easy Updates**: `npm update @iotauz/design-tokens`  
✅ **Modular**: Import full or partial  
✅ **Organized**: Logical file structure  
✅ **Documented**: README, examples, changelog  
✅ **Extendable**: Override or add tokens  
✅ **Maintainable**: Single source of truth  

## Migration Steps for Downstream Projects

### Step 1: Install Package

```bash
npm install @iotauz/design-tokens tailwindcss@^4.0.0
```

### Step 2: Update CSS Import

**Before:**
```css
/* Copy of main.css or relative import */
@import "../../path/to/main.css";

@source "./app/**/*.{js,ts,jsx,tsx}";
```

**After:**
```css
@import "@iotauz/design-tokens";

@source "./app/**/*.{js,ts,jsx,tsx}";
```

### Step 3: Move Custom Tokens (if any)

**Before:**
```css
@theme {
  --color-brand-500: oklch(58.73% 0.23 279.66);
  --color-custom: oklch(50% 0.2 180);  /* Custom token */
}
```

**After:**
```css
@import "@iotauz/design-tokens";

@theme {
  --color-custom: oklch(50% 0.2 180);  /* Only custom tokens */
}
```

### Step 4: Test and Verify

```bash
npm run build  # Verify build works
npm run dev    # Test in development
```

## Impact on IOTA SDK Main Project

The SDK itself can now import the shared package:

**Before:**
```css
/* modules/core/presentation/assets/css/main.css */
@import "tailwindcss";
@source "../../../../../modules/**/templates/**/*.templ";
/* ... 1172 lines of code ... */
```

**After:**
```css
/* modules/core/presentation/assets/css/main.css */
@import "@iotauz/design-tokens";
@source "../../../../../modules/**/templates/**/*.templ";
/* SDK-specific customizations only */
```

Or use relative path during development:
```css
@import "../../../../packages/design-tokens/index.css";
@source "../../../../../modules/**/templates/**/*.templ";
```

## Comparison Matrix

| Aspect | Before (main.css) | After (Package) |
|--------|------------------|----------------|
| **Distribution** | Copy file | NPM package |
| **Versioning** | Manual | Semantic versioning |
| **Updates** | Manual sync | `npm update` |
| **Size** | 40 KB (1 file) | 30 KB (8 files) |
| **Modularity** | Monolithic | Modular imports |
| **Documentation** | Inline comments | README + Examples |
| **Extensibility** | Edit file | Override via `@theme` |
| **Maintainability** | Difficult | Easy |
| **Consistency** | Manual | Automatic |
| **Discovery** | Search file | NPM registry |

## Upgrade Path

When design tokens are updated in the SDK:

### Before (Manual Sync)
1. Find changes in main.css
2. Copy changes to each project
3. Test each project individually
4. Hope nothing breaks

### After (Package Update)
1. SDK updates package version
2. Projects run `npm update @iotauz/design-tokens`
3. Review changelog for breaking changes
4. Test once, deploy everywhere

## Breaking Changes Management

### Version Strategy

- **Patch (1.0.x)**: Bug fixes, safe updates
- **Minor (1.x.0)**: New tokens/components, backwards compatible
- **Major (x.0.0)**: Breaking changes (token renames, removals)

### Example Update

```bash
# Safe update (patch/minor)
npm update @iotauz/design-tokens

# Major version (review changes first)
npm install @iotauz/design-tokens@2.0.0
```

## File Size Comparison

| Component | Before | After | Difference |
|-----------|--------|-------|------------|
| Theme tokens | Inline (2 KB) | tokens/*.css (2.4 KB) | +400 B |
| Base styles | Inline (10 KB) | base.css (9.9 KB) | -100 B |
| Components | Inline (15 KB) | components.css (15 KB) | Same |
| Utilities | Inline (2 KB) | utilities.css (2.4 KB) | +400 B |
| Entry point | main.css (40 KB) | index.css (210 B) | -39.8 KB |
| **Total** | **40 KB** | **30 KB** | **-10 KB (25% smaller)** |

*Note: Package is smaller due to modular structure and removal of duplicate code*

## Developer Experience

### Before
```bash
# Developer needs design tokens
1. Find main.css in SDK
2. Copy entire file to project
3. Edit to customize
4. Manually track SDK updates
5. Re-copy when updates available
```

### After
```bash
# Developer needs design tokens
1. npm install @iotauz/design-tokens
2. @import "@iotauz/design-tokens"
3. Extend via @theme if needed
4. npm update when new version available
```

## Conclusion

The migration from `main.css` to `@iotauz/design-tokens` provides:

✨ **Better Distribution**: NPM package vs file copy  
✨ **Version Control**: Semantic versioning  
✨ **Easier Maintenance**: Update once, use everywhere  
✨ **Modularity**: Import what you need  
✨ **Documentation**: Comprehensive guides  
✨ **Developer Experience**: Modern workflow  
✨ **Consistency**: Single source of truth  

The package is now ready for:
- ✅ Publishing to NPM
- ✅ Use in downstream projects
- ✅ Version management
- ✅ Collaborative development

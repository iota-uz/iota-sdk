# @iotauz/design-tokens - Package Summary

## Overview

A shareable design tokens package for the IOTA SDK that provides a complete design system based on Tailwind CSS v4. This package allows downstream projects to import and extend the IOTA SDK design system without copying files.

## Package Information

- **Name**: `@iotauz/design-tokens`
- **Version**: `1.0.0`
- **License**: MIT
- **Repository**: https://github.com/iota-uz/iota-sdk
- **Main Entry**: `index.css`

## Package Structure

```
@iotauz/design-tokens/
├── package.json          # NPM package configuration
├── README.md             # Comprehensive documentation
├── CHANGELOG.md          # Version history and changes
├── EXAMPLE.md            # Usage examples for consumers
├── validate.sh           # Package validation script
├── index.css             # Main entry point (full import)
├── theme.css             # @theme block with design tokens
├── base.css              # Base styles, fonts, CSS variables
├── components.css        # Component classes (.btn, .form-control, etc.)
├── utilities.css         # Utility classes and keyframes
└── tokens/
    ├── colors.css        # Color design tokens (39 colors)
    ├── typography.css    # Typography tokens
    └── spacing.css       # Spacing and sizing tokens
```

## What's Included

### Design Tokens (via `@theme`)
- **39 Color tokens** in OKLCH format
  - Base: white, black, black-800, black-950
  - Brand: brand-400 through brand-700
  - Gray: gray-50 through gray-950 (11 shades)
  - Semantic: red, green, pink, yellow, blue, purple
  - Success indicators
- **Typography**: Gilroy (sans-serif)
- **Spacing**: Modular scale from --size-00 to --size-5

### Base Styles
- Font-face declarations (Gilroy, Inter)
- CSS custom properties (170+ variables)
- Dark mode support (via `html.dark`)
- Global element styles
- Selection styles
- Popover styles

### Component Classes
- **Buttons**: `.btn` with 5 variants (primary, secondary, danger, outline, sidebar)
- **Forms**: `.form-control` with input, label, states
- **Dialogs**: `.dialog` with 4 animation variants
- **Tables**: `.table` with rounded corners
- **Tabs**: `.tab-slider` with multiple configurations
- **Sidebar**: Collapsible sidebar components
- **Loading states**: HTMX integration

### Utility Classes
- **Scrollbar**: `.hide-scrollbar`
- **Transitions**: `.no-transition`
- **Sliders**: `.slider-thumb`
- **iOS fixes**: Dynamic viewport height
- **Lazy loading**: Fade-in animations

### Animations
- 8 slide animations (in/out, up/down, left/right)
- Scale animations
- Dot flashing (loading indicators)

## Usage Patterns

### Full Import (Recommended)
```css
@import "@iotauz/design-tokens";
@source "./app/**/*.{js,ts,jsx,tsx}";
```

### Partial Import
```css
@import "tailwindcss";
@import "@iotauz/design-tokens/theme.css";
@import "@iotauz/design-tokens/base.css";
```

### Extension
```css
@import "@iotauz/design-tokens";
@theme {
  --color-custom: oklch(50% 0.2 180);
}
```

## Benefits

1. **No File Copying**: Direct package import
2. **Consistent Design**: Same tokens across all projects
3. **Easy Updates**: `npm update @iotauz/design-tokens`
4. **Selective Import**: Import only what you need
5. **Extendable**: Override or add custom tokens
6. **Versioned**: Semantic versioning for upgrades
7. **Documented**: Comprehensive README and examples

## File Sizes

| File | Size | Purpose |
|------|------|---------|
| index.css | 210 B | Entry point (imports only) |
| theme.css | 189 B | Theme wrapper (imports tokens) |
| tokens/colors.css | 2.1 KB | Color design tokens |
| tokens/typography.css | 73 B | Typography tokens |
| tokens/spacing.css | 168 B | Spacing tokens |
| base.css | 9.9 KB | Base styles, fonts, variables |
| components.css | 15 KB | Component classes |
| utilities.css | 2.4 KB | Utility classes, keyframes |
| **Total** | **~30 KB** | Complete package (unminified) |

## Dependencies

### Peer Dependencies
- `tailwindcss`: `^4.0.0`

### No Runtime Dependencies
This is a CSS-only package with no JavaScript runtime dependencies.

## Browser Support

- Chrome 111+ (OKLCH support)
- Firefox 113+ (OKLCH support)
- Safari 16.4+ (OKLCH support)
- Edge 111+ (OKLCH support)

For older browsers, consider using PostCSS plugins to transform OKLCH colors.

## Publishing Checklist

Before publishing to npm:

- [ ] Run validation: `bash validate.sh`
- [ ] Update version in `package.json`
- [ ] Update `CHANGELOG.md`
- [ ] Verify all files are included in `files` array
- [ ] Test installation in consumer project
- [ ] Review README and EXAMPLE docs
- [ ] Commit all changes
- [ ] Tag release: `git tag v1.0.0`
- [ ] Push: `git push && git push --tags`
- [ ] Publish: `npm publish --access public`

## Publishing Commands

```bash
# First time setup
npm login

# Publish package (ensure you're in packages/design-tokens/)
npm publish --access public

# Update package
npm version patch  # or minor, or major
npm publish
```

## Installation in Consumer Projects

```bash
npm install @iotauz/design-tokens tailwindcss@^4.0.0
```

## Local Development/Testing

For local development before publishing:

```bash
# In iota-sdk/packages/design-tokens
npm link

# In consumer project
npm link @iotauz/design-tokens
```

Or use file path:
```json
{
  "dependencies": {
    "@iotauz/design-tokens": "file:../iota-sdk/packages/design-tokens"
  }
}
```

## Future Enhancements

Potential additions for future versions:

1. **Dark mode variants**: Additional dark mode color tokens
2. **Component variants**: More button/form styles
3. **Animation presets**: Additional keyframe animations
4. **Grid system**: Custom grid utilities
5. **Typography scale**: More font-size tokens
6. **Breakpoint tokens**: Responsive design tokens
7. **Icon system**: SVG icon tokens
8. **Accessibility**: Focus state improvements

## Support

- **Issues**: https://github.com/iota-uz/iota-sdk/issues
- **Documentation**: See README.md and EXAMPLE.md
- **Repository**: https://github.com/iota-uz/iota-sdk

## License

MIT License - See LICENSE file in repository root

---

**Package Created**: 2024-02-01  
**Last Updated**: 2024-02-01  
**Status**: ✅ Ready for publishing

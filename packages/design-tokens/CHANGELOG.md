# Changelog

All notable changes to @iotauz/design-tokens will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2024-02-01

### Added
- Initial release of @iotauz/design-tokens package
- Complete OKLCH color palette with brand, gray, and semantic colors
- Tailwind CSS v4 theme configuration via `@theme` directive
- Font-face declarations for Gilroy and Inter fonts
- Comprehensive CSS variables for custom components
- Pre-built component styles:
  - `.btn` with variants (primary, secondary, danger, outline, sidebar)
  - `.form-control` with input, label, and state management
  - `.dialog` with animation variants
  - `.table` with rounded corners
  - `.tab-slider` with multiple configurations
  - Sidebar components with collapse/expand states
- Custom utility classes:
  - `.hide-scrollbar`
  - `.no-transition`
  - `.slider-thumb`
- Dark mode support via `html.dark` class
- Animation keyframes for slides, scale, and dot-flashing
- iOS Safari viewport height fix
- Lazy loading component styles
- Modular package structure:
  - `index.css` - Full import
  - `theme.css` - Theme tokens only
  - `base.css` - Base styles and fonts
  - `components.css` - Component classes
  - `utilities.css` - Utility classes
  - `tokens/colors.css` - Color design tokens
  - `tokens/typography.css` - Typography tokens
  - `tokens/spacing.css` - Spacing tokens
- Comprehensive README with usage examples
- Example usage guide (EXAMPLE.md)
- MIT License

### Color Tokens
- Base: white, black, black-800, black-950
- Brand: brand-400, brand-500, brand-600, brand-650, brand-700
- Gray: gray-50 through gray-950 (11 shades)
- Red: red-100, red-200, red-300, red-500, red-600, red-700
- Green: green-50, green-100, green-200, green-500, green-600
- Pink: pink-500, pink-600
- Yellow: yellow-500
- Blue: blue-500, blue-600
- Purple: purple-500
- Semantic: success, on-success

### Design Principles
- Mobile-first responsive design
- Accessibility-focused color contrasts (OKLCH color space)
- Consistent spacing scale (--size-00 through --size-5)
- Semantic color naming for better maintainability
- CSS custom properties for runtime theming
- Tailwind utilities for rapid development

### Browser Support
- Modern browsers with OKLCH color space support
- Chrome 111+
- Firefox 113+
- Safari 16.4+
- Edge 111+

### Breaking Changes
None - Initial release

### Migration Guide
For projects currently using main.css from iota-sdk, see EXAMPLE.md for migration steps.

---

## Template for Future Releases

## [Unreleased]

### Added
- New features go here

### Changed
- Changes to existing functionality

### Deprecated
- Features that will be removed in future versions

### Removed
- Features that have been removed

### Fixed
- Bug fixes

### Security
- Security improvements

---

## Versioning Guidelines

- **Major (x.0.0)**: Breaking changes to API, token names, or component structure
- **Minor (0.x.0)**: New features, new components, new tokens (backwards compatible)
- **Patch (0.0.x)**: Bug fixes, documentation updates (backwards compatible)

## Release Process

1. Update version in package.json
2. Update CHANGELOG.md with changes
3. Commit changes: `git commit -m "chore: release v{version}"`
4. Tag release: `git tag v{version}`
5. Push: `git push && git push --tags`
6. Publish: `npm publish`

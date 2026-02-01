# IOTA SDK Packages

This directory contains shareable packages that can be published to NPM and used by downstream projects.

## Available Packages

### [@iotauz/design-tokens](./design-tokens/)

**Version**: 1.0.0  
**Status**: ✅ Ready for Publishing

A shareable design tokens package that provides the complete IOTA SDK design system with Tailwind CSS v4 support.

#### Features
- 39 OKLCH color tokens
- Typography tokens (Gilroy, Inter)
- Spacing scale
- Pre-built components (.btn, .form-control, .dialog, etc.)
- Utility classes
- Dark mode support
- Comprehensive documentation

#### Quick Start

```bash
npm install @iotauz/design-tokens tailwindcss@^4.0.0
```

```css
@import "@iotauz/design-tokens";
@source "./app/**/*.{js,ts,jsx,tsx}";
```

```jsx
<button className="btn btn-primary">Click me</button>
```

#### Documentation
- [README](./design-tokens/README.md) - Complete documentation
- [QUICKSTART](./design-tokens/QUICKSTART.md) - 2-minute setup
- [EXAMPLE](./design-tokens/EXAMPLE.md) - Usage examples
- [MIGRATION](./design-tokens/MIGRATION.md) - Migration guide

---

## Publishing Packages

### Local Testing

```bash
cd packages/design-tokens
npm link

# In consumer project
npm link @iotauz/design-tokens
```

### Publishing to NPM

```bash
cd packages/design-tokens
npm login
npm publish --access public
```

### Updating Versions

```bash
npm version patch  # Bug fixes (1.0.0 → 1.0.1)
npm version minor  # New features (1.0.0 → 1.1.0)
npm version major  # Breaking changes (1.0.0 → 2.0.0)
npm publish
```

---

## Package Guidelines

When creating new packages in this directory:

1. **Naming**: Use `@iotauz/package-name` format
2. **Structure**: Follow modular architecture
3. **Documentation**: Include comprehensive README
4. **Versioning**: Follow semantic versioning
5. **Testing**: Validate before publishing
6. **License**: Use MIT license

---

## Contributing

See the main [IOTA SDK repository](https://github.com/iota-uz/iota-sdk) for contribution guidelines.

# Quick Start Guide - @iotauz/design-tokens

Get started with @iotauz/design-tokens in 2 minutes! 🚀

## Step 1: Install

```bash
npm install @iotauz/design-tokens tailwindcss@^4.0.0
```

## Step 2: Create CSS File

Create `app/globals.css` (or your main CSS file):

```css
@import "@iotauz/design-tokens";

@source "./app/**/*.{js,ts,jsx,tsx}";
@source "./components/**/*.{js,ts,jsx,tsx}";
```

## Step 3: Use It!

### In HTML/JSX

```jsx
// Use pre-built components
<button className="btn btn-primary">Click me</button>

// Use Tailwind utilities with design tokens
<div className="bg-brand-500 text-white p-4 rounded-lg">
  Hello IOTA SDK!
</div>

// Use form controls
<div className="form-control">
  <label className="form-control-label">Email</label>
  <input type="email" className="form-control-input" />
</div>
```

### In CSS

```css
.my-component {
  background-color: oklch(var(--primary-500));
  padding: var(--size-3);
  border-radius: var(--size-1);
  transition: transform 200ms var(--ease-2);
}
```

## That's It! 🎉

You now have access to:
- ✅ 39 color tokens as Tailwind utilities
- ✅ Pre-built components (.btn, .form-control, etc.)
- ✅ 170+ CSS variables for custom styling
- ✅ Dark mode support (add `class="dark"` to `<html>`)
- ✅ Animations and utilities

## Next Steps

- 📖 Read [README.md](./README.md) for full documentation
- 💡 Check [EXAMPLE.md](./EXAMPLE.md) for more examples
- 🎨 Browse color palette in [tokens/colors.css](./tokens/colors.css)

## Common Use Cases

### Button Variants
```jsx
<button className="btn btn-primary">Primary</button>
<button className="btn btn-secondary">Secondary</button>
<button className="btn btn-danger">Danger</button>
<button className="btn btn-primary-outline">Outline</button>
```

### Form Example
```jsx
<form className="space-y-4">
  <div className="form-control">
    <input type="text" className="form-control-input" placeholder="Name" />
  </div>
  <button type="submit" className="btn btn-primary">Submit</button>
</form>
```

### Dark Mode Toggle
```jsx
<button 
  onClick={() => document.documentElement.classList.toggle('dark')}
  className="btn btn-secondary"
>
  Toggle Dark Mode
</button>
```

### Custom Theme Override
```css
@import "@iotauz/design-tokens";

@theme {
  /* Override brand color */
  --color-brand-500: oklch(60% 0.25 280);
}
```

## Need Help?

- 📚 Full docs: [README.md](./README.md)
- 💻 Examples: [EXAMPLE.md](./EXAMPLE.md)
- 🐛 Issues: https://github.com/iota-uz/iota-sdk/issues

Happy coding! 🎨✨

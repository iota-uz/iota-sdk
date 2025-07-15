# Alpine Tooltip Component

A powerful tooltip directive for Alpine.js 3.x powered by Tippy.js.

> Based on [Alpine Tooltip](https://github.com/ryangjchandler/alpine-tooltip) by Ryan Chandler.

## Overview

This package provides an `x-tooltip` directive for Alpine.js that allows you to easily add tooltips to your elements. It's built on top of the robust Tippy.js library, providing a wide range of customization options through Alpine modifiers.

## Directory Structure

```
alpine-tooltip/
├── dist/
│   ├── alpine-tooltip.js      # Unminified JavaScript source
│   └── alpine-tooltip.min.js   # Minified JavaScript source
├── css/
│   ├── tippy.css              # Core Tippy.js styles
│   └── tippy-animations.css   # Animation styles
└── docs/
    └── examples.html          # Usage examples
```

## Installation

### Method 1: Direct Download (Recommended for this package)

1. Include the JavaScript file in your HTML:
```html
<script src="path/to/alpine-tooltip/dist/alpine-tooltip.min.js" defer></script>
```

2. Include the CSS files:
```html
<link rel="stylesheet" href="path/to/alpine-tooltip/css/tippy.css">
<!-- Optional: Include animations -->
<link rel="stylesheet" href="path/to/alpine-tooltip/css/tippy-animations.css">
```

3. Make sure Alpine.js is loaded after the tooltip plugin:
```html
<script defer src="https://cdn.jsdelivr.net/npm/alpinejs@3.x.x/dist/cdn.min.js"></script>
```

### Method 2: CDN

```html
<!-- Alpine Tooltip Plugin -->
<script src="https://cdn.jsdelivr.net/npm/@ryangjchandler/alpine-tooltip@1.x.x/dist/cdn.min.js" defer></script>

<!-- Tippy.js CSS -->
<link rel="stylesheet" href="https://unpkg.com/tippy.js@6/dist/tippy.css" />

<!-- Alpine.js -->
<script defer src="https://cdn.jsdelivr.net/npm/alpinejs@3.x.x/dist/cdn.min.js"></script>
```

### Method 3: NPM (If using a build tool)

```bash
npm install @ryangjchandler/alpine-tooltip
```

Then in your JavaScript:
```javascript
import Alpine from 'alpinejs'
import Tooltip from '@ryangjchandler/alpine-tooltip'

Alpine.plugin(Tooltip)
Alpine.start()
```

## Basic Usage

### Simple Tooltip

```html
<div x-data="{ message: 'Hello, World!' }">
    <button x-tooltip="message">Hover me!</button>
</div>
```

### Raw Text Tooltip

```html
<button x-data x-tooltip.raw="This is a tooltip!">
    Hover for info
</button>
```

### Dynamic Tooltip

```html
<div x-data="{ enabled: true, tooltip: 'Click to toggle' }">
    <button x-tooltip="enabled ? tooltip : ''" @click="enabled = !enabled">
        Toggle Tooltip
    </button>
</div>
```

## Modifiers

The `x-tooltip` directive supports various modifiers to customize behavior:

### Duration
Control the transition duration (in milliseconds):
```html
<button x-tooltip.duration.300="message">300ms duration</button>
```

### Delay
Set show/hide delay:
```html
<!-- Same delay for show and hide -->
<button x-tooltip.delay.500="message">500ms delay</button>

<!-- Different delays: show 500ms, hide immediately -->
<button x-tooltip.delay.500-0="message">Custom delays</button>
```

### Cursor Following
Make the tooltip follow the cursor:
```html
<!-- Follow cursor within the element -->
<button x-tooltip.cursor="message">Follow cursor</button>

<!-- Follow only horizontally -->
<button x-tooltip.cursor.x="message">Follow X axis</button>

<!-- Show at initial cursor position -->
<button x-tooltip.cursor.initial="message">Initial position</button>
```

### Trigger Events
Change when the tooltip shows:
```html
<button x-tooltip.on.click="message">Click to show</button>
<button x-tooltip.on.focus="message">Focus to show</button>
<button x-tooltip.on.mouseenter="message">Hover to show</button>
```

### Appearance
Customize the tooltip appearance:
```html
<!-- Remove arrow -->
<button x-tooltip.arrowless="message">No arrow</button>

<!-- Allow HTML content -->
<button x-tooltip.html="'<strong>Bold</strong> text'">HTML content</button>

<!-- Change theme -->
<button x-tooltip.theme.light="message">Light theme</button>

<!-- Set max width -->
<button x-tooltip.max-width.200="message">Limited width</button>
```

### Placement
Control tooltip position:
```html
<button x-tooltip.placement.top="message">Top</button>
<button x-tooltip.placement.bottom="message">Bottom</button>
<button x-tooltip.placement.left="message">Left</button>
<button x-tooltip.placement.right="message">Right</button>
<button x-tooltip.placement.top-start="message">Top Start</button>
<button x-tooltip.placement.bottom-end="message">Bottom End</button>

<!-- Disable auto-flipping -->
<button x-tooltip.placement.top.no-flip="message">Always on top</button>
```

### Interactive Tooltips
Allow users to interact with the tooltip:
```html
<!-- Basic interactive -->
<button x-tooltip.interactive="message">Interactive</button>

<!-- With custom border -->
<button x-tooltip.interactive.border.20="message">20px border</button>

<!-- With debounce -->
<button x-tooltip.interactive.debounce.300="message">300ms debounce</button>
```

### Animations
Apply different animations:
```html
<button x-tooltip.animation.scale="message">Scale animation</button>
<button x-tooltip.animation.shift-away="message">Shift away</button>
<button x-tooltip.animation.shift-toward="message">Shift toward</button>
<button x-tooltip.animation.perspective="message">Perspective</button>
```

## Advanced Usage

### Magic Function $tooltip

Show tooltips programmatically:

```html
<button @click="$tooltip('Quick tooltip!')">
    Click for tooltip
</button>

<!-- With custom timeout -->
<button @click="$tooltip('Shows for 5 seconds', { timeout: 5000 })">
    Custom timeout
</button>

<!-- With Tippy configuration -->
<button @click="$tooltip('Custom tooltip', { 
    placement: 'right',
    animation: 'scale',
    delay: [200, 0]
})">
    Configured tooltip
</button>
```

### Using HTML Elements as Content

```html
<div x-data="{ user: 'John Doe' }">
    <template x-ref="userCard">
        <div class="p-4 bg-white rounded shadow">
            <h3 class="font-bold" x-text="user"></h3>
            <p>Click to see profile</p>
        </div>
    </template>

    <button x-tooltip="{
        content: () => $refs.userCard.innerHTML,
        allowHTML: true,
        interactive: true,
        appendTo: $root
    }">
        User Info
    </button>
</div>
```

### Complex Interactive Tooltip

```html
<div x-data="{ count: 0 }">
    <template x-ref="counter">
        <div class="p-4">
            <p>Count: <span x-text="count"></span></p>
            <button @click="count++" class="px-2 py-1 bg-blue-500 text-white rounded">
                Increment
            </button>
        </div>
    </template>

    <button x-tooltip="{
        content: () => $refs.counter.innerHTML,
        allowHTML: true,
        interactive: true,
        trigger: 'click',
        appendTo: $root,
        placement: 'bottom',
        theme: 'light'
    }">
        Click for counter
    </button>
</div>
```

## Setting Default Properties

Configure default properties for all tooltips:

```javascript
import Alpine from 'alpinejs'
import Tooltip from '@ryangjchandler/alpine-tooltip'

Alpine.plugin(
    Tooltip.defaultProps({
        delay: 100,
        duration: 200,
        theme: 'dark',
        placement: 'top',
        animation: 'scale'
    })
)

Alpine.start()
```

## Complete Example

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Alpine Tooltip Example</title>
    
    <!-- Tippy.js CSS -->
    <link rel="stylesheet" href="css/tippy.css">
    <link rel="stylesheet" href="css/tippy-animations.css">
    
    <!-- Alpine Tooltip -->
    <script src="dist/alpine-tooltip.min.js" defer></script>
    
    <!-- Alpine.js -->
    <script defer src="https://cdn.jsdelivr.net/npm/alpinejs@3.x.x/dist/cdn.min.js"></script>
    
    <style>
        .demo-button {
            padding: 8px 16px;
            margin: 4px;
            background: #3b82f6;
            color: white;
            border: none;
            border-radius: 4px;
            cursor: pointer;
        }
        .demo-button:hover {
            background: #2563eb;
        }
    </style>
</head>
<body>
    <div x-data="{ 
        tooltip: 'This is a tooltip!',
        dynamicText: 'Dynamic content',
        showTooltip: true 
    }" class="p-8">
        
        <h2 class="text-2xl mb-4">Alpine Tooltip Examples</h2>
        
        <!-- Basic tooltip -->
        <button x-tooltip="tooltip" class="demo-button">
            Basic Tooltip
        </button>
        
        <!-- Raw text -->
        <button x-tooltip.raw="Raw text tooltip" class="demo-button">
            Raw Text
        </button>
        
        <!-- With modifiers -->
        <button x-tooltip.placement.right.duration.1000.delay.200="tooltip" class="demo-button">
            Right + Slow + Delayed
        </button>
        
        <!-- Interactive with HTML -->
        <button x-tooltip.interactive.html="'<div class=&quot;p-2&quot;><strong>Interactive</strong><br>You can hover this!</div>'" class="demo-button">
            Interactive HTML
        </button>
        
        <!-- Click trigger -->
        <button x-tooltip.on.click.placement.bottom="'Click me!'" class="demo-button">
            Click Trigger
        </button>
        
        <!-- Cursor following -->
        <button x-tooltip.cursor.theme.light="'Following your cursor!'" class="demo-button">
            Cursor Follow
        </button>
        
        <!-- Programmatic -->
        <button @click="$tooltip('Programmatic tooltip!', { placement: 'top' })" class="demo-button">
            Use $tooltip
        </button>
        
        <!-- Toggle -->
        <div class="mt-4">
            <label>
                <input type="checkbox" x-model="showTooltip">
                Enable tooltip
            </label>
            <button x-tooltip="showTooltip ? 'Tooltip enabled!' : ''" class="demo-button ml-2">
                Conditional Tooltip
            </button>
        </div>
    </div>
</body>
</html>
```

## Tips and Best Practices

1. **Performance**: For many tooltips, consider using the `raw` modifier for static content to avoid unnecessary reactivity.

2. **Accessibility**: Tooltips are automatically accessible. They use `aria-describedby` and proper focus management.

3. **Mobile**: Tooltips work on mobile devices. On touch devices, they typically show on tap.

4. **Z-index**: Tooltips have a high z-index by default. If you have z-index conflicts, you can configure this in the Tippy options.

5. **Memory**: Tooltips are automatically cleaned up when elements are removed from the DOM.

## Browser Support

- Chrome (latest)
- Firefox (latest)
- Safari (latest)
- Edge (latest)
- Mobile browsers (iOS Safari, Chrome Android)

## Troubleshooting

### Tooltip not showing
- Ensure Alpine.js is loaded after the tooltip plugin
- Check that the CSS file is loaded
- Verify the element has the `x-data` directive (either on the element or a parent)

### Positioning issues
- Make sure the parent container doesn't have `overflow: hidden`
- Check z-index conflicts with other elements
- Try using the `no-flip` modifier if auto-positioning is problematic

### Interactive tooltips not working
- Ensure you've added the `interactive` modifier
- Check that `allowHTML` is true if using HTML content
- Verify the `appendTo` option is set correctly for Alpine components

## Credits

This component is based on [Alpine Tooltip](https://github.com/ryangjchandler/alpine-tooltip) by [Ryan Chandler](https://github.com/ryangjchandler).

### Key Features from Original
- Plugin system integration with Alpine.js
- Comprehensive modifier system for Tippy.js configuration
- Magic `$tooltip` function for programmatic tooltips
- Support for dynamic content and HTML templates
- Automatic cleanup and memory management

### Versioning
This component follows the same versioning as the original Alpine Tooltip plugin to maintain compatibility.

## License

This component uses the Alpine Tooltip plugin by Ryan Chandler, which is licensed under the MIT License.

Tippy.js is also licensed under the MIT License.

## Links

- [Original Repository](https://github.com/ryangjchandler/alpine-tooltip)
- [NPM Package](https://www.npmjs.com/package/@ryangjchandler/alpine-tooltip)
- [Tippy.js Documentation](https://atomiks.github.io/tippyjs/)
- [Alpine.js Documentation](https://alpinejs.dev/)


# Solid UI

This workspace builds the public `@iota-uz/sdk/solid-ui` component library and
hosts deterministic visual parity checks against the templ component library.

Application code imports components from the public package. A standalone
application imports the compiled visual contract once at its root:

```ts
import { Button, Input } from '@iota-uz/sdk/solid-ui'
import '@iota-uz/sdk/solid-ui/standalone.css'
```

Add each component and meaningful state to `src/catalog.ts`, render it from
`App.tsx`, and keep its screenshots exact in both light and dark themes. The
Playwright lane fixes Chromium rendering, viewport, device scale, locale,
timezone, color profile, animation, and caret behavior. Its default tolerance
is zero pixels.

```bash
just solid-ui dev
just solid-ui build
just solid-ui check
just solid-ui vr
just solid-ui vr-update
```

Linux x86_64 screenshots under `vr/baselines/linux` are the canonical review
artifacts. Darwin and Windows baselines are local and ignored. Only update a
canonical baseline after reviewing the expected, actual, and diff images.

# @iota-uz/sdk

This is the single public JavaScript distribution for iota-sdk. Its bounded
subpath APIs are built from separate internal workspace modules:

- `@iota-uz/sdk/client-host`
- `@iota-uz/sdk/solid`
- `@iota-uz/sdk/solid-ui`
- `@iota-uz/sdk/solid-ui/standalone.css`
- `@iota-uz/sdk/solid-ui/embedded-css` (a string module: the kit's utilities for a page that already carries the SDK stylesheet; consumed by the runtime host bundle, not by application code)
- `@iota-uz/sdk/styles.css`
- `@iota-uz/sdk/lens`
- `@iota-uz/sdk/lens/styles.css`
- `@iota-uz/sdk/identity`
- `@iota-uz/sdk/testing` (Playwright Node process only; optional `@playwright/test` peer)
- `@iota-uz/sdk/runtime`, `@iota-uz/sdk/runtime/server`, `@iota-uz/sdk/runtime/client`,
  `@iota-uz/sdk/runtime/testing` and the `iota-ext` command: the extension runtime for authoring packages
  (`docs/extensions/authoring.md`). It ships from the release that contains `dist/runtime`; `iota-ext init` pins that
  exact version in the new package. The test host is `runtime/testing`, not `testing`.

Version 0.5 is a clean break: legacy root, BiChat, applet, Tailwind, and asset
exports are intentionally absent. Solid is the standard client-route renderer;
React and ReactDOM remain optional peers for migration compatibility;
Lens implementation dependencies are contained in lazy Lens chunks and are not
loaded by client-host imports.

Solid routes rendered inside the standard iota shell can use the component
exports directly. Standalone applications and the component gallery import the
compiled stylesheet once at their root:

```ts
import { Button, Input } from '@iota-uz/sdk/solid-ui'
import '@iota-uz/sdk/solid-ui/standalone.css'
```

Browser test projects import synchronization helpers and lifecycle/scenario fixture adapters from `@iota-uz/sdk/testing`. Helpers attach observers before actions and reject missing requests, HTMX refusal and missing readiness. Component hosts reuse explicit font/chart readiness while preserving existing approved screenshot baselines. The source workflow and adapter contracts are documented in `web/testing/README.md`.

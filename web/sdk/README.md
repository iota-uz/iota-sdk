# @iota-uz/sdk

This is the single public JavaScript distribution for iota-sdk. Its bounded
subpath APIs are built from separate internal workspace modules:

- `@iota-uz/sdk/client-host`
- `@iota-uz/sdk/solid`
- `@iota-uz/sdk/styles.css`
- `@iota-uz/sdk/lens`
- `@iota-uz/sdk/lens/styles.css`
- `@iota-uz/sdk/identity`

Version 0.5 is a clean break: legacy root, BiChat, applet, Tailwind, and asset
exports are intentionally absent. Solid is the standard client-route renderer;
React and ReactDOM remain optional peers for migration compatibility;
Lens implementation dependencies are contained in lazy Lens chunks and are not
loaded by client-host imports.

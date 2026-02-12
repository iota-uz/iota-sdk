# BiChat Web Frontend

React + TypeScript frontend for BiChat module, built with Vite.

## Development

```bash
# Install dependencies
pnpm install

# Start dev server
pnpm run dev

# Build for production
pnpm run build

# Preview production build
pnpm run preview
```

For embedded applet HMR through the Go server, run from repo root:

```bash
just dev bichat
```

## SDK Dependency Policy

- Pin `@iota-uz/sdk` to an exact npm version in `package.json`.
- Do not use `file:`, `link:`, or `workspace:` overrides for `@iota-uz/sdk`.
- Local SDK changes are picked up in dev by `just dev bichat` without changing dependency specs.

## Architecture

- **Context**: Reads session data from `window.__BICHAT_CONTEXT__` injected by Go backend
- **RPC**: Uses applet RPC for request/response calls
- **Routing**: React Router with `/` (sessions list) and `/session/:id` (chat interface)
- **UI**: Imports shared components from `@iota-uz/sdk/bichat`

## Integration with Go Backend

The app expects the Go backend to:

1. Serve `index.html` and inject initial context
2. Inject `window.__BICHAT_CONTEXT__` with user/tenant/config data
3. Serve built assets from `../assets/dist/`
4. Provide RPC endpoint at `config.rpcUIEndpoint`
5. Provide stream endpoint at `config.streamEndpoint`

## Build Output

Vite builds to `../assets/dist/`:
- `main.js` - Application bundle
- `main.css` - Styles
- `assets/` - Static assets

These files are served by the Go backend's applet controller.

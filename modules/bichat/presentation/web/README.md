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

## Architecture

- **Context**: Reads session data from `window.__BICHAT_CONTEXT__` injected by Go backend
- **GraphQL**: Uses urql client for queries/mutations
- **Routing**: React Router with `/` (sessions list) and `/session/:id` (chat interface)
- **UI**: Imports shared components from `@iota-uz/sdk/bichat`

## Integration with Go Backend

The app expects the Go backend to:

1. Serve `index.html` with `#root` div
2. Inject `window.__BICHAT_CONTEXT__` with user/tenant/config data
3. Serve built assets from `../assets/dist/`
4. Provide GraphQL endpoint at `config.graphqlEndpoint`

## Build Output

Vite builds to `../assets/dist/`:
- `main.js` - Application bundle
- `main.css` - Styles
- `assets/` - Static assets

These files are served by the Go backend's applet controller.

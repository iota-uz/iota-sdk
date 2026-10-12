# Documentation
Static Astro pages with native Solid MDX components. No React runtime or React dependency is installed.

- `pnpm install --frozen-lockfile`
- `pnpm dev`: local authoring on port 4000.
- `pnpm build`: static `out/` and Pagefind search.
- `pnpm start`: preview the built site.
- `pnpm typecheck`, `pnpm test:browser`, `pnpm check:react-free`.

MDX content remains in `content/`; metadata ordering lives in `_meta.js`.
The static shell is `src/pages/[...slug].astro`; custom MDX components use Solid JSX.
Search, theme, tabs and mobile navigation use native browser events.
URLs keep the existing base path and static HTML output for the existing Caddy/GitHub Pages hosts.

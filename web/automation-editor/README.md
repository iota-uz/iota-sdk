# @iota-uz/sdk/automation-editor

The automation graph editor (issue #1331): a canvas with edges by outcomes, the ⌘K palette, the step
panel with forms from the catalog, the Validation dock, undo/redo, copy/paste, Tidy, touch, and a
read-only outline on phones. It holds no product code: the host passes the catalog and its operations
(`EditorApi`, `save`, `saveLayout`, `reload`) and owns routes, storage and permissions.

```ts
import { Editor } from '@iota-uz/sdk/automation-editor'
import '@iota-uz/sdk/automation-editor/styles.css' // next to the SDK stylesheet and solid-ui
```

`Editor` and `Canvas` render a skeleton and load the canvas chunk (`@xyflow/system` 1.0.0 and d3,
bundled) on first render; `loadEditor()` preloads it. `Outline` and the pure helpers (`GRID`, `snap`,
`tidyLayout`, `ensureLayout`, `placeNew`, history, clipboard, graph operations) cost no canvas code.
Only `solid-js` and the SDK's own `solid-ui` subpath stay external.

## Decisions in code

- `src/xy/adapter.ts` is the only module that imports `@xyflow/system`; `src/xy/dom.ts` holds its DOM
  contract (`iota-flow__node`, `iota-flow__handle`, `nopan`, `data-id`, `data-nodeid`, `data-handleid`).
  No xyflow type is in the public declarations (`scripts/prune-types.mjs` fails the build otherwise).
- The canvas is controlled: it emits `onChange`, `onMove` (once per drag end, snapped), `onDrop`,
  `onMenu`, `onSelect`. One edge per `(node, outcome)`: a new connection replaces the old one.
- Card geometry is computed (`src/geometry.ts`): width from outcomes, fixed height, handles declared
  from geometry; the DOM is not measured. Only cards and edges in view are mounted.
- Layout is apart from the graph: a move is a history step and is saved by `saveLayout` 500 ms after
  the last change; it never makes the graph dirty or creates a version.
- Tidy lays out left to right (an accepted deviation from the standard's top-to-bottom).
- Validation results belong to the snapshot they checked: they are rebased onto the current draft by
  item id, an edit invalidates a check in flight, and Save checks exactly the snapshot it sends.
- Layout saves run one at a time; when one ends, the layout on screen is compared with what was saved
  and the latest snapshot is sent again if they differ.
- An unavailable connection (ED10): the host's `validate` returns an error with
  `code: "connection_unavailable"` whose message names the connection; a catalog type with
  `connected: false` is reported by the editor itself. Such steps get a "No connection" badge.

## Keys (K1)

| Key | Result |
| --- | --- |
| ⌘K / Ctrl+K | Palette of steps and triggers |
| ← ↑ → ↓ (on a step) | Focus the nearest step in that direction |
| Enter | Open the step panel, focus inside |
| Esc | Close the top layer: menu, palette, panel, selection |
| Delete / Backspace | Delete the selection, focus the parent step |
| Shift+F10 / ContextMenu | Step or canvas menu; "Connect <outcome> to…" opens the target palette |
| Alt+arrows | Move the selected steps by one grid cell (one history step per run) |
| Alt while dragging | Drag without snapping to the grid |
| ⌘C · ⌘V · ⌘D | Copy · paste (clipboard JSON, between tabs) · duplicate |
| ⌘A | Select all |
| ⌘Z · ⇧⌘Z (Ctrl+Y) | Undo · redo |
| ⌘S | Save |
| ⌘Enter | Validate |
| ⇧1 · ⇧0 | Fit · zoom to 100 % |
| ? | Key map (on touch it starts with gestures) |

Shortcuts do nothing while focus is in a field (⌘Z there is the field's own undo), except ⌘S.

## Budgets and checks

| Check | Limit | Where |
| --- | --- | --- |
| Canvas chunk (minified, gzip, without solid-js) | ≤ 50 KB | `scripts/check-chunk.mjs` in `build:package` |
| Mount, 200 steps / 500 edges, 4× CPU throttle | ≤ 150 ms | `e2e/bench.spec.ts` |
| Whole-graph replacement (undo), same graph | ≤ 60 ms (median of 10) | `e2e/bench.spec.ts` |
| Drag: long tasks · p95 frame | 0 · ≤ 17 ms | `e2e/bench.spec.ts` |
| `@iota-uz/sdk/solid` and the base host do not pull the editor | — | `scripts/verify-frontend-artifacts.mjs` |
| SSR import next to solid-ui | — | `scripts/ssr-smoke.mjs`, `web/sdk/scripts/check-package-exports.mjs` |

The benchmark writes its profile to `e2e/results/bench.json`; CI uploads it.

## Commands

```bash
just automation-editor dev     # fixture host with a fake API (query: graph, locale, theme, readonly, latency, fail, conflict, overlay, mode=canvas)
just automation-editor check   # typecheck and unit tests
just automation-editor build   # package build: chunk budget, declarations, SSR smoke
just automation-editor e2e     # Playwright scenarios (Chromium)
just automation-editor touch   # Chromium with CDP touch events and WebKit iPad
just automation-editor bench   # performance regression
pnpm --dir web/automation-editor vr          # compare with the approved Linux baselines (0 px); a missing one fails and is written as a candidate
pnpm --dir web/automation-editor vr-update   # rewrite the baselines on purpose, then review and commit them
```

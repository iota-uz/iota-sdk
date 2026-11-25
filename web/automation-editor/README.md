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
- Tidy (v2, issue #1391) lays the graph out top to bottom (`src/layout.ts`):
  - A column per trigger holds the steps reachable from that trigger alone; the steps reachable from
    several triggers (or none) sit below the columns, centred under the ones that feed them. Between
    groups the gap is twice the step between cards (`GROUP_GAP`, `GROUP_ROW_GAP`).
  - Steps entered only through `failed`, `timeout` or `low_confidence` (or from such steps) form a
    failure lane to the right of their chain (`FAILURE_OUTCOMES`).
  - A child stands under the port of the outcome that leads to it (the target handle is the middle of a
    card's top edge); long links reserve a slot in every row they pass, so a free channel always exists.
  - Edges are orthogonal (`src/route.ts`): out of the port, through the gaps between rows and columns,
    around the cards; the edges of one source share their channels (a common trunk). A card being
    dragged has a plain orthogonal edge until it lands.
  - `ensureLayout` places one missing step under its source; when more than a third of the steps, or a
    whole branch (two or more linked steps), lack positions, the graph is laid out again. Tidy marks
    every position it leaves with a quarter of a pixel on both coordinates (`TIDY_MARK`, `isTidied`): the
    layout is saved as a bare id → position map, so that is where "the system put it here" lives, and
    the server keeps it (coordinates are floats). A card that is dragged or moved with Alt+arrows is
    snapped or rounded to whole numbers and loses the mark; the canvas draws positions rounded
    (`unmarked`), so nothing shows. Groups with an unmarked position (placed by hand, or saved before the
    mark) keep their positions when the graph is laid out again; only the button, the menu item or ⇧⌘L
    moves them. This holds after a reload.
  - Quality is measured by pure functions (`src/quality.ts`: overlapping cards, links through cards,
    crossings, mixed groups); the tests hold Tidy to zero overlaps, zero links through cards and zero
    mixed groups on the fixtures `multiTrigger` and `foreachJoin`, and to no more crossings than the
    left-to-right Tidy it replaced (`src/test/legacyTidy.ts`). 240 steps take under 100 ms.
  - The Tidy button (broom, ⇧⌘L) is in the canvas controls next to Fit; the context menu has it too.
    One Tidy is one undo step; the cards glide for 200 ms (none under `prefers-reduced-motion`).
- Validation results belong to the snapshot they checked: they are rebased onto the current draft by
  item id, an edit invalidates a check in flight, and Save checks exactly the snapshot it sends.
- Layout saves run one at a time; when one ends, the layout on screen is compared with what was saved
  and the latest snapshot is sent again if they differ.
- An unavailable connection (ED10): the host's `validate` returns an error with
  `code: "connection_unavailable"` whose message names the connection; a catalog type with
  `connected: false` is reported by the editor itself. Such steps get a "No connection" badge.

## Runs, Dry run and Test node (issue #1347)

The host adds `runs`, `run` and, optionally, `version`, `cancelRun`, `retryRun` and `resolveRun` to
`EditorApi`, and passes `automationId`. The editor then owns the Runs tab (`src/runs/`, its own lazy
chunk); a host that passes `runsBody` keeps its own.

- **Runs tab** (RV1–RV2, RV9): a tab in the same toolbar; the draft, its version and Save survive the
  switch, the canvas tools are hidden. The list searches by subject (`runs({ subject })`: a key prefix, or the name the host finds by it),
  filters by state chips, shows the state as a dot and a word, the subject, node, version, time and the
  phrase of a failure; ↑ ↓ Home End move between rows. The list is the open version's history
  (`runs({ version })`); a run of another version still opens by its link. With no run chosen the newest
  opens (not on a phone, where the list is the screen). An answer to an older filter never replaces a
  newer one.
- **View state** (D1): `runsView` + `onRunsView(view, push)` — run, node, state filter, subject search. The
  host keeps it in the URL; without `onRunsView` the tab keeps it itself.
- **Run view** (RV3–RV4): the graph of the run's **version** (`api.version(n)`, cached per editor), never
  the draft, so steps removed since are still drawn; a banner «Run on version N · Open vN» when it is not
  the open version. The overlay is the pure `runOverlay(graph, run)` (`src/runs/model.ts`): finished step →
  passed / failed (the last attempt decides), active token → running, open wait → waiting, unreached by
  the end of a final run → skipped, a step closed by a cancel → cancelled, taken edges (a parallel's and
  a for each's from the tokens they made: the engine records them as `fork` / `each`), badges for attempts
  (> 1) and for each items (> 1), the message of a failed branch (`output.error.message`) as the card's
  note. A branch's `arrived` step on a join or a collect is not that node's outcome: it has no state
  until its own step (on any other node `arrived` is an outcome like any other).
- **Polling**: `run(id)` every 2 s while the run is `running` or `waiting`; an equal answer redraws
  nothing (P4). The indicator says Live / Reconnecting… / Finished. Runs read in the editor are kept, so
  a finished run is not asked for again and its poll does not resume on a tab switch.
- **Node drawer** (RV5): waits, steps (attempts, items), outcome, error, input and output as text or a
  JSON tree (no HTML is interpreted; a list or object of more than 200 entries shows «Show all N»; above
  256 KB folded with «Load in full», which shows every entry), logs (what a call node
  wrote with `engine.Log`, masked with the connection's secrets and kept with the step)
  with a filter and its own empty state; «Copy» is announced through `aria-live`.
- **Operate** (RV10): Cancel (confirmed, focus on «Keep running», the refusal shown in the dialog), Retry
  and Resolve (a combobox of the blocked step's outcomes, each with its target) for a blocked call. A
  button exists only with `can.operate` and its method; after an action succeeds the buttons wait for
  the run's new state (no second Retry meanwhile). Every action follows F1–F3: spinner, «Still
  waiting…» after 4 s, an error with «Retry» in place, a second click ignored.
- **Dry run** (RV7, `can.author`): subject, events, the decision mode (from cache, live call, set the
  answer), the path on the canvas and in the panel, the condition that kept each trigger from firing (its
  CEL filter or `param = value`), the number of calls made to providers (`live_calls`; a decision from the
  cache is a step with status `cached`) and what would be sent (captured effects with rendered params). The result
  belongs to the draft it ran on: an edit takes it off the canvas.
- **Test node** (RV8, `can.author`): the input is the subject, the event of the trigger that leads to the
  step (chosen when several do; the event carries that trigger's `kind`) by its payload schema, and the
  outputs of the steps before it (`nodes.<id>.output`). «From the last run» asks the server for the newest run that
  reached the step itself (`runs({ node })`, following the cursor past runs whose step the author may not read; each run is read
  with `run(id, { nodes })` — in batches of 50 for a long graph — so the latest readable visit of the node
  and of the nodes before it come even past the detail's bound of latest steps, and a later visit the
  author may not read does not hide it) and fills its subject, its event (`RunData.event`, projected by the server for
  the author, failing closed: an event or a field no current rule can check is left out) and those outputs. Every confirmed test carries an `idempotency_key` that a retry repeats:
  the server claims the key in `automation.node_tests`, so a repeat on any process gets the stored answer,
  and one without an answer (still running, its process died, or its answer expired after a day — the
  claim itself is never dropped) is refused as `outcome_unknown` rather than run again (F3); a step with an external effect is confirmed first,
  with the connection's name (`connectionName`) and «The effect is irreversible» unless the catalog says
  `idempotent: true`; a secret the step lacks (`secret_missing`) is marked «not set».
- **Phone**: the list full width, a run its own screen with the read-only outline (items open the
  drawer as a bottom sheet).

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
| ⇧⌘L | Tidy |
| ? | Key map (on touch it starts with gestures) |
| ↑ ↓ · Home End (Runs list) | Open the previous / next run, the first / last |
| Enter · Space (a run's row) | Open the run |
| Esc (a run's node drawer) | Close the drawer, focus its step |

Shortcuts do nothing while focus is in a field (⌘Z there is the field's own undo), except ⌘S.

## Budgets and checks

| Check | Limit | Where |
| --- | --- | --- |
| Canvas chunk (minified, gzip, without solid-js) | ≤ 52 KB | `scripts/check-chunk.mjs` in `build:package` |
| Mount, 200 steps / 500 edges, 4× CPU throttle | ≤ 150 ms | `e2e/bench.spec.ts` |
| Whole-graph replacement (undo), same graph | ≤ 60 ms (median of 10) | `e2e/bench.spec.ts` |
| Drag: long tasks · p95 frame | 0 · ≤ 17 ms | `e2e/bench.spec.ts` |
| `@iota-uz/sdk/solid` and the base host do not pull the editor | — | `scripts/verify-frontend-artifacts.mjs` |
| SSR import next to solid-ui | — | `scripts/ssr-smoke.mjs`, `web/sdk/scripts/check-package-exports.mjs` |

The benchmark writes its profile to `e2e/results/bench.json`; CI uploads it.

## Commands

```bash
just automation-editor dev     # fixture host with a fake API (query: graph (sample, connected, empty, bench, multi, foreach), locale, theme, readonly, latency, fail, conflict, overlay, mode=canvas)
just automation-editor check   # typecheck and unit tests
just automation-editor build   # package build: chunk budget, declarations, SSR smoke
just automation-editor e2e     # Playwright scenarios (Chromium)
just automation-editor touch   # Chromium with CDP touch events and WebKit iPad
just automation-editor bench   # performance regression
pnpm --dir web/automation-editor vr          # compare with the approved Linux baselines (0 px); a missing one fails and is written as a candidate
pnpm --dir web/automation-editor vr-update   # rewrite the baselines on purpose, then review and commit them
```

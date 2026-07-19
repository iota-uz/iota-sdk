# Lens React runtime workflow

Lens runtime work is verified through fixtures, Ladle stories, and Chromium
screenshots. The normal loop does not require a running ERP:

1. Edit the Go contract, runtime, panels, styles, or stories.
2. Run `just lens check` from the repository root.
3. Run `just lens ladle` and inspect the affected story at
   `http://localhost:61000`.
4. Run `just lens vr`.

A runtime PR that adds or changes a visible surface without corresponding
stories and visual-regression coverage is incomplete.

## Setup and commands

Install the workspace and Chromium once:

```sh
just lens install
cd web/lens && pnpm exec playwright install chromium
```

On a restricted machine that cannot write Playwright's user cache, use
`PLAYWRIGHT_BROWSERS_PATH=0 pnpm exec playwright install chromium`. The VR
runner detects that hermetic install automatically.

Use these root commands during development:

```sh
just lens check       # regenerate contract, reject drift, typecheck, lint, test
just lens ladle       # interactive story grid
just lens vr          # compare, or bootstrap ignored local OS baselines
just lens vr-update   # intentionally replace current-OS baselines
just lens build       # rebuild the embedded runtime distribution
```

The VR profile uses Chromium only, a 1600×1000 CSS viewport, device scale 1,
UTC, `en-US`, reduced motion, and the bundled Lens Inter font. URLs include
`lens-vr=1`; the Ladle hook disables CSS animations, View Transitions, and
ECharts animation before a story renders. Tests wait for the expected canvases,
font readiness, and two animation frames before taking full-page screenshots.

## Fixtures

Record a sanitized runtime document from an endpoint with:

```sh
just lens fixture https://example.test/lens/document --output my-case.json
```

The default output is `web/lens/fixtures/small.json`. Use `--cookie` or
`LENS_SESSION_COOKIE` for an authenticated local endpoint. Never commit session
cookies, credentials, or tenant-sensitive data. Give a durable scenario its own
named fixture, parse it through the generated contract, and reference it from a
story and tests. Refresh a fixture when the contract or the scenario's intended
shape changes, not merely to hide a validation failure.

## Type generation and drift

`just lens check` snapshots `web/lens/src/contract`, runs
`go run ./cmd/lens-typegen`, and fails if generation changes that snapshot before
it runs the web checks. If the Go document contract changed, run
`just lens typegen`, review the regenerated TypeScript, and rerun the check. CI
also runs typegen and compares the committed files with Git, so missing generated
output cannot pass after commit. Do not edit generated files by hand.

## Adding a panel kind

Definition of done for a new kind:

1. Add the Go contract kind and validation/build mappings under
   `pkg/lens/document`, then run `just lens typegen`.
2. Implement the panel or chart adapter and register it in
   `src/panels/registry.tsx`. Keep registry exhaustiveness tests green.
3. Add loading, empty, error, stale, and data cases to
   `src/PanelMatrix.stories.tsx` in both light and dark themes.
4. Add focused interaction or layout stories when the matrix cannot express the
   behavior. Add every new Ladle story to `vr/lens.spec.ts`; the manifest test
   fails if story and VR coverage diverge.
5. Run `just lens check`, inspect Ladle, run `just lens vr`, and rebuild with
   `just lens build` when runtime or contract output changed.
6. Rebaseline only after confirming the pixel change is intentional. Review the
   complete diff, run `just lens vr-update`, then run `just lens vr` again.

## Baseline platform and promotion

Font rasterization differs by operating system. Linux CI screenshots are the
only approved references. Baselines live in `vr/baselines/linux`; macOS and
Windows baselines are local, per-platform, and gitignored. Never copy a local
macOS image into the Linux directory.

When no Linux PNGs are committed, the regular CI lane enters bootstrap mode: it
generates candidates, passes without pretending to compare them, and uploads
`lens-vr-linux-candidates-<sha>`. To promote an honest reference:

1. Run the `Test, lint & build` workflow manually on the target commit with
   `lens_vr_update` enabled.
2. Download the `lens-vr-linux-candidates-<sha>` artifact.
3. Verify the story inventory and images, copy its PNGs into
   `web/lens/vr/baselines/linux`, and commit them without modification.
4. Push and confirm the normal `Lens Visual Regression` lane compares against
   those PNGs.

After promotion, unapproved pixel drift fails CI. The job uploads
`lens-vr-linux-diffs-<sha>` with actual, expected, diff, and trace artifacts.
Use `just lens vr-update` only for an approved visual change; updating snapshots
is never a substitute for understanding the diff.

## Guard check

To prove the lane is active, make a temporary one-pixel layout change in a
covered story or style, run `just lens vr`, and confirm it fails with a diff.
Revert that temporary change and rerun `just lens vr`; it must pass. Do not leave
the guard change or its local artifacts in the working tree.

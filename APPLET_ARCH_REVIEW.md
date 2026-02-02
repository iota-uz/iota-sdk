# Applet Architecture Review (pkg/applet + BiChat)

Date: 2026-02-02

This document reviews the current `pkg/applet/` integration design, how it is (and is not) used by `modules/bichat/`, and proposes a concrete, phased plan to converge on a single durable interconnect layer between Go (iota-sdk) and frontend applets.

## Executive take

- The repo currently has two competing integration patterns for the same concept (frontend applets embedded into the authenticated iota UI):
  - `pkg/applet` controller model: serve an applet shell + inject `window.__*_CONTEXT__`.
  - BiChat web controller model: render authenticated layout via templ + inject `window.IOTA_CONTEXT`.
- These patterns drift in context shape, routing, asset loading, and security posture.
- The `pkg/applet` system is not actually wired through `pkg/application` because there is a duplicated/mismatched “applet registry” and an application-level `Applet` interface that does not expose `Config()`.

Target direction:

- Bless one integration contract: `ui/applet-core` + a single Go-provided `InitialContext` schema.
- Wire applets through `pkg/application` with a single registry + a single applet interface.
- Make asset delivery compatible with Vite (manifest) and make context injection safe.
- Migrate BiChat to the blessed model and delete the bespoke duplicate.

## Current architecture (as it exists today)

### Applet system

- `pkg/applet` defines:
  - `Applet` interface (includes `Config()`)
  - `Config` that drives:
    - HTML shell injection (`WindowGlobal` + JSON context)
    - endpoints (GraphQL, stream)
    - assets (server-side filesystem + convention for JS/CSS)
    - routing helpers
    - optional `CustomContext`
    - optional middleware
  - `ContextBuilder` that builds a large `InitialContext` from request context and locale.
  - `AppletController` that mounts routes and serves assets.

### BiChat module

- `modules/bichat/applet.go` implements `applet.Applet` and provides:
  - base path `/bi-chat`
  - window global `__BICHAT_CONTEXT__`
  - assets from embedded `dist/`
  - endpoints `/bi-chat/graphql`, `/bi-chat/stream`
- But `modules/bichat/module.go` currently does not register the applet controller; it registers a bespoke web controller instead.
- The bespoke controller uses an authenticated templ layout and injects `window.IOTA_CONTEXT` via `<script type="application/json" id="iota-context-data">...`.
- Frontend BiChat code should read `window.__BICHAT_CONTEXT__` only (remove `window.IOTA_CONTEXT` dependency).

### Application wiring

- There is a robust registry in `pkg/applet/registry.go`.
- There is also a separate registry inside `pkg/application`, plus an application-level `Applet` interface that does not expose `Config()`.
- Net effect: `pkg/applet` is a self-contained subsystem but not truly a first-class integration point of the application.

## Problems found

### 1) Two integration patterns cause drift

Symptoms:

- `window.IOTA_CONTEXT` vs `window.__BICHAT_CONTEXT__`.
- Two different context schemas.
- Two different asset delivery strategies (manifest loader vs hard-coded `main.js`).

Impact:

- Frontend must support multiple globals.
- Backend duplications create inconsistent runtime behavior across applets.
- Harder to build new applets and keep them aligned.

### 2) Registry/interface mismatch blocks a single applet pipeline

Symptoms:

- `pkg/applet` expects an `applet.Applet` with `Config()`.
- `pkg/application` uses its own `Applet` interface/registry.

Impact:

- Applet controller mounting can’t be centralized.
- No single place to enforce invariants (unique base paths, middleware ordering, asset contract).

### 3) Context schema drift between Go and TypeScript

Symptoms:

- Go uses `InitialContext.Extensions` and has an `Error` payload.
- TS type in `ui/applet-core` uses `custom?` and is missing `error`.

Impact:

- Typed code on the frontend is lying about runtime.
- Encourages ad-hoc fields and one-off glue.

### 4) Route parsing/base path handling appears incorrect

Symptoms:

- `ContextBuilder.Build` calls `router.ParseRoute(r, "")` rather than using the applet base path.

Impact:

- Route info in context can be wrong for mounted applets.
- Frontend routing, breadcrumbs, and deep links become fragile.

### 5) Panic-heavy dependencies reduce composability

Symptoms:

- `composables.UsePageCtx` panics if middleware did not set it.
- BiChat applet has a helper that panics if the application is not in request context.

Impact:

- Applet usage becomes “works only in exactly this middleware stack”.
- Harder to test and reuse in different routes or embedding contexts.

### 6) Asset contract is not Vite-friendly

Symptoms:

- `AppletController` assumes `assetsURL/main.js`.
- BiChat web controller already has a manifest loader.

Impact:

- Hashed builds break unless you constrain build output to fixed names.
- Every applet may reinvent asset loading.

### 7) Context injection/security gaps

Symptoms:

- Custom context sanitization does not handle arrays/slices.
- JSON injection into `<script>` is not robust against `</script>` sequences unless escaped.

Impact:

- Potential XSS footguns.
- Unclear invariants for what is safe to pass via extensions.

## Recommended target architecture

### Blessed contract

- One global per applet: `window.<WindowGlobal>`.
- One context schema: `InitialContext` as defined by Go, with a TypeScript mirror.
- One backend integration point: application owns applet registration and mounts controllers consistently.

### Embedding mode

BiChat’s “embedded in authenticated layout” is legitimate, but it should be an option of the applet system, not a separate parallel system.

Two supported modes (same schema):

- **Standalone shell**: applet controller renders a minimal HTML shell.
- **Embedded shell**: applet controller (or a wrapper) renders a templ layout and still injects the same `window.<WindowGlobal>` context.

The key is: same context, same asset pipeline, same routing invariants.

## Concrete plan (phased)

### Phase 0 (now): Document + align contracts

- Add this doc (done).
- Decide the single blessed context global name for each applet: keep `window.__BICHAT_CONTEXT__` and remove `window.IOTA_CONTEXT` as an integration dependency.
- Align `ui/applet-core` types to match Go `pkg/applet/types.go`:
  - rename `custom` -> `extensions` (or add both temporarily with deprecation)
  - add `error` field
  - ensure URL casing matches (e.g., `refreshURL` vs `refreshUrl`)

Deliverable: frontend compiles with correct types and reads the applet global.

### Phase 1: Make application own applet wiring

- Remove the duplicated registry concept:
  - Option A (recommended): `pkg/application` uses `pkg/applet.Registry` directly.
  - Option B: define a thin application wrapper that delegates to `pkg/applet.Registry`.
- Unify the applet interface so application can mount applets via controller:
  - Update `pkg/application/interface.go` applet interface to include `Config() applet.Config` (or embed `applet.Applet`).
  - Ensure uniqueness by base path at registration time.

Deliverable: a single registration path for all applets.

### Phase 2: Fix routing invariants in context

- Update `ContextBuilder.Build` to use the correct base path when parsing route.
- Add/adjust tests to validate `route.path` trimming and params for mounted base paths.

Deliverable: `InitialContext.Route` is correct for mounted applets.

### Phase 3: Make asset loading Vite-compatible

- Add a manifest-based asset resolver option to `pkg/applet`:
  - e.g., `AssetsManifestPath` + `Entrypoint` (like `main.tsx`) OR `AssetsResolver` interface.
- Continue to support fixed-name assets as a fallback (for non-Vite or constrained builds).

Deliverable: applets can ship hashed assets without bespoke web controllers.

### Phase 4: Harden context injection and sanitization

- Ensure JSON embedded into `<script>` is safe:
  - escape `<` to `\u003c` at minimum (and/or use a safe JSON-in-script helper shared across controllers).
- Expand extension sanitization:
  - support arrays (`[]interface{}`) recursively.
  - consider allowing numbers/bools/null if there is a legitimate need; otherwise document as “strings only”.

Deliverable: consistent, safe injection across all applets.

### Phase 5: Migrate BiChat to the blessed model

- Re-enable applet registration in `modules/bichat/module.go`.
- Replace bespoke `BiChatWebController` usage with `pkg/applet` controller in embedded mode.
- Remove `window.IOTA_CONTEXT` initialization in `modules/bichat/presentation/templates/pages/index.templ`.
- Remove fallback reads in `modules/bichat/presentation/web/src/contexts/IotaContext.tsx` once migration is complete.

Deliverable: BiChat uses the same pipeline as other applets.

## Suggested file-level changes (high level)

- `pkg/application/interface.go`: unify applet interface to include `Config()`.
- `pkg/application/application.go`: remove local applet registry; use `pkg/applet.Registry`.
- `pkg/applet/context.go`: pass base path into route parsing; remove panic paths if possible (return errors).
- `pkg/applet/controller.go`: add manifest/entrypoint asset resolver path; use safe JSON-in-script output.
- `pkg/applet/security.go`: sanitize arrays (and possibly allow basic JSON primitives).
- `ui/applet-core/src/types/index.ts`: align `InitialContext` to Go.
- `modules/bichat/module.go`: register applet controller; delete bespoke controller wiring.
- `modules/bichat/presentation/controllers/web_controller.go`: delete or reduce to layout wrapper around applet embedding.
- `modules/bichat/presentation/templates/pages/index.templ`: stop setting `window.IOTA_CONTEXT`.

## Verification checklist

- `go test ./...` (or module-local tests if available) still pass.
- Route context reflects mounted path correctly for `/bi-chat/...`.
- BiChat loads correct hashed assets in dev + production build.
- No inline-script breakage when context contains `<` or `</script>`-like sequences.
- Frontend typechecks with `ui/applet-core` types.

## Task checklist (do not execute here)

This section is the actionable work breakdown to implement the plan. Treat it as the source of truth for execution.

Breaking-change posture (explicit):

- Prefer deleting legacy paths over keeping compatibility shims.
- Treat `window.IOTA_CONTEXT` and `InitialContext.custom` as legacy; remove them rather than maintaining dual support.

Status note:

- Some Phase 0 changes may already be present in the working tree (TypeScript types + BiChat context global usage + dev mock). If you are executing tasks from a clean baseline (main), implement them; if you are continuing from the current worktree, verify them and move on.

### Phase 0: Align contracts (schema + global)

- [ ] Decide/confirm the single context global name for BiChat: `window.__BICHAT_CONTEXT__` (remove `window.IOTA_CONTEXT`).
- [ ] Align TypeScript `InitialContext` to Go `pkg/applet/types.go`.
  - Files: `ui/applet-core/src/types/index.ts`
  - Requirements:
    - include `error` (nullable) and `extensions` (optional)
    - keep `refreshURL` casing
    - remove `custom` entirely (breaking)
- [ ] Remove any remaining frontend usage of `InitialContext.custom`.
  - Files: `ui/applet-core/src/**`, `ui/bichat/src/**`, `modules/**/presentation/web/src/**`
  - Acceptance: grep shows no `\.custom` access; only `extensions`.
- [ ] Remove `window.IOTA_CONTEXT` as an integration input in BiChat web app (breaking).
  - Files: `modules/bichat/presentation/web/src/contexts/IotaContext.tsx`
  - Acceptance: app throws a clear error unless `window.__BICHAT_CONTEXT__` is present.
- [ ] Update dev-only mock context injection to match the blessed global and schema.
  - Files: `modules/bichat/presentation/web/src/dev/mockIotaContext.ts`
  - Acceptance: `pnpm -C modules/bichat/presentation/web exec tsc --noEmit` passes.

### Phase 1: Single applet registration + mounting path

- [ ] Unify applet registry: remove/replace the duplicate registry in `pkg/application`.
  - Files: `pkg/application/application.go`, `pkg/applet/registry.go`
  - Option A (recommended): `pkg/application` owns a `pkg/applet.Registry` instance.
  - Acceptance:
    - applets can be registered once
    - uniqueness is enforced (name + base path)
    - lookup supports base-path routing.
- [ ] Unify the applet interface so application can mount a `pkg/applet` controller.
  - Files: `pkg/application/interface.go`
  - Acceptance: application-level `Applet` interface exposes `Config()` (or embeds `applet.Applet`).
- [ ] Add a single place to mount all registered applets.
  - Files: `pkg/application` router setup (where modules/controllers are mounted)
  - Acceptance: each applet base path mounts:
    - GET app shell
    - assets handler
    - optional stream + graphql endpoints (if configured).

### Phase 2: Fix route context correctness

- [ ] Fix `InitialContext.Route` computation for mounted applets.
  - Files: `pkg/applet/context.go`, `pkg/applet/router.go`
  - Work:
    - ensure `router.ParseRoute(...)` receives the applet base path (or make routers not require it)
  - Tests:
    - update/add tests in `pkg/applet/router_test.go` and/or `pkg/applet/context_test.go`
  - Acceptance: `route.path` is the path after base path for `/bi-chat/...`.

### Phase 3: Vite/manifest-friendly asset resolution

- [ ] Introduce a manifest-based asset resolution option for `pkg/applet`.
  - Files: `pkg/applet/controller.go` (and a small new helper type/file)
  - Design:
    - add an `AssetsResolver` interface, or
    - add `AssetsManifestPath` + `Entrypoint` to config
  - Acceptance:
    - hashed Vite assets load without requiring `main.js`
    - embedded FS assets still work.

### Phase 4: Security hardening for injected context

- [ ] Make JSON injection into `<script>` safe.
  - Files: `pkg/applet/controller.go` (and preferably a shared helper, e.g. `pkg/applet/jsonscript.go`)
  - Work:
    - ensure sequences like `<` and `</script>` cannot terminate the script tag (escape strategy)
  - Acceptance: context containing `<` does not break the page and cannot inject HTML.
- [ ] Expand extension sanitization to support arrays recursively.
  - Files: `pkg/applet/security.go`
  - Acceptance: `extensions` can contain nested arrays of strings/maps safely (or explicitly reject with clear error).

### Phase 5: Migrate BiChat to the blessed applet pipeline

- [ ] Re-enable applet registration for BiChat.
  - Files: `modules/bichat/module.go`
  - Acceptance: BiChat registers its `applet.Applet` and mounts via the unified application applet system.
- [ ] Remove bespoke `BiChatWebController` (or reduce it to a thin layout wrapper that still delegates context + assets to `pkg/applet`).
  - Files: `modules/bichat/presentation/controllers/web_controller.go`
  - Acceptance: only one source produces the runtime context.
- [ ] Remove `window.IOTA_CONTEXT` initialization from the templ page.
  - Files: `modules/bichat/presentation/templates/pages/index.templ`
  - Acceptance: frontend only relies on `window.__BICHAT_CONTEXT__`.

### Final verification

- [ ] Backend: `go vet ./...`
- [ ] Backend tests: `go test ./...`
- [ ] Frontend: `pnpm -C modules/bichat/presentation/web build`
- [ ] Frontend typecheck: `pnpm -C modules/bichat/presentation/web exec tsc --noEmit`

## Reviewer checklist (for follow-up review)

- In Go, confirm there is exactly one applet registry and exactly one applet interface used for registration/mounting.
- In BiChat, confirm the templ page no longer sets `window.IOTA_CONTEXT` and React code only reads `window.__BICHAT_CONTEXT__`.
- In `ui/applet-core`, confirm `InitialContext` matches Go `pkg/applet/types.go` (including `error` and `extensions`) and there is no legacy alias.
- In `pkg/applet`, confirm route parsing uses base path correctly and has tests.
- In `pkg/applet`, confirm asset resolution supports Vite manifest (hashed assets) without per-module bespoke loaders.
- In `pkg/applet`, confirm context injection escaping prevents script-tag breaks and `extensions` sanitization handles arrays.

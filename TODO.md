# BiChat Frontend Parity (vs EAI Ali)

This document tracks frontend gaps between the IOTA SDK BiChat module app and the EAI Ali BI Chat app. Items below describe what is missing in `iota-sdk` and what to implement for parity.

**Reference:** EAI implementation lives in `eai/back/modules/ali/presentation/web/`. SDK BiChat app lives in `iota-sdk/modules/bichat/presentation/web/`.

---

## 1. Archived list page — Missing in SDK

**Status:** ❌ No route in SDK

**In EAI:**
- Route: `/archived` → full-page `<ArchivedChatList />`.
- Page uses `dataSource.listSessions({ includeArchived: true, limit: 100 })`, filters to `status === 'archived'`, shows search, date groups, `SessionItem` in "archived" mode, restore (with `ConfirmModal`) and rename; toasts for success/error.
- Sidebar exposes "Archived chats" link that calls `onArchivedView` → `navigate('/archived')`.

**TODO:**
- [x] Add route `/archived` in `modules/bichat/presentation/web/src/App.tsx`.
- [x] Add a page that lists only archived sessions (reuse `ArchivedChatList` from `@iota-uz/sdk/bichat` with `dataSource`, `onBack` → `navigate('/')`, or build a thin page that calls `listSessions({ includeArchived: true })`, filters by `status === 'archived'`, and uses the same UI patterns).
- [x] Ensure Sidebar has an "Archived" entry that navigates to `/archived`.

---

## 2. "All Chats" (permission-based) — Disabled in SDK

**Status:** ❌ `canReadAllChats = false` in module Sidebar

**In EAI:**
- `usePermission('AIChat.ReadAll')` → `canReadAllChats`; `<SdkSidebar showAllChatsTab={canReadAllChats} />`. When true, Sidebar shows "All Chats" tab and uses `dataSource.listAllSessions?.(...)`.

**TODO:**
- [x] Either switch module to use shared `Sidebar` from `@iota-uz/sdk/bichat` and pass `showAllChatsTab` from app permission/config, or add an "All Chats" tab in the custom Sidebar and call `listAllSessions` when provided.
- [x] Gate the tab with a permission or feature flag from IotaContext if desired.

---

## 3. Mobile sidebar (overlay, swipe) — Desktop-only in SDK

**Status:** ❌ No mobile layout in SDK

**In EAI:**
- `useSidebarState()`: `isMobile` via `matchMedia('(max-width: 767px)')`, `isMobileOpen` / `openMobile` / `closeMobile`.
- Desktop: sidebar in column `hidden md:block`.
- Mobile: sidebar only when overlay open; `motion.div` backdrop + drawer with `drag="x"`, swipe-left-to-close, `useFocusTrap(sidebarRef, ...)`. Sidebar receives `onClose` to close overlay after navigation.

**TODO:**
- [x] Add layout that, below a breakpoint (e.g. 768px), hides sidebar from normal flow and shows it as an overlay drawer (e.g. via menu button).
- [x] Add a small state hook (e.g. `useSidebarState`) for open/close and "is mobile".
- [x] Use Framer Motion (or equivalent) for backdrop + swipe-to-close; optional focus trap when drawer is open.
- [x] Pass `onClose` into Sidebar on mobile so drawer closes after navigation.

---

## 4. Read-only session — Not in SDK

**Status:** ❌ No read-only mode in SDK

**In EAI:**
- `?readonly=true` on session URL; `ChatSessionCore` receives `isReadOnly`: header shows "Read-only" badge, message input hidden, regenerate/edit disabled, MessageList does not show edit/regenerate on turns.

**TODO:**
- [x] In module `ChatPage`, parse `searchParams.get('readonly') === 'true'` and pass a prop (e.g. `readOnly`) into `ChatSession`.
- [x] In SDK UI `ChatSession` (and shared header/message list), add `readOnly` prop: when true, hide message input, disable regenerate/edit, show "Read-only" in header.

---

## 5. Toasts / touch / haptics — Not in SDK module app

**Status:** ❌ No global toasts or touch/haptics in module

**In EAI:**
- `ToastProvider` at app root (uses `useToast()` from `@iota-uz/sdk/bichat` + `ToastContainer`). Components use toast for restore, rename, archive, etc.
- `TouchProvider` and `useTouchDevice` for touch detection; `useHapticFeedback()` (e.g. `navigator.vibrate`) for light feedback.

**TODO:**
- [x] Wrap BiChat app in a provider that uses `useToast()` and renders `ToastContainer`; use it in Sidebar, ArchivedChatList, etc. for success/error.
- [x] (Optional) Add TouchProvider and useHapticFeedback in module app if touch/haptic parity is desired.

---

## 6. Navigation guard (scope) — Not in SDK

**Status:** ❌ No guard in SDK

**In EAI:**
- `useNavigationGuard`: validates `location.pathname` with `isValidRoute(path)`; on invalid, calls `onInvalidNavigation()` (e.g. `navigate('/', { replace: true })`).
- `useSafeNavigate`: wraps `useNavigate()`, rejects `../` and paths outside whitelist (`/`, `/session/*`, `/archived`), redirects to `/` otherwise.

**TODO:**
- [x] Add `useNavigationGuard` that validates current path and redirects to `/` (or base) when invalid.
- [x] (Optional) Add `useSafeNavigate` that only allows in-scope paths when app is embedded under a base path.

---

## Summary

| Feature                     | EAI | SDK | Action |
|----------------------------|-----|-----|--------|
| Session list in sidebar     | ✅  | ✅  | — |
| Archive/unarchive (API)     | ✅  | ✅  | — |
| Archived list page         | ✅  | ❌  | §1 |
| "All Chats" (permission)    | ✅  | ❌  | §2 |
| Artifacts panel            | ✅  | ✅  | — |
| Mobile sidebar             | ✅  | ❌  | §3 |
| Read-only session          | ✅  | ❌  | §4 |
| Toasts / touch / haptics   | ✅  | ❌  | §5 |
| Navigation guard           | ✅  | ❌  | §6 |
| Feature flags in context   | ✅  | ✅  | — |
| Debug limits in context    | ❌  | ✅  | — |

---

# Other Features Missing in SDK (Present in EAI)

Features that exist in EAI (ali or surrounding app) but are not yet in the SDK, beyond the frontend parity list above.

---

## Backend / integration

### 8. App-specific permissions for StreamController

**In EAI:** When registering the BiChat stream controller, EAI passes `WithRequireAccessPermission(alipermissions.AIChatAccess)` and `WithReadAllPermission(alipermissions.AIChatReadAll)` so that stream access and “read other users’ sessions” use EAI’s permission model.

**In SDK:** The bichat module registers `StreamController` with default options (base path, default bichat permissions). Apps that need their own permission constants cannot override these without registering the controller themselves (outside the module).

**TODO:**
- [x] Document that apps can register `StreamController` themselves with `WithRequireAccessPermission` / `WithReadAllPermission` when using BiChat as a library.
- [x] Or: allow `ModuleConfig` (or applet config) to accept optional permission overrides for the stream controller so the module registers it with app-specific permissions.

---

### 9. Serving export/artifact files (uploads controller)

**In EAI:** `UploadsController` serves files from a local directory (e.g. `uploads/ali`) under `/ali/uploads/`, with auth and `AIChatAccess` check. Export tools write Excel/PDF there; artifact URLs point at this path so the UI can download them.

**In SDK:** `AttachmentStorageBaseURL` is set by the app; the app must ensure that URL is reachable. There is no built-in controller that serves files from a local path with auth.

**TODO:**
- [ ] Document that apps must expose artifact/export URLs (e.g. register a controller that serves from the attachment base path with auth).
- [ ] Or: add an optional generic “uploads controller” in the SDK (or in `modules/bichat`) that serves a given directory under a path with authorization middleware.

---

## Frontend (additional)

### 10. SkipLink and global keyboard shortcuts in module app

**In EAI:** App root renders `<SkipLink />` and calls `useKeyboardShortcuts([{ key: 'n', ctrl: true, callback: () => navigate('/'), description: 'New chat' }])` so Ctrl+N opens a new chat.

**In SDK:** The SDK UI package exports `SkipLink` and `useKeyboardShortcuts`, but the bichat module app (`modules/bichat/presentation/web`) does not use them.

**TODO:**
- [ ] In the module app, add `SkipLink` at the root and register at least one shortcut (e.g. Ctrl+N for new chat) via `useKeyboardShortcuts`.

---

### 11. Client-side rate limiting

**In EAI:** `utils/rateLimiter.ts` defines a sliding-window rate limiter (e.g. 20 requests per minute). The chat UI uses it to throttle send/RPC and show a rate limit message.

**In SDK:** Module app has no client-side rate limiter.

**TODO:**
- [ ] (Optional) Add a configurable client-side rate limiter (e.g. max requests per minute) and use it before send message or RPC, with user-visible feedback when limit is hit.

---

### 12. Session event context (sidebar refresh on new session)

**In EAI:** `SessionEventProvider` provides `notifySessionCreated()` and `onSessionCreated(callback)`. When the user sends the first message on the landing page, a new session is created and `notifySessionCreated()` is called so the sidebar can refetch and show the new session.

**In SDK:** Module app has no session event pub/sub; the sidebar may not refresh after creating a session from the home page until the user navigates or reloads.

**TODO:**
- [ ] Add a small session-event context (or equivalent) in the module app: emit “session created” when the first message creates a session, and have the sidebar subscribe and refetch its list.

---

### 13. Message queue persistence (sessionStorage)

**In EAI:** `utils/queueStorage.ts` persists queued messages to `sessionStorage` per `sessionId` (`saveQueue` / `loadQueue`). If the user refreshes while messages are queued, they are restored.

**In SDK:** SDK UI has in-memory `messageQueue` in ChatContext but no persistence; a refresh clears the queue.

**TODO:**
- [ ] (Optional) Add queue persistence (e.g. sessionStorage keyed by sessionId) in the SDK UI or module app, and restore queue on load when using the same session.

---

### 14. Top-level error boundary

**In EAI:** The app is wrapped in an `ErrorBoundary` with `ErrorBoundaryContent` so React errors are caught and show a friendly message instead of a blank screen.

**In SDK:** Module app has no top-level error boundary.

**TODO:**
- [ ] Wrap the module app (or the main route tree) in an error boundary with a simple fallback UI (retry / go home).

---

### 15. Route transition animations

**In EAI:** Route changes use `RouteTransition` and Framer Motion’s `AnimatePresence` for enter/exit animations.

**In SDK:** Module app does not use route-level transitions.

**TODO:**
- [ ] (Optional) Add route transition wrapper (e.g. AnimatePresence + simple fade/slide) for polish.

---

# Things Missing in EAI (Present or Recommended in SDK)

For teams (e.g. EAI) that consume the SDK: capabilities the SDK provides or recommends that EAI does not yet use or implement.

---

### Debug limits in context

**In SDK:** The bichat applet `buildCustomContext` injects `extensions.debug.limits` (policyMaxTokens, modelMaxTokens, effectiveMaxTokens, completionReserveTokens) from ModuleConfig so the UI can show token usage and limits.

**In EAI:** Ali applet `buildCustomContext` only injects `features` (all false) and `permissions`; it does not pass `debug.limits`.

**Suggestion for EAI:** If the Ali frontend should display token limits or usage, add a `debug.limits` object to the applet custom context (e.g. from the same context policy and model used when building the agent).

---

### Config-driven feature flags

**In SDK:** Feature flags (vision, webSearch, codeInterpreter, multiAgent) come from `ModuleConfig` and are passed to the frontend via `extensions.features`.

**In EAI:** Ali applet hardcodes all feature flags to `false` in `buildCustomContext`. Enabling vision or code interpreter would require code changes.

**Suggestion for EAI:** If Ali will support vision, web search, or code interpreter, drive these from config (or env) and pass them through custom context instead of hardcoding false.

---

### Observability (EventBridge, Shutdown)

**In SDK:** When using `NewModuleWithConfig(cfg)`, the module can register observability providers and an EventBridge, and expose `Shutdown()` to flush and tear down providers on app exit.

**In EAI:** EAI does not use the bichat module’s `NewModuleWithConfig` or observability; it wires services directly and does not register the bichat module.

**Suggestion for EAI:** If you want BiChat events and traces in the same pipeline as the rest of the app, consider adopting the SDK’s observability options (or ensure your own observability covers the bichat services you use).

---

### Multi-agent orchestration

**In SDK:** ModuleConfig supports an optional `AgentRegistry` and multi-agent mode; the parent agent can delegate to sub-agents (e.g. SQL agent) via the `task` tool.

**In EAI:** Ali uses a single parent agent with no sub-agents or delegation.

**Suggestion for EAI:** Optional. If you want delegation (e.g. to a dedicated SQL or explorer sub-agent), you can adopt the SDK’s agent registry and multi-agent wiring.

---

### listAllSessions for “All Chats” tab

**In SDK:** The `ChatDataSource` interface has an optional `listAllSessions?(...)` for listing sessions across users (for “All Chats” with read-all permission). The standard bichat RPC router does not expose a procedure for this; only `bichat.session.list` (per-user) exists. The SDK `HttpDataSource` does not implement `listAllSessions`.

**In EAI:** Ali shows `showAllChatsTab` when the user has read-all permission, but the SDK RPC does not provide list-all. So either the “All Chats” tab has no backend (and cannot load other users’ sessions) or EAI has a custom backend/RPC for it.

**Suggestion for EAI:** If “All Chats” should work, implement a list-all-sessions backend (tenant-scoped, permission-checked) and expose it (e.g. via a custom RPC procedure or endpoint). Then implement `listAllSessions` on the data source used by the Sidebar so the tab can load and display those sessions.

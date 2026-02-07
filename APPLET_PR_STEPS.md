# Applet DX – intermediate PR and branch cleanup

## 1. Compare current branch vs main ✅
- Compared `codex/applet-dx-v1` (formerly `codex/applet-dx-v1-local`) with `main`.
- Identified applet-architectural changes (no BiChat logic).

## 2. Applet-only PR branch ✅
- **Branch:** `codex/applet-dx-pr` (pushed to `origin`)
- **PR link:** https://github.com/iota-uz/iota-sdk/pull/new/codex/applet-dx-pr

### Included in this PR
- **cmd/dev/main.go** – unified dev runner (templ, tailwind, air, optional applet Vite)
- **scripts/applets.json** – applet registry
- **Justfile** – `just dev <name>`, `just applet [rpc-gen|rpc-check|deps-check]`
- **pkg/cli/root.go** – register `applet` command
- **internal/applet/rpccodegen/** – RPC contract TypeScript codegen
- **pkg/applet/** – typed RPC, controller, manifest, security
- **pkg/commands/cli_applet.go** – applet CLI
- **ui/src/applet-core/, applet-devtools/, applet-host/** – host and runtime UI
- **modules/core/.../authenticated.templ** (+ generated) – theme switcher tweaks

### Not included (stay in PR #562)
- cmd/server/main.go, cmd/superadmin/main.go (BiChat wiring)
- modules/core/module.go, upload_api_controller, session_repository (upload API used by BiChat)
- All modules/bichat, pkg/bichat, ui/src/bichat, etc.

## 3. What you do
1. Open PR: **base `main` ← head `codex/applet-dx-pr`**
2. Review and merge into `main`

## 4. After you merge – update current branch (clean history)
From repo root:

```bash
git fetch origin main
git checkout codex/applet-dx-v1
git rebase origin/main
# Resolve any conflicts (applet files now come from main; keep BiChat-only changes).
git push origin codex/applet-dx-v1 --force-with-lease
```

This drops the applet-only commit from `codex/applet-dx-v1` so PR #562 only shows BiChat-related diff.

## 5. Branch rename ✅
- Local branch renamed: **codex/applet-dx-v1-local** → **codex/applet-dx-v1**
- Upstream set: **origin/codex/applet-dx-v1**

---

**Stash:** You had local changes in `pkg/bichat/domain/artifact.go` and `pkg/cli/exitcode/codes.go` (stash: "temp before applet PR"). Restore with:

```bash
git stash pop
```

---
allowed-tools: |
  Bash(git status:*), Bash(git diff:*), Bash(go vet:*), Bash(make *),
  Read, Glob, Task
argument-hint: "[optional] files/dirs/globs (defaults to uncommitted changes)"
description: "Holistic refactor with permission to break compatibility; replace layered hacks with simpler architectures"
---

# /refactor-review

**Usage**

- `/refactor-review` — analyze uncommitted changes (staged+unstaged) and perform holistic refactor
- `/refactor-review ./pkg/...` — focus scope
- Flags: `--dry-run`, `--max-chunk=<int>` (default: 700 LOC), `--tests=auto|none|^TestName$` (default: auto),
  `--make-target="check lint"`

---

## Guardrails (hard rules)

- Prefer **replacement** over patch-on-patch; if multiple nested fixes exist, **redesign**.
- Allowed to **break backwards compatibility** when it meaningfully simplifies the system. Emit migration notes.
- Keep public surface **small and cohesive**; hide complexity behind narrow interfaces.
- Idempotent edits: re-running yields no churn.
- Never edit `vendor/**`, `node_modules/**`, generated files (`*.pb.*`, `*.gen.*`, `*.min.*`), migrations.

---

## Orchestration

### 0) Target selection

- If args present → use via **Glob**; else use `git status --porcelain` + `git diff --name-only`.
- Filter to sources (`go, ts, tsx, js, py`) and exclude paths above.

### 1) **Holistic Exploration (always)**

Read targets + neighbors; output a concise **System Map**:

- Responsibilities per file; edges (data/flow), hot paths, owning tests.
- **Pain Points:** duplication, tight coupling, leaky/temporal abstractions.
- **Design Options (A/B/C):** each with 1–2 bullets (benefits/tradeoffs) + *estimated edit size*.
- **Pick the simplest option that removes layered hacks**, even if large refactor is required.

### 2) **Apply Changes (clarity > minimality)**

**Task(subagent_type:go-editor) → prompt:**

Goal: Deliver code that is easy to read and reason about by replacing brittle,
layered patches with simpler, cohesive architectures. You MAY break compatibility
to achieve a meaningfully simpler design; include migration notes.

ENFORCE THIS RUBRIC

1. Boundaries & Cohesion: extract modules with single responsibility; no cross-layer leaks.
2. Flow: early returns; flatten nesting; max func ~30 LOC unless clearly readable.
3. Naming: precise, pronounceable; avoid cryptic abbreviations.
4. Errors: wrap with context; centralize logging; no swallowing.
5. Data Shapes: explicit structs/DTOs at boundaries; avoid ad-hoc maps.
6. Interfaces: accept interfaces; return concrete; narrow signatures.
7. Tests: table-driven; Given/When/Then; stabilize flaky paths; cover new boundaries.
8. Performance: obvious wins that don’t harm clarity (prealloc, reduce allocs in hot paths).
9. Security: validate inputs; safe SQL/HTML building; enforce role/permission checks.
10. Delete dead code; merge duplicates; replace multi-branch hacks with a single clear path.

IOTA SDK norms (when relevant): SQL mgmt, repository pattern, DDD naming,
HTMX flows, consistent error/logging, role/permission checks for broadcasts.

WORK MODE

* Batch ≤ {{--max-chunk}} LOC; show unified diff per batch.
* For each change: add a one-line “Why this is clearer”.

OUTPUT (strict):

## System Map

## Chosen Design (why; tradeoffs; files N; LOC ~X)

## Changes

### Critical ❌

### Minor 🟡

### Style/Nits 🟢

## Next Steps (optional)

Idempotence: re-runs produce no noisy churn.

### 3) Verification loop (stop on red)

- `go vet ./...`
- If `--make-target` → `make <target>`
- Tests:
    - `--tests=none` → skip
    - `--tests=^TestName$` → targeted run
    - `auto` → run tests referenced by agent; else `go test ./... -count=1`
- On failure: classify (impl|test|fixture|dep) → one corrective batch → re-verify.

---

## Output footer

## Verification Results

* go vet: PASS/FAIL
* make {{--make-target}}: PASS/FAIL (if run)
* tests: PASS/FAIL summary

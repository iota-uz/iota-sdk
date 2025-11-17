---
allowed-tools: |
  Bash(git fetch:*), Bash(git rev-parse:*), Bash(git for-each-ref:*),
  Bash(git diff:*), Bash(git show:*), Bash(git log:*),
  Read, Grep, Glob, Task,
  mcp__github__get_pull_request,
  mcp__github__get_pull_request_status,
  mcp__github__get_pull_request_comments,
  mcp__github__get_pull_request_reviews
argument-hint: PR URL or PR number (e.g., https://github.com/owner/repo/pull/123 or 123)
description: "Read-only: collect PR context (what/why/where), list changed files & targeted diffs, summarize unresolved comments & CI; print everything to stdout; then await user instructions."
---

# Continue Work on a PR — Read-Only Context

Input: $ARGUMENTS

## Stop Rules

- **Never write files.** No checkout, add, commit, or push.
- **Only print to stdout**, then pause for next instruction.

## 0) Resolve PR Coordinates

- If URL `https://github.com/{owner}/{repo}/pull/{num}` → extract `{owner, repo, num}`.
- If number → use current repo’s `origin` as `{owner/repo}`.
- Fetch PR meta via `mcp__github__get_pull_request`.

## 1) Collect PR Metadata (read-only)

- Title, author, state, draft?, base→head branches, labels, milestone, link.
- PR body (first ~2000 chars, normalized whitespace).
- CI snapshot via `mcp__github__get_pull_request_status` (overall + top failing checks, no logs).
- Reviews/comments:
    - `mcp__github__get_pull_request_comments`
    - `mcp__github__get_pull_request_reviews`
    - Keep **unresolved/actionable** only; group by file:line.

## 2) Locate BASE/HEAD SHAs (no checkout)

- BASE: `origin/{base.ref}` (verify with `git for-each-ref`).
- HEAD reference resolution:
    - Try same-remote fetch: `git fetch origin +refs/pull/{num}/head:refs/remotes/origin/pr-{num} || true`
    - Fallback: `git fetch {head.repo.clone_url} +refs/heads/{head.ref}:refs/remotes/pr/{num}`
- HEAD = `refs/remotes/origin/pr-{num}` or `refs/remotes/pr/{num}`.

## 3) Change Surface (bounded)

- Diffstat: `git diff --numstat BASE...HEAD` (cap to 500 files).
- Per-file detail (only if total changed lines ≤ 400 for that file):
    - `git diff -U2 BASE...HEAD -- {file}` (truncate each patch at ~300 lines).
- Detect hot spots:
    - Files with >200 changed lines
    - API/toplevel signatures (grep `export|public|interface|class|func|type|api`)

## 4) Print **Context Pack** (stdout only)

Render in this exact order:

=== PR OVERVIEW ====================================================
PR: #{num} • {title}
Link: {html_url}
Author: {user} • State: {open/closed} • Draft: {true/false}
Base: {base.repo}:{base.ref} ← Head: {head.repo}:{head.ref}
Labels: [a, b, c] • Milestone: {name|none}

--- WHY (from PR body, condensed) ---------------------------------
{5–10 line summary}

--- CI SNAPSHOT ----------------------------------------------------
Overall: {success|pending|failed|skipped}
Checks (top 10): {name → status}

--- UNRESOLVED REVIEW ITEMS ---------------------------------------

{file}:{line} → {one-line actionable}

…
(Total unresolved: N)

--- CHANGED FILES (numstat) ---------------------------------------
{added} {removed} {path}
…
(Total files: F • Total +/-: +X/-Y)

--- HOT SPOTS ------------------------------------------------------

{path} → {~lines changed}, {reason}

--- TARGETED PATCHES (truncated) ----------------------------------

{path}
{small unified diff up to limit}

{path}
{…}
(Only small patches shown; large files summarized above.)
After printing the Context Pack, use AskUserQuestion to ask what to do next:

```
AskUserQuestion:
- Question: "What would you like to do next?"
- Header: "Continue PR"
- Options:
  - "Deep CI dive (fetch failing job logs and analyze)"
  - "Address unresolved comment (specify index or file:line)"
  - "Generate targeted test plan for touched packages"
  - "Draft reviewer replies to comments"
  - "Lint/format plan (no writes, just show commands)"
- multiSelect: false
```

After user selects an option, proceed with that specific analysis or action.

## Guardrails

- Never create/modify refs other than temporary remote refs; **no checkout**.
- Large PRs: keep totals, skip patches beyond limits.
- Fork PRs: use `refs/remotes/pr/{num}` namespace; do not alter remotes.

## Exit

- After printing the Context Pack, present AskUserQuestion menu (see section above)
- Wait for user selection
- Process selected action based on their choice
- Do NOT make assumptions about next steps

Decision Tree (compact):
Parse input → fetch PR meta → resolve BASE/HEAD (fetch-only) → compute diffstats & small patches → collect unresolved comments & CI status → print Context Pack → present AskUserQuestion menu → await user selection → execute chosen action.


---
allowed-tools: Read, Grep, Glob, Bash(grep:*), Bash(find:*), Bash(make:*), Bash(curl:*), Bash(test:*), Bash(sed:*), WebFetch, Task
description: "Validate that API references, makefile commands, file paths, markdown cross-references, and external URLs in config files and CLAUDE.md files are still valid and not outdated"
model: sonnet
disable-model-invocation: true
---

# Claude Code Configuration Validator

Validates all references in `.claude/` + `CLAUDE.md` files: API refs, makefile commands, file paths, agent/cmd names,
bash syntax. Enforces: no redundant CLAUDE.md refs, single responsibility, no duplication. Delegates to `Explore`
agents → apply fixes → report.

## Reference Documentation

**Claude Code Guides (for architectural rules):**

- `.claude/guides/claude-code/architecture.md` - Core principles, reserved commands, patterns
- `.claude/guides/claude-code/commands.md` - Command format and conventions
- `.claude/guides/claude-code/agents.md` - Agent format and conventions
- `.claude/guides/claude-code/settings.md` - Settings format and conventions

## Workflow

### Phase 0: Multi-Agent Orchestration Strategy

**Note:** `Explore` and `Plan` are built-in agent types provided by Claude Code. They do not need to be defined in
`.claude/agents/` and are always available for complex, multi-step tasks.

**Agent Delegation Pattern:**

Launch **4 `Explore` agents in parallel** for specialized validation domains:

```
(Explore #1 & Explore #2 & Explore #3 & Explore #4)
→ consolidate results
→ apply fixes
→ generate report
```

**Work Distribution by Validation Domain:**

| Agent                     | Validation Scope                              | Files Scanned                |
|---------------------------|-----------------------------------------------|------------------------------|
| **IOTA SDK Validator**    | API references, Makefile commands             | All `.claude/` + `CLAUDE.md` |
| **Claude Code Validator** | Agent/Cmd refs, Bash syntax, CLAUDE.md refs   | All `.claude/` + `CLAUDE.md` |
| **File Structure**        | File paths, line-based refs, anchor migration | All `.claude/` + `CLAUDE.md` |
| **Content Quality**       | Line numbers, cross-refs, duplication, SRP    | All `.claude/` + `CLAUDE.md` |

**Common Template for All Agents:**

*Files to Scan:* `.claude/agents/*.md`, `.claude/commands/*.md`, `.claude/guides/**/*.md`, `CLAUDE.md`,`back/CLAUDE.md`,
`bff/CLAUDE.md`, `app/CLAUDE.md`

*Deliverable Format:* JSON with `agent`, `issues` array containing
`{category, file, line, issue, recommendation, severity}`

---

**Agent #1: IOTA SDK Validator**

```
You are validating IOTA SDK REFERENCES in .claude/ config files and CLAUDE.md files.

**Your Scope (and ONLY your scope):**

1. **API References:** Find and validate all IOTA SDK API calls
   - Target packages: composables, shared, htmx, repo, itf, di
   - Check existence in ../iota-sdk/pkg/{package}/
   - Verify function signatures haven't changed
   - Report: file:line, reference, status (valid/deprecated/missing)

2. **Makefile Commands:** Verify make targets exist in Makefile and back/Makefile
   - Check all `make target` references
   - Validate target names against actual Makefile contents
   - Verify command syntax in make examples

**Categories:** `api_references`, `makefile_commands`
```

**Agent #2: Claude Code Validator**

```
You are validating CLAUDE CODE ORCHESTRATION in .claude/ config files and CLAUDE.md files.

**Your Scope (and ONLY your scope):**

1. **Agent & Command References:** Validate CLAUDE.md orchestration patterns
   - Agent names (backtick format like `editor`) match .claude/agents/*.md
   - Slash commands (like /commit-pr) match .claude/commands/*.md
   - Workflow syntax is correct (&, &&, Nx agent patterns)
   - Verify agent capabilities match their usage context

2. **Bash Commands:** Check command syntax in code blocks
   - Verify bash/sh/shell command syntax is valid
   - Check common CLI tool flags and patterns
   - Validate command examples are executable

3. **Redundant CLAUDE.md References:** Remove all references to CLAUDE.md files
   - WHY: Claude Code auto-loads CLAUDE.md into context (root always loaded, child CLAUDE.md auto-loads when working in that directory)
   - FORBIDDEN: Any file → CLAUDE.md references (redundant noise)
   - FORBIDDEN: Child CLAUDE.md → parent CLAUDE.md (both already in context)
   - Scan for: "CLAUDE.md", "../CLAUDE.md", "../../CLAUDE.md", "back/CLAUDE.md", "bff/CLAUDE.md", "app/CLAUDE.md"
   - Report: file:line location of each reference found

**Categories:** `agent_command_references`, `bash_commands`, `redundant_claude_refs`
```

**Agent #3: File Structure Validator**

```
You are validating FILE STRUCTURE AND REFERENCES in .claude/ config files and CLAUDE.md files.

**Your Scope (and ONLY your scope):**

1. **File Path References:** Check all markdown cross-references
   - Verify all file paths exist
   - **ENFORCE ABSOLUTE PATHS ONLY**: All internal project paths must be absolute from project root
   - FORBIDDEN: Relative paths like `../`, `./`, or bare filenames like `patterns.md`
   - REQUIRED: Absolute paths like `.claude/guides/backend/patterns.md`
   - Exception: External paths (e.g., `../iota-sdk` for external monorepo) are allowed
   - Report any relative paths with file:line location and severity: high

2. **Line-Based References (NEW):** Enforce line-based syntax for internal markdown references
   - **FORBID old anchor syntax** for internal files: `patterns.md#testing-standards` (severity: high)
   - **REQUIRE line-based syntax** using one of two formats:
     - Colon syntax: `.claude/guides/backend/patterns.md:145` or `:145:20` (offset or offset:limit)
     - Hash-L syntax: `.claude/guides/backend/patterns.md#L145` or `#L145-L165` (line or range)
   - **VALIDATE line ranges** are within file bounds:
     - Read file to get total line count
     - Verify offset ≤ total lines
     - Verify offset + limit ≤ total lines (colon syntax)
     - Verify end line ≤ total lines (hash-L syntax)
   - Exception: External URLs (starting with http:// or https://) can still use `#anchors`
   - Detection pattern: `#[a-zA-Z-_]` indicates old anchor syntax (not `#L\d+`)
   - Report violations with file:line location and recommended line-based replacement

3. **Anchor Links (DEPRECATED):** Flag old anchor-based references for migration
   - Old style: `patterns.md#testing-standards` → FORBIDDEN for internal files
   - New style: `patterns.md:145:20` or `patterns.md#L145-L165` → REQUIRED
   - Scan for any `#` followed by non-numeric text in internal file references
   - Report each as migration needed with severity: high

**Categories:** `file_paths`, `line_based_references`, `anchor_links_deprecated`
```

**Agent #4: Content Quality Validator**

```
You are validating CONTENT QUALITY AND CONSISTENCY in .claude/ config files and CLAUDE.md files.

**Your Scope (and ONLY your scope):**

1. **Line Number Claims:** Validate line number references
   - Check claims like "see line 145" or "at :712"
   - Verify line content matches claimed context
   - Report outdated line numbers with actual content mismatch

2. **Cross-References:** Validate all inter-file references
   - Ensure referenced sections/files still exist
   - Check for moved or renamed files
   - Validate guide → guide references
   - Verify agent → guide references are accurate

3. **Content Duplication:** Detect duplicate content blocks
   - Each guide file should focus on ONE concern
   - No content duplication between CLAUDE.md and guide files
   - No overlapping content between different guide files
   - Identify substantial content blocks (3+ lines) that appear in multiple files
   - Report duplication with file:line pairs and similarity assessment

4. **Single Responsibility:** Ensure clean separation of concerns
   - Each guide should have one clear purpose
   - Flag guides with mixed concerns (e.g., patterns + routing in one file)
   - CLAUDE.md should reference guides, not duplicate their content
   - If same information appears in multiple places, recommend consolidation

**Categories:** `line_numbers`, `cross_references`, `content_duplication`, `mixed_concerns`
```

### Phase 1: Launch Parallel Agents

**Action:** Launch 4 `Explore` agents simultaneously in a single message with 4 Task tool calls

**CRITICAL:** Use a single message with FOUR Task tool calls (one per agent):

```
<message>
  <Task tool use #1: Explore agent with "IOTA SDK Validator" prompt>
  <Task tool use #2: Explore agent with "Claude Code Validator" prompt>
  <Task tool use #3: Explore agent with "File Structure Validator" prompt>
  <Task tool use #4: Explore agent with "Content Quality Validator" prompt>
</message>
```

**Files All Agents Scan:**

- `.claude/agents/*.md` - Agent definitions
- `.claude/commands/*.md` - Slash command definitions
- `.claude/guides/**/*.md` - Documentation guides
- `CLAUDE.md` - Root orchestration file
- `back/CLAUDE.md` - Backend project instructions
- `bff/CLAUDE.md` - BFF project instructions
- `app/CLAUDE.md` - Frontend project instructions
- `CLAUDE.local.md` - User-specific instructions

**Agent Launch Template:**

- **Agent #1 (IOTA SDK):** Copy full prompt from Phase 0 → Agent #1
- **Agent #2 (Claude Code):** Copy full prompt from Phase 0 → Agent #2
- **Agent #3 (File Structure):** Copy full prompt from Phase 0 → Agent #3
- **Agent #4 (Content Quality):** Copy full prompt from Phase 0 → Agent #4

### Phase 2: Consolidate Agent Reports

**Action:** Wait for all 4 agents to complete, then merge their issues

**Consolidation Strategy:**

1. **Collect Reports** - Retrieve JSON from all 4 agents
2. **Merge Issues** - Combine all issues arrays by category
3. **Deduplicate** - Remove any overlapping issues (same file:line)
4. **Sort by Priority** - Order by severity (high → medium → low)

**Merged Report Structure:**

```json
{
  "issues_by_category": {
    "api_references": [
      ...
    ],
    "agent_command_references": [
      ...
    ],
    "makefile_commands": [
      ...
    ],
    "bash_commands": [
      ...
    ],
    "file_paths": [
      ...
    ],
    "line_based_references": [
      ...
    ],
    "anchor_links_deprecated": [
      ...
    ],
    "line_numbers": [
      ...
    ],
    "cross_references": [
      ...
    ],
    "redundant_claude_refs": [
      ...
    ],
    "bash_placement_violations": [
      ...
    ],
    "content_duplication": [
      ...
    ],
    "mixed_concerns": [
      ...
    ]
  }
}
```

### Phase 3: Quick Reference Guide (For Humans)

All validation is handled by the `Explore` agent. This section provides quick lookup tables for understanding
validation rules.

#### API References

| Package         | Location                       | Validation Method                                         |
|-----------------|--------------------------------|-----------------------------------------------------------|
| `composables.*` | `../iota-sdk/pkg/composables/` | Verify function exists with `grep -r "func FunctionName"` |
| `shared.*`      | `../iota-sdk/pkg/shared/`      | Check function signature matches                          |
| `htmx.*`        | `../iota-sdk/pkg/htmx/`        | Validate helper functions                                 |
| `repo.*`        | `../iota-sdk/pkg/repo/`        | Verify query builder methods                              |
| `itf.*`         | `../iota-sdk/pkg/itf/`         | Check testing framework functions                         |
| `di.*`          | `../iota-sdk/pkg/di/`          | Validate DI container methods                             |

#### CLAUDE.md Orchestration Patterns

| Pattern Type      | Example            | Validation                             |
|-------------------|--------------------|-----------------------------------------|
| Agent reference   | `` `editor` ``     | Must match `.claude/agents/editor.md`   |
| Slash command     | `/commit-pr`       | Must match `.claude/commands/commit-pr.md` |
| Parallel syntax   | `agent1 & agent2`  | Correct workflow syntax                 |
| Sequential syntax | `agent1 && agent2` | Correct workflow syntax                 |
| Scaled agents     | `3x editor`        | Valid pattern for multiple instances    |

#### Redundant CLAUDE.md References

| Source          | Target           | Allowed | Why                                        |
|-----------------|------------------|---------|--------------------------------------------|
| Any file        | CLAUDE.md files  | No      | Auto-loaded into context (redundant noise) |
| Child CLAUDE.md | Parent CLAUDE.md | No      | Both already in context                    |
| Guides          | Other guides     | Yes     | Cross-references are useful                |
| CLAUDE.md       | Guides           | Yes     | References to guides are useful            |

**Validation:** `grep -rE "(CLAUDE\\.md|/CLAUDE\\.md)" .claude/`

#### Bash Command Placement

| Location        | Operational Commands   | Illustrative Examples       |
|-----------------|------------------------|-----------------------------|
| CLAUDE.md files | Yes (project-specific) | Yes                         |
| Guide files     | No (move to CLAUDE.md) | Yes (generic patterns only) |

**Operational:** `make test`, `templ generate`, database/build/deployment commands
**Illustrative:** Generic syntax examples like `grep pattern file`

**Validation:** `grep -E '```(bash\|sh\|shell)' .claude/guides/ -r`

#### Single Responsibility & No Duplication

| Issue Type          | Detection Method                          | Red Flag Example                            |
|---------------------|-------------------------------------------|---------------------------------------------|
| Content duplication | 3+ lines appearing in multiple files      | Same code example in CLAUDE.md and guide    |
| Mixed concerns      | Guide covers multiple topics              | `patterns.md` has both patterns AND routing |
| CLAUDE.md bloat     | Detailed content that should be in guides | Large tables/lists duplicated from guides   |

**Expected:** Each guide has ONE concern (`routing.md` = only paths/structure, `patterns.md` = only code patterns, etc.)

#### File Path References

| Reference Type       | Example                                        | Status      | Validation Method                                   |
|----------------------|------------------------------------------------|-------------|-----------------------------------------------------|
| Absolute paths       | `.claude/guides/backend/patterns.md`           | Valid      | `test -f path`                                      |
| Relative paths       | `../guides/patterns.md`, `./file.md`           | Invalid    | Report as forbidden (use absolute paths)            |
| Bare filenames       | `patterns.md`, `component-library.md`          | Invalid    | Report as forbidden (use absolute paths)            |
| External paths       | `../iota-sdk/pkg/htmx/` (outside EAI project)  | Valid      | Exception for external monorepo                     |
| **Line-based (NEW)** | `.claude/guides/backend/patterns.md:145`       | Valid      | Read from line 145, verify offset ≤ total lines     |
| **Line-based (NEW)** | `.claude/guides/backend/patterns.md:145:20`    | Valid      | Read 20 lines from 145, verify range within bounds  |
| **Line-based (NEW)** | `.claude/guides/backend/patterns.md#L145`      | Valid      | Reference line 145, verify line exists              |
| **Line-based (NEW)** | `.claude/guides/backend/patterns.md#L145-L165` | Valid      | Reference range 145-165, verify range within bounds |
| Anchor links (OLD)   | `.claude/guides/backend/patterns.md#testing`   | FORBIDDEN  | Old syntax - migrate to line-based (severity: high) |
| External URL anchors | `https://example.com/docs#section`             | Valid      | Exception: external URLs can use anchors            |
| Line number claims   | "see line 145" or "at :712"                    | Review     | Check line content matches claimed context          |

**Rules**:

- All internal project references MUST use absolute paths from project root (starting with `.claude/`, `back/`, `bff/`,
  `app/`, etc.)
- All internal markdown cross-references MUST use line-based syntax (`:offset`, `:offset:limit`, `#Lline`, or
  `#Lstart-Lend`)
- Old anchor syntax (`#section-id`) is FORBIDDEN for internal files (migration required)

**Syntax Mapping to Read Tool:**

```
:145        → Read(file_path="...", offset=145)
:145:20     → Read(file_path="...", offset=145, limit=20)
#L145       → Read(file_path="...", offset=145)
#L145-L165  → Read(file_path="...", offset=145, limit=21)  # 165-145+1=21
```

#### Makefile Commands

| Pattern       | Validation                    | Example                     |
|---------------|-------------------------------|-----------------------------|
| `make target` | `grep -E "^target:" Makefile` | `make test` → verify exists |

#### Bash Commands

| Check Type        | Validation             | Example Fix                   |
|-------------------|------------------------|-------------------------------|
| Command syntax    | Verify flags are valid | `go vet .` → `go vet ./...`   |
| Tool availability | Common CLI tools       | `rg pattern` requires ripgrep |

### Phase 4: Apply Fixes (if needed)

**Action:** Review consolidated issues and apply fixes based on priority

**Only proceed if any agent identified issues requiring fixes:**

1. **Review Consolidated Recommendations**
    - Examine merged issues from all 4 agents
    - Verify suggested fixes are accurate and safe
    - Identify which fixes can be automated vs. manual review

2. **Apply Fixes by Priority**
    - Use Edit tool to update references in `.claude/` files
    - Apply fixes in order of priority (see below)
    - Track all changes for final report

3. **Track Changes**
    - Record file:line for each fix applied
    - Note what changed and why
    - Flag any issues that need manual review

**Fix Priority (from Phase 2 consolidated report):**

1. **Redundant CLAUDE.md references** (immediate - architectural violation)
    - WHY: CLAUDE.md is auto-loaded into context, references are redundant noise
    - ACTION: Remove all references to CLAUDE.md files
    - AUTOMATED: Yes

2. **Content duplication violations** (immediate - architectural rule)
    - WHY: Violates single source of truth principle
    - ACTION: Consolidate to one location, update cross-references
    - AUTOMATED: Partial (flag for review)

3. **Mixed concerns in guides** (immediate - architectural rule)
    - WHY: Each guide should have single responsibility
    - ACTION: Split or refocus guide files
    - AUTOMATED: No (requires architectural decision)

4. **Broken file paths** (immediate - critical)
    - WHY: Causes broken navigation and confusion
    - ACTION: Update paths to correct locations
    - AUTOMATED: Yes

5. **Invalid agent/command references** (immediate - critical)
    - WHY: Orchestration will fail with wrong agent names
    - ACTION: Update to correct agent/command names
    - AUTOMATED: Yes

6. **Bash command placement violations** (review - guideline)
    - WHY: Prefer operational commands in CLAUDE.md, not guides
    - ACTION: Review context, relocate if truly operational
    - AUTOMATED: No (requires context judgment)

7. **Old anchor syntax migration** (immediate - architectural rule)
    - WHY: Anchor-based references break when headings change, line-based is more precise
    - ACTION: Convert `#section-id` to `:offset:limit` or `#Lstart-Lend` format
    - AUTOMATED: Partial (requires manual verification of line ranges)

8. **Line range validation failures** (important)
    - WHY: References to non-existent line ranges cause errors
    - ACTION: Update line numbers to valid ranges within file bounds
    - AUTOMATED: Yes

9. **Deprecated API references** (important)
    - WHY: Old SDK APIs may not exist or behave differently
    - ACTION: Update to current IOTA SDK API patterns
    - AUTOMATED: Yes (if mapping known)

10. **Shifted line numbers** (low priority)
    - WHY: Context has changed, claims are outdated
    - ACTION: Update or remove line number claims
    - AUTOMATED: Partial

11. **Outdated makefile commands** (low priority)
    - WHY: Commands may not work as documented
    - ACTION: Update command names/flags
    - AUTOMATED: Yes

12. **Bash syntax issues** (informational)
    - WHY: May cause copy-paste errors
    - ACTION: Fix syntax in examples
    - AUTOMATED: Yes

**Automation Strategy:**

- **Immediate auto-fix:** Redundant refs, file paths, agent/cmd names, line range validation, makefile cmds
- **Flag for review:** Old anchor migration (needs line range lookup), content duplication, mixed concerns, bash
  placement
- **Manual decision:** Architectural changes requiring refactoring

### Phase 5: Generate Final Report

**Action:** Generate comprehensive report showing issues found and fixes applied

**Report shows:**

- Issues found (grouped by category)
- Fixes applied (with file:line)
- Manual review needed (issues requiring human judgment)

```markdown
## Configuration Validation Report

**Issues Found & Fixed:**

### API References

- [file:line]: `old.Function()` → `new.Function()` (Fixed / Needs Review)

### Agent & Command References

- [file:line]: Invalid agent `` `old-agent` `` → `` `new-agent` `` (Fixed)
- [file:line]: Missing command `/old-cmd` (No replacement found)

### File Path References

- [file:line]: Broken path `old/path.md` → `new/path.md` (Fixed)

### Line-Based Reference Violations

- [file:line]: Old anchor syntax `patterns.md#testing-standards` → `patterns.md:145:20` (Fixed / Needs verification)
- [file:line]: Line range `#L500-L550` exceeds file bounds (420 lines) → `#L400-L420` (Fixed)

### Redundant CLAUDE.md References

- [file:line]: File references CLAUDE.md (REMOVE - CLAUDE.md is auto-loaded into context)
- Example: `.claude/guides/backend/patterns.md:42` references `CLAUDE.md` (guide → CLAUDE.md)
- Example: `back/CLAUDE.md:15` references parent `CLAUDE.md` (child → parent, both in context)
- Example: `.claude/commands/foo.md:8` references `back/CLAUDE.md` (pointless, auto-loaded)

### Content Duplication Issues

- [file:line] ↔ [file:line]: Duplicate content found (CONSOLIDATE)
- Example: `CLAUDE.md:50-55` duplicates `.claude/guides/backend/patterns.md:20-25`
- Example: `.claude/guides/frontend/routing.md:30-35` duplicates `.claude/guides/frontend/component-library.md:100-105`

### Mixed Concerns in Guides

- [file:line]: Guide file has multiple concerns (SPLIT or REFOCUS)
- Example: `.claude/guides/backend/patterns.md` contains both implementation patterns AND routing information

### Bash Command Placement Violations

- [file:line]: Bash command in guide file (DISCOURAGED - move to CLAUDE.md or justify)
- Example: `.claude/guides/backend/patterns.md:67` contains `make test` command

### Makefile Commands

- [file:line]: `make old-target` → `make new-target` (Fixed)

### Bash Commands

- [file:line]: `go vet .` → `go vet ./...` (Fixed)

**Manual Review Required:**

- [file:line]: [issue description] - [recommended action]
```

## Success Criteria

- 4 agents launched in parallel and all completed validation
- Agent reports consolidated without duplication
- IOTA SDK references validated (APIs, makefile commands)
- Claude Code orchestration validated (agents, commands, bash syntax, CLAUDE.md refs)
- File structure verified (paths, line-based refs, anchor migration)
- Content quality checked (line numbers, cross-refs, duplication, single responsibility)
- Critical issues fixed automatically, others flagged for manual review
- Comprehensive report generated with categorized issues and fixes

## Error Handling

**Multi-Agent Failure Scenarios:**

- **All 4 agents fail to launch** → Report critical error, abort
- **1-3 agents fail to launch** → Continue with successful agents, note failure in report
- **Agent returns incomplete data** → Use partial results, flag as incomplete
- **Agent timeout** → Cancel agent, report timeout, continue with other agents
- **IOTA SDK path not found** → Report and skip API validation (affects agent #1 only)
- **File read permission denied** → Log error, continue with accessible files
- **Consolidation merge conflict** → Prefer highest severity, keep both if different

**Graceful Degradation:**

- Always generate report even with partial agent results
- Clearly mark which validation domains completed vs. failed
- Provide actionable next steps if some validations failed

---

**Begin by launching 4 `Explore` agents in parallel using a SINGLE MESSAGE with FOUR Task tool calls, using the
specialized prompts from Phase 0:**

- Task #1: IOTA SDK Validator
- Task #2: Claude Code Validator
- Task #3: File Structure Validator
- Task #4: Content Quality Validator

---
allowed-tools: |
  Bash(git status:*), Bash(git log:*), Bash(git diff:*), Bash(git branch:*),
  Bash(find:*), Bash(grep:*), Bash(ls:*), Bash(date:*), Bash(wc:*), Bash(go version:*),
  Read, Write, Grep, Glob
argument-hint: "[optional: custom filename or 'brief' for summary mode]"
description: "Dump comprehensive project context, work state, and progress to a markdown file for resuming work or sharing with team"
---

# Context Dump - Project State Capture

**Usage:**
- `/dump-context` - Create full context dump with timestamp
- `/dump-context mycontext` - Create dump with custom filename
- `/dump-context brief` - Create brief summary without file diffs

## Workflow

### 1. Gather Project Metadata
Collect essential project information:
- Project name and description from CLAUDE.md
- Current timestamp: !`date +"%Y-%m-%d %H:%M:%S %Z"`
- Working directory and SDK paths
- Active user/developer info

### 2. Git State Analysis
Capture complete version control state:
- Current branch: !`git branch --show-current`
- Last 10 commits: !`git log --oneline -10`
- Modified files count: !`git status --porcelain | wc -l`
- Staged vs unstaged changes
- Untracked files list

### 3. Work In Progress Detection
Analyze current work areas:
- Files with recent changes (last 24 hours)
- TODO/FIXME/HACK comments in modified files
- Incomplete test files (tests with t.Skip or missing assertions)
- Files with merge conflicts (if any)

### 4. Module Status Analysis
For each major module (logistics, finance, safety):
- Count of modified files
- List of deleted files (refactoring indicators)
- New files added
- Migration status if applicable

### 5. Active Development Areas
Identify hot spots:
- Controllers with recent changes
- Services under modification
- Repository/persistence layer changes
- Template (templ) files updated
- Database migrations pending

### 6. Build & Test State
Capture development environment status:
- Last successful build time (if available)
- Test coverage for modified packages
- Failing tests (if any)
- Linting issues in modified files

### 7. Dependencies & Configuration
Document environment:
- Go version: !`go version`
- Key environment variables (non-sensitive)
- Active MCP servers (from settings.json)
- Database connection status

### 8. TODO Extraction
Collect action items from:
- TODO comments in code
- FIXME annotations
- Incomplete implementations (panic("not implemented"))
- Skipped tests with reasons

### 9. Context Reconstruction Hints
Provide resumption guidance:
- Next logical steps based on current state
- Blocking issues or dependencies
- Suggested agent delegations for remaining work
- Commands to run for validation

### 10. Generate Output File

Create structured markdown file at: `CONTEXT_DUMP_[timestamp].md` or custom filename

## Output Format

```markdown
# Project Context Dump - IOTA SDK
Generated: [timestamp]
Branch: [branch_name]
Developer: [user]

## Executive Summary
[2-3 sentence overview of current state]

## Git Status
### Current Branch
- Branch: [name]
- Tracking: [remote/branch]
- Behind/Ahead: [counts]

### Recent Activity
[Last 10 commits]

### Modified Files ([count] total)
#### Logistics Module
- Controllers: [list]
- Services: [list]
- Repositories: [list]
- Templates: [list]

#### Finance Module
- [Similar breakdown]

#### Deleted/Moved Files
[List of removed files indicating refactoring]

## Work In Progress

### Active Development Areas
1. [Area 1]: [Description and files]
2. [Area 2]: [Description and files]

### TODO Items
Location | Priority | Description
---------|----------|------------
[file:line] | HIGH | [TODO text]
[file:line] | MEDIUM | [TODO text]

### Incomplete Features
- [ ] [Feature 1]: [Status and next steps]
- [ ] [Feature 2]: [Status and next steps]

## Module Migration Status
### Finance Module Restructuring
- Files moved: [count]
- Files remaining: [count]
- Integration points to update: [list]

## Testing & Quality
### Test Coverage
- Modified packages with tests: [list]
- Packages needing tests: [list]
- Skipped tests: [count with reasons]

### Build Status
- Last build: [status]
- Linting issues: [count]
- go vet warnings: [count]

## Environment & Configuration
- Go Version: [version]
- PostgreSQL: [status]
- Redis: [status]
- IOTA SDK: ../iota-sdk/

## Next Steps (Recommended)
1. [Immediate action needed]
2. [Next priority task]
3. [Follow-up items]

## Agent Delegation Suggestions
Based on current state:
- Use `go-editor` for: [specific Go code changes and test coverage]
- Use `database-expert` for: [migration needs]
- Use `ui-editor` for: [template and frontend changes]
- Use `refactoring-expert` for: [final review before deploy]

## Quick Resume Commands
```bash
# Validate current state
go vet ./...
make test

# Check specific areas
git diff --stat modules/finance/
git diff --stat modules/logistics/

# Review TODOs
grep -r "TODO" --include="*.go" modules/
```

## File Change Summary
$ARGUMENTS != "brief" ? [Include detailed diff stats] : [Skip]

### Detailed Changes
[If not brief mode, include file-by-file summary]

---
Context dump complete. Share this file to transfer project state.
```

## Implementation Logic

1. **Timestamp Generation**: Use ISO format for sorting: `date +"%Y%m%d_%H%M%S"`
2. **Smart Filtering**: Exclude vendor/, node_modules/, generated files
3. **TODO Prioritization**: HIGH for broken tests, MEDIUM for missing features, LOW for optimizations
4. **Diff Summarization**: Group by module and operation type (add/modify/delete)
5. **State Persistence**: Save to project root for easy access

## Success Criteria
- Captures enough context for another developer to continue work
- Identifies all work in progress and blocking issues
- Provides clear next steps and agent delegations
- Completes in <30 seconds for large projects
- Output file is readable and well-structured

## Error Handling
- Handle missing git repository gracefully
- Skip inaccessible files with note
- Provide partial dump if some commands fail
- Always generate output file even if incomplete
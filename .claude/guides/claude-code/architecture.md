Core architectural concepts, patterns, and principles for Claude Code configuration.

## Key Architectural Patterns

### Commands vs Agents vs MCP

- **Commands**: Extend Claude's interface with custom workflows; use dynamic context loading. Shared context.
- **Agents**: Isolated specialists for focused tasks with separate context windows. No interference between agents.
- **MCP Servers**: External integrations (GitHub, databases, APIs) with isolated, secure execution.

### CLAUDE.md Loading in Monorepos

Claude loads CLAUDE.md files hierarchically from root to nested directories:

- Parent files load first (root CLAUDE.md applies everywhere)
- Child files load on-demand when accessing subdirectories (e.g., `back/CLAUDE.md`)
- All files are additive; nested definitions override parent definitions on conflicts
- **Example**: `eai/back/` loads root + `back/CLAUDE.md`; root sets orchestration rules, `back/CLAUDE.md` adds
  backend-specific patterns

### Context Isolation Benefits

- Prevents "context pollution" of main conversation
- Each agent maintains focused expertise without distraction
- Enables safe parallel execution without interference
- Cleaner handoffs between specialized roles

### MCP (Model Context Protocol)

Enables Claude Code to connect to external tools, databases, and APIs. See `.claude/guides/claude-code/settings.md` for
full MCP configuration, security best practices, and examples.

Key points:

- **Tool Naming**: `mcp__<server>__<tool>` (e.g., `mcp__github__create_issue`)
- **Credentials**: Always use `${ENV_VAR}` expansion, never hardcode
- **Output Limits**: Set per-server to prevent context overflow
- **Security**: Vet servers carefully; third-party servers unverified by Anthropic

## Core Principles

### 1. Separation of Concerns

Each agent MUST have a single, clear responsibility with zero overlap. If two agents handle the same file type,
consolidate or clarify boundaries explicitly.

### 2. No Information Duplication

One canonical source of truth per concept. Reference via cross-references (§ X.Y notation) instead of repeating.
Consolidate redundant rules across files.

### 3. Token Efficiency & Conciseness

Every word must earn its place. Prefer tables and bullets over paragraphs. Remove filler words. Example: "You should
make sure to always verify" → "Verify"

### 4. Clarity & No Ambiguity

Use specific, actionable language. Define exact file patterns (`*_controller.go` not "controller files") and exact
commands (`go vet ./...` not "check for errors"). Use "MUST" for requirements, "should" for recommendations, "can" for
options.

### 5. Spelling & Grammar Quality

Run spell check on all content. Verify technical terms (templ, HTMX, PostgreSQL, etc.) and consistent terminology across
files. Use proper markdown syntax.

### 6. Holistic Impact Analysis

Think system-wide: "What breaks if I change this?" and "What else needs updating?" Maintain consistency across all files
and update cross-references when moving/renaming content.

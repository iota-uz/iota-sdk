# Agent Skills Guide

Create, manage, and share Skills to extend Claude's capabilities in Claude Code.

## What are Agent Skills?

Agent Skills package expertise into discoverable capabilities. Each Skill consists of a `SKILL.md` file with instructions that Claude reads when relevant, plus optional supporting files like scripts and templates.

**Key characteristics:**

- **Model-invoked**: Claude autonomously decides when to use Skills based on your request and the Skill's description
- **Different from slash commands**: Commands are user-invoked (you type `/command`), Skills are automatic
- **Composable**: Multiple Skills can work together for complex tasks

## Skill Types

### Personal Skills (`~/.claude/skills/`)

Available across all your projects.

**Use for:**
- Individual workflows and preferences
- Experimental Skills you're developing
- Personal productivity tools

### Project Skills (`.claude/skills/`)

Shared with your team via git.

**Use for:**
- Team workflows and conventions
- Project-specific expertise
- Shared utilities and scripts

### Plugin Skills

Bundled with Claude Code plugins, automatically available when the plugin is installed.

## Create a Skill

### 1. Create directory structure

```bash
# Personal Skill
mkdir -p ~/.claude/skills/my-skill-name

# Project Skill (recommended for teams)
mkdir -p .claude/skills/my-skill-name
```

### 2. Write SKILL.md

Create a `SKILL.md` file with YAML frontmatter and Markdown content:

```yaml
---
name: Your Skill Name
description: Brief description of what this Skill does and when to use it
---

# Your Skill Name

## Instructions
Provide clear, step-by-step guidance for Claude.

## Examples
Show concrete examples of using this Skill.
```

**Critical: The `description` field determines when Claude uses your Skill.**

### 3. Add supporting files (optional)

```
my-skill/
├── SKILL.md (required)
├── reference.md (optional documentation)
├── examples.md (optional examples)
├── scripts/
│   └── helper.py (optional utility)
└── templates/
    └── template.txt (optional template)
```

Reference these files from SKILL.md:

````markdown
For advanced usage, see [reference.md](reference.md).

Run the helper script:
```bash
python scripts/helper.py input.txt
```
````

Claude uses progressive disclosure - only reads files when needed.

## Restrict Tool Access with allowed-tools

Use `allowed-tools` frontmatter to limit which tools Claude can use when a Skill is active:

```yaml
---
name: Safe File Reader
description: Read files without making changes. Use when you need read-only file access.
allowed-tools: Read, Grep, Glob
---

# Safe File Reader

This Skill provides read-only file access.

## Instructions
1. Use Read to view file contents
2. Use Grep to search within files
3. Use Glob to find files by pattern
```

**When to use allowed-tools:**
- Read-only Skills that shouldn't modify files
- Skills with limited scope (e.g., only data analysis)
- Security-sensitive workflows where you want to restrict capabilities

If `allowed-tools` is not specified, Claude follows the standard permission model.

## Writing Effective Skills

### Keep Skills Focused

One Skill = one capability.

**Good (focused):**
- "PDF form filling"
- "Excel data analysis"
- "Git commit messages"

**Bad (too broad):**
- "Document processing" → Split into separate Skills
- "Data tools" → Split by data type or operation

### Write Clear Descriptions

Help Claude discover when to use Skills by including specific triggers:

**Clear:**
```yaml
description: Analyze Excel spreadsheets, create pivot tables, and generate charts. Use when working with Excel files, spreadsheets, or analyzing tabular data in .xlsx format.
```

**Vague:**
```yaml
description: For files
```

Include both:
1. What the Skill does
2. When Claude should use it (trigger keywords)

### Provide Examples

Show concrete usage examples in your Skill:

````markdown
## Examples

### Extract text from PDF

```python
import pdfplumber
with pdfplumber.open("doc.pdf") as pdf:
    text = pdf.pages[0].extract_text()
```

### Fill PDF form

```bash
python scripts/fill_form.py input.pdf output.pdf --field "Name" "John Doe"
```
````

### Document Dependencies

List required packages in the description and instructions:

```yaml
---
name: PDF Processing
description: Extract text, fill forms, merge PDFs. Use when working with PDF files. Requires pypdf and pdfplumber packages.
---

# PDF Processing

## Requirements

Packages must be installed in your environment:
```bash
pip install pypdf pdfplumber
```
```

Claude will ask for permission to install dependencies when needed.

## Test and Debug

### View Available Skills

Ask Claude directly:

```
What Skills are available?
```

or

```
List all available Skills
```

### Test a Skill

Ask questions that match your description:

```
Can you help me extract text from this PDF?
```

Claude autonomously decides to use your Skill if it matches - you don't explicitly invoke it.

### Debug Common Issues

#### Skill not activating

**Check description specificity:**

```yaml
# Too vague
description: Helps with documents

# Specific
description: Extract text and tables from PDF files, fill forms, merge documents. Use when working with PDF files or when the user mentions PDFs, forms, or document extraction.
```

**Verify file path:**

```bash
# Personal Skills
ls ~/.claude/skills/*/SKILL.md

# Project Skills
ls .claude/skills/*/SKILL.md
```

**Check YAML syntax:**

```bash
cat SKILL.md | head -n 10
```

Ensure:
- Opening `---` on line 1
- Closing `---` before Markdown content
- Valid YAML syntax (no tabs, correct indentation)

**View errors with debug mode:**

```bash
claude --debug
```

#### Multiple Skills conflict

Be specific in descriptions with distinct trigger terms:

```yaml
# Skill 1
description: Analyze sales data in Excel files and CRM exports. Use for sales reports, pipeline analysis, and revenue tracking.

# Skill 2
description: Analyze log files and system metrics data. Use for performance monitoring, debugging, and system diagnostics.
```

## Share Skills with Your Team

### Option 1: Project Skills (Simple)

1. Create Skill in project:
   ```bash
   mkdir -p .claude/skills/team-skill
   # Create SKILL.md
   ```

2. Commit to git:
   ```bash
   git add .claude/skills/
   git commit -m "Add team Skill for PDF processing"
   git push
   ```

3. Team members get Skills automatically:
   ```bash
   git pull
   claude  # Skills are now available
   ```

### Option 2: Plugin Distribution (Recommended)

See [Claude Code plugins documentation](https://docs.claude.com/en/docs/claude-code/plugins) for creating and distributing plugins with Skills.

## Manage Skills

### Update a Skill

Edit SKILL.md directly:

```bash
# Personal Skill
code ~/.claude/skills/my-skill/SKILL.md

# Project Skill
code .claude/skills/my-skill/SKILL.md
```

Restart Claude Code to load changes.

### Remove a Skill

Delete the Skill directory:

```bash
# Personal
rm -rf ~/.claude/skills/my-skill

# Project
rm -rf .claude/skills/my-skill
git commit -m "Remove unused Skill"
```

### Version Documentation

Document Skill versions in SKILL.md content:

```markdown
# My Skill

## Version History
- v2.0.0 (2025-10-01): Breaking changes to API
- v1.1.0 (2025-09-15): Added new features
- v1.0.0 (2025-09-01): Initial release
```

## Example Skills

### Simple Skill (single file)

```
commit-helper/
└── SKILL.md
```

```yaml
---
name: Generating Commit Messages
description: Generates clear commit messages from git diffs. Use when writing commit messages or reviewing staged changes.
---

# Generating Commit Messages

## Instructions

1. Run `git diff --staged` to see changes
2. I'll suggest a commit message with:
   - Summary under 50 characters
   - Detailed description
   - Affected components

## Best practices

- Use present tense
- Explain what and why, not how
```

### Skill with Tool Permissions

```
code-reviewer/
└── SKILL.md
```

```yaml
---
name: Code Reviewer
description: Review code for best practices and potential issues. Use when reviewing code, checking PRs, or analyzing code quality.
allowed-tools: Read, Grep, Glob
---

# Code Reviewer

## Review checklist

1. Code organization and structure
2. Error handling
3. Performance considerations
4. Security concerns
5. Test coverage

## Instructions

1. Read the target files using Read tool
2. Search for patterns using Grep
3. Find related files using Glob
4. Provide detailed feedback on code quality
```

### Multi-file Skill

```
pdf-processing/
├── SKILL.md
├── FORMS.md
├── REFERENCE.md
└── scripts/
    ├── fill_form.py
    └── validate.py
```

**SKILL.md:**

````yaml
---
name: PDF Processing
description: Extract text, fill forms, merge PDFs. Use when working with PDF files, forms, or document extraction. Requires pypdf and pdfplumber packages.
---

# PDF Processing

## Quick start

Extract text:
```python
import pdfplumber
with pdfplumber.open("doc.pdf") as pdf:
    text = pdf.pages[0].extract_text()
```

For form filling, see [FORMS.md](FORMS.md).
For detailed API reference, see [REFERENCE.md](REFERENCE.md).

## Requirements

Packages must be installed in your environment:
```bash
pip install pypdf pdfplumber
```
````

## Best Practices Checklist

- [ ] Skill has a clear, focused purpose (one capability)
- [ ] Description includes what it does AND when to use it
- [ ] Description includes specific trigger keywords
- [ ] YAML frontmatter is valid (no tabs, proper formatting)
- [ ] Instructions are clear and step-by-step
- [ ] Examples show concrete usage
- [ ] Dependencies are documented in description
- [ ] File paths use forward slashes (Unix style)
- [ ] Scripts have execute permissions if needed
- [ ] Tested with questions matching the description
- [ ] Team members have reviewed (for project Skills)

## Core Principles

Apply these principles when creating Skills:

1. **Separation of Concerns**: One Skill = one capability
2. **No Duplication**: Skills should complement, not duplicate each other
3. **Token Efficiency**: Use progressive disclosure (reference files only when needed)
4. **Clarity**: Clear descriptions with specific trigger keywords
5. **Holistic Impact**: Consider how Skills work together

## Further Reading

- Official docs: https://docs.claude.com/en/docs/claude-code/skills
- Agent Skills overview: https://docs.anthropic.com/en/docs/agents-and-tools/agent-skills/overview
- Best practices: https://docs.anthropic.com/en/docs/agents-and-tools/agent-skills/best-practices
- Engineering blog: https://www.anthropic.com/engineering/equipping-agents-for-the-real-world-with-agent-skills

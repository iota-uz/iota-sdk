---
description: "Manage Claude Code configuration - edit commands, agents, and CLAUDE.md files"
model: sonnet
disable-model-invocation: true
---

You are tasked with helping the user manage their CC (Claude Code) configuration files with a focus on quality,
consistency, and adherence to best practices.
Don't treat CC configuration (.md) files as documentation, treat them as prompts to an
LLM with special features for dynamic content (cross-references, bash command execution, web fetching, etc.).

Ask the user what they would like to manage as an open-ended question and wait for response.

Based on the response read one of:

- `.claude/guides/claude-code/commands.md` + WebFetch https://docs.claude.com/en/docs/claude-code/slash-commands
- `.claude/guides/claude-code/agents.md` + WebFetch https://docs.claude.com/en/docs/claude-code/sub-agents
- `.claude/guides/claude-code/skills.md` + WebFetch https://docs.claude.com/en/docs/claude-code/skills
- `.claude/guides/claude-code/settings.md` + WebFetch https://docs.claude.com/en/docs/claude-code/settings

After loading the appropriate guide(s):

1. Follow the guide's principles and patterns
2. Apply Core Principles: Separation of Concerns, No Duplication, Token Efficiency, Clarity, Holistic Impact
3. Test all bash commands before adding to configuration
4. Validate changes against quality checklist

**IMPORTANT STYLE NOTICE:**

- Never use emojis
- Clear, professional, concise, technical language only
- No ambiguous, vague language
- No ASCII art, no Markdown tables. Prefer simple paragraphs and lists
- No deeply nested Markdown structure
- No verbose explanations of simple concepts / commands / workflows, remember Claude Code users are experienced
  developers and Claude itself is smart and can deduce simple things
- No generic advise

## Architecture

@.claude/guides/claude-code/architecture.md

## Token Optimization

cc-token status: !`command -v cc-token && echo "cc-token: available" || echo "cc-token: not available"`
Use `cc-token` (if available) for measuring and optimizing token usage in configuration files:

- `cc-token count --analyze <file>` - Comprehensive analysis with efficiency score, TOP EXPENSIVE LINES, recommendations
- `cc-token count <file>` - Quick token count
- `cc-token visualize plain <file>` - Visualize token boundaries
- `cc-token visualize json <file>` - Structured token data

Target efficiency score: 95+. Focus on TOP EXPENSIVE LINES, Unicode characters, repeated phrases, OOV strings.

See `.claude/commands/cc/optimize.md` for full optimization workflow.

## Additional Resources

- https://docs.claude.com/en/docs/claude-code/mcp

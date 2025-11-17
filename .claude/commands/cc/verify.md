---
description: "Verify recently edited files against quality principles and clean up temporary artifacts"
disable-model-invocation: true
---

Review CC config files (.claude/**/*.md, CLAUDE.md, *.local.md) against the principles below. Be very thorough.

* Completeness - all required elements present
* Correctness - logic is sound, references accurate
* Cohesion - related functionality grouped together
* Consistency - naming and formatting match project style
* DRY - no duplication, single source of truth
* Separation of Concerns - clear boundaries, one purpose per unit
* Lack of Ambiguity - clear instructions, no "or" statements, one agent per task, unclear intent / instructions
* Token Efficiency - concise, pipe-separated lists, table format
* For Slash commands verify each Bash command in the file runs without errors
* Verify each dynamic execution (!\`command\` syntax) in the file is whitelisted in .claude/settings.json
* Verify file paths are correct

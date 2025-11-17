---
description: "Verify recently edited files against quality principles and clean up temporary artifacts"
model: haiku
disable-model-invocation: true
---

Clean up temporary artifacts, remove temporary files created during current session and verify code quality.
Clean up these files:

* Temporary files like: *.temp, *.backup, *.back (except go.work.back*)
* Binaries (compiled executables without extensions or with platform-specific extensions)
* Random uncommitted markdown files (*.md) outside .claude directory
* Random uncommitted sql/bash/shell scripts

Review code files (*.go, *.templ, *.sql, *.toml) against the following principles:

* Completeness - all required elements present
* Correctness - logic is sound, references accurate
* No unimplemented features (Unresolved TODOs, FIXMEs, placeholders)
* No bugs
* Cohesion - related functionality grouped together
* Consistency - naming and formatting match project style
* DRY - no duplication, single source of truth
* Separation of Concerns - clear boundaries, one purpose per unit
* No dead / unused code
* No unnecessary complexity
* No unnecessary dependencies
* No unnecessary logging
* No unnecessary comments
* No unnecessary variables

After you have cleaned up temporary files and verified code quality, do a run of qa-tester agent to spot potential bugs.
---
description: "Load context summary from DUMP_*.md files for work resumption (usage: /load-dump or /load-dump N)"
argument-hint: "[DUMP_NUMBER]"
disable-model-invocation: true
---

Available context dumps: !`ls -t .claude/dumps/DUMP_*.md 2>/dev/null || echo "No DUMP files found"`

1. **Determine target file:**
    - If `$1` provided: Load `.claude/dumps/DUMP_$1.md` directly and skip to step 4
    - If no argument: Continue to step 2
2. **Read dump file summaries:**
    - For each DUMP_*.md file, read the top 40 lines using the Read tool with `limit: 40`
    - Extract key information from each dump:
        - User Intent section (what task was being worked on)
        - Key Findings & Decisions section (main insights)
        - Current Phase/Status (what was completed, what's pending)
    - Create a concise 1-2 sentence summary for each dump file highlighting the main task/issue
3. **Present options using AskUserQuestion:**
    - Create options for each DUMP file found
    - Label: Dump number (e.g., "Dump 2")
    - Description: The 1-2 sentence summary extracted from step 2, including phase/status if available
    - Header: "Select Dump"
    - Question: "Which context dump would you like to load?"
    - Set multiSelect to false (single selection)
4. Load selected file based on user's choice from AskUserQuestion (or from $1 argument) and ask if user wants to resume
   the work described in the dump

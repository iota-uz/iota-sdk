---
description: "Optimize token usage in a Claude Code configuration file by making actual edits"
argument-hint: "<file-path>"
model: sonnet
disable-model-invocation: true
---

Optimize token usage in $1 by analyzing and implementing improvements.

## Pre-Analysis Visualization Phase

1. Run `cc-token visualize plain $1 | head -100` to inspect token boundaries in expensive sections
2. Run `cc-token visualize json $1` to get structured token data for programmatic analysis
3. Use stdin to analyze specific sections:
    ```bash
    # Visualize expensive line context (replace 12,17 with actual line range)
    sed -n '12,17p' $1 | cc-token visualize plain -

    # Compare section densities to find hotspots
    head -n 50 $1 | cc-token count -   # First section
    tail -n 50 $1 | cc-token count -   # Last section
    ```
4. Identify problematic token splits (subword tokens, OOV strings)

## Analysis Phase

1. Run `cc-token count --analyze $1` to get comprehensive analysis:
    - Efficiency Score (target: 95+)
    - Token Density Heatmap (identifies heavy sections)
    - Category Breakdown (prose/code/formatting/whitespace)
    - TOP EXPENSIVE LINES (exact line numbers to target)
    - OPTIMIZATION RECOMMENDATIONS (concrete suggestions)

2. Read the file to understand current content and structure

3. Parse analysis output to prioritize optimizations:
    - Target lines from "TOP EXPENSIVE LINES" section
    - Apply recommendations from "OPTIMIZATION RECOMMENDATIONS"
    - Common patterns:
        * Unicode box-drawing characters (use -, *, + instead)
        * Repeated phrases (abbreviate or reference)
        * OOV strings (use semantic placeholders like <URL>, <HASH>)
        * Verbose prose (bullet points/tables)
        * Inline content (extract to `.claude/guides/`)
        * Excessive examples (consolidate essential cases)
        * Long paragraphs (structured lists)

4. Test pattern replacements with stdin before applying:
    ```bash
    # Test if removing Unicode arrows reduces tokens
    cat $1 | sed 's/→/->/g' | cc-token count --yes -

    # A/B test rewrites (pick lowest token count)
    echo "original verbose sentence from line X" | cc-token count --yes -
    echo "concise alternative version" | cc-token count --yes -
    echo "very concise version" | cc-token count --yes -
    ```

## Implementation Phase

1. Test edits non-destructively with stdin before writing:
    ```bash
    # Test a potential edit without modifying file
    cat $1 | sed 's/verbose pattern/concise/' | cc-token count --yes -

    # A/B test multiple rewrite candidates (pick lowest)
    echo "Option 1: original wording from line X" | cc-token count --yes -
    echo "Option 2: concise alternative" | cc-token count --yes -
    echo "Option 3: very concise version" | cc-token count --yes -

    # Verify token boundaries in rewrite
    echo "new version of expensive line" | cc-token visualize json --yes -
    ```

2. Apply optimizations iteratively targeting specific lines:
    - Test edit with stdin first (see above)
    - Make ONE targeted edit to high-token lines from analysis
    - Run `cc-token count $1` (no --analyze) to verify token reduction
    - If tokens increased or stayed same, revert and try different approach
    - If tokens decreased, proceed to next optimization
    - Focus on:
        * Lines identified in TOP EXPENSIVE LINES
        * Specific recommendations from analysis output
        * Token density hotspots from heatmap

3. Re-run `cc-token count --analyze $1` for final measurement

## Output Format

Show concrete changes made matching cc-token's analysis structure:

```
TOKEN OPTIMIZATION RESULTS
File: [path]

BEFORE OPTIMIZATION
Total: X tokens across Y lines (Z tokens/line)
Efficiency Score: N/100
Token Density: [density distribution]
Category Breakdown:
  Prose:       X tokens (Y%)
  Code Blocks: X tokens (Y%)
  Formatting:  X tokens (Y%)
  Whitespace:  X tokens (Y%)

OPTIMIZATIONS APPLIED

1. [Optimization from analysis]: [description]
   Lines targeted: L1, L2, L3 (from TOP EXPENSIVE LINES)
   Before: X tokens | After: Y tokens
   Savings: Z tokens

2. [Optimization from recommendations]: [description]
   Applied to: [specific pattern/content]
   Before: X tokens | After: Y tokens
   Savings: Z tokens

[Additional optimizations...]

AFTER OPTIMIZATION
Total: X tokens across Y lines (Z tokens/line)
Efficiency Score: N/100 (improved from M/100)
Token Density: [new density distribution]

SUMMARY
- Total edits: N changes
- Token reduction: X → Y (-Z tokens, W% reduction)
- Efficiency gain: M → N (+P points)
- Cost savings: ~$X.XX per invocation
```

## Guidelines

- **CRITICAL**: Verify token count after EVERY edit - never skip this step
- Use visualization to understand token boundaries before optimizing
- Target specific lines from TOP EXPENSIVE LINES analysis output
- Apply concrete recommendations from OPTIMIZATION RECOMMENDATIONS section
- Revert any change that increases token count or provides no benefit
- Make ONE edit at a time, verify, then proceed to next
- Prioritize high-impact optimizations:
    1. Lines from TOP EXPENSIVE LINES (highest token density)
    2. Recommendations from analysis (Unicode, repeated phrases, OOV strings)
    3. Category imbalances (excessive prose/formatting)
- Preserve all critical information and instructions
- Maintain file purpose and functionality
- Keep examples essential for understanding
- Ensure edits don't break references or syntax
- Track efficiency score improvements (target: 95+)

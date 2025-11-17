---
description: "Analyze CI workflow $1 performance and interactively apply optimizations"
argument-hint: "<workflow-name>"
model: sonnet
---

# CI Optimize Command

Analyze CI workflow performance, identify bottlenecks, and interactively apply optimizations.

Usage: `/ci:optimize <workflow-name>`

Examples: `/ci:optimize lint` or `/ci:optimize publish`

## Key Documentation

- Full docs: [1]
- Docker caching: [2]
- Cost reduction: [3]
- Dockerfile optimization: [4]
- Cache is King guide: [5]
- Dashboard: https://app.blacksmith.sh

[1]: https://docs.blacksmith.sh/llms-full.txt
[2]: https://docs.blacksmith.sh/blacksmith-caching/docker-builds
[3]: https://www.blacksmith.sh/blog/how-to-reduce-spend-in-github-actions
[4]: https://www.blacksmith.sh/blog/how-to-optimize-dockerfile-faster-docker-builds
[5]: https://www.blacksmith.sh/blog/cache-is-king-a-guide-for-docker-layer-caching-in-github-actions

## Workflow

### Phase 1: Data Collection

Workflow to analyze: $1

1. **Validate workflow exists**:

```bash
gh workflow list --json name,path | jq -r '.[] | select(.name | test("$1"; "i")) | .path'
```

If no match found, list available workflows and ask user to specify correct name.

2. **Fetch recent runs** (last 30):

```bash
gh run list --workflow "$1" --limit 30 --json databaseId,startedAt,updatedAt,conclusion,createdAt
```

3. **Calculate baseline metrics**:

- Average duration: `(updatedAt - startedAt)` in seconds
- P50, P90, P95 percentiles
- Success rate: `conclusion == "success"` count / total
- Get top 3 most recent successful runs for detailed analysis

4. **Get detailed step timings** for representative runs:

```bash
gh run view RUN_ID --json jobs
```

Extract for each job:
- Job name, duration
- Each step: name, `startedAt`, `completedAt`
- Calculate step duration: `(completedAt - startedAt)`
- Sort steps by duration descending

5. **Identify top 5 slowest steps** across analyzed runs (with percentage of total time)

### Phase 2: Analysis

Detect optimization opportunities by checking workflow YAML and run data:

**Read workflow file** from path obtained in Phase 1 using Read tool.

**Check for common issues**:

1. **Missing concurrency control** (High Impact):
   - Pattern: No `concurrency:` + `cancel-in-progress: true` on PR-triggered workflows
   - Expected improvement: -30% wasted runs on rapid commits

2. **Missing timeouts** (Medium Impact):
   - Pattern: No `timeout-minutes:` on jobs
   - Recommend: 3x average duration (from metrics)
   - Expected improvement: Prevent indefinite hangs

3. **Slow setup steps** (High Impact):
   - Pattern: Setup steps (Go, Node, Docker) taking >30s
   - Check: Is Blacksmith action used? (`useblacksmith/setup-*`)
   - Expected improvement: -40-60% setup time

4. **Missing fail-fast on matrix** (Low Impact):
   - Pattern: `strategy: matrix:` without `fail-fast: true`
   - Expected improvement: -5-10% on failures

5. **Dockerfile optimization** (Medium Impact):
   - If workflow contains Docker build steps, check Dockerfile
   - Pattern: Single-stage, `COPY . .` before dependency layers
   - Expected improvement: -20-40% build time

6. **Missing workflow telemetry** (Observability):
   - Pattern: No `catchpoint/workflow-telemetry-action` step
   - Benefit: CPU/memory/disk metrics for debugging

7. **Wrong runner size** (Cost/Performance):
   - Compare job duration vs CPU cores used
   - If job completes in <2min on 8vcpu runner, suggest 2vcpu
   - If job takes >5min on 2vcpu with high CPU usage, suggest 8vcpu

**Flag unusual patterns** for researcher:

- Any step taking >120s (2 minutes)
- Success rate <70%
- Duration variance >50% (P95/P50 ratio)
- Unexplained recent performance degradation (compare first 10 vs last 10 runs)

If flagged, use Task tool with subagent_type=researcher:

Investigate performance issue in $1 workflow:

Problem detected:
- [SPECIFIC_ISSUE: e.g., "Setup Go step taking 58s, expected <20s"]
- Baseline: [METRICS]

Research tasks:
1. Search Blacksmith documentation for optimization of [SPECIFIC_COMPONENT]
   - Start with: [1]
   - Focus on caching strategies, runner configuration
2. Find similar workflow optimization case studies
3. Check GitHub Actions best practices for [ISSUE_TYPE]

Return:
- Root cause analysis
- Specific optimization recommendations with expected improvements
- Configuration examples
- Relevant documentation links

### Phase 3: Report Generation

Generate markdown report:

```markdown
## Workflow Performance Report: $1

### Baseline (Last 30 runs)

- **Average Duration**: [X]m [Y]s
- **P50**: [X]m [Y]s | **P90**: [X]m [Y]s | **P95**: [X]m [Y]s
- **Success Rate**: [Z]% ([SUCCESS]/30 successful)

### Top 5 Slowest Steps

1. [Step Name] - [X]s ([Y]% of total) [SLOW if >30s]
2. [Step Name] - [X]s ([Y]% of total)
3. [Step Name] - [X]s ([Y]% of total)
4. [Step Name] - [X]s ([Y]% of total)
5. [Step Name] - [X]s ([Y]% of total)

### Optimization Opportunities

#### HIGH IMPACT

[For each high-impact issue:]

**[N]. [Issue Title]** (Expected: [IMPROVEMENT])

- **Current**: [CURRENT_STATE]
- **Issue**: [DESCRIPTION]
- **Fix**: [SPECIFIC_ACTION]
- **Docs**: [BLACKSMITH_LINK]

#### MEDIUM IMPACT

[Same format]

#### LOW IMPACT

[Same format]

### Blacksmith Dashboard Recommendations

Review these metrics manually at https://app.blacksmith.sh:

- **Cache Analytics**: Check hit rates for Go modules, Docker layers
- **Job Monitoring**: Step-by-step duration trends, failure patterns
- **Cost Breakdown**: Spending by job, identify expensive operations
- **Test Analytics** (Beta): Failing test trends

### Next Steps

[Generated in Phase 4 based on user selections]
```

### Phase 4: Interactive Fixes

Present optimizations to user using AskUserQuestion tool:

**Question**: "Which optimizations would you like to apply to $1?"
**Multi-select**: true
**Options**: [Generated from detected issues, max 4 highest-priority items]

Example options:
- "Add concurrency control (cancel old runs on new commits)"
- "Add timeout protection (15min based on P95 duration)"
- "Add fail-fast to matrix strategy"
- "Add workflow telemetry for observability"

Based on selections, use Task tool with subagent_type=editor:

Apply the following CI optimizations to $1 ([PATH]):

User selected:
- [OPTIMIZATION_1]
- [OPTIMIZATION_2]
...

For each optimization:

1. **Add concurrency control**:
   Add after `on:` block:
   ```yaml
   concurrency:
     group: ${{ github.workflow }}-${{ github.ref }}
     cancel-in-progress: true
   ```

2. **Add timeout**:
   Add to each job:
   ```yaml
   jobs:
     job-name:
       timeout-minutes: [RECOMMENDED_VALUE]
   ```

3. **Add fail-fast**:
   Modify matrix strategy:
   ```yaml
   strategy:
     fail-fast: true
     matrix:
       ...
   ```

4. **Add workflow telemetry**:
   Add step to each job (after checkout):
   ```yaml
   - name: Workflow Telemetry
     uses: catchpoint/workflow-telemetry-action@v2
     with:
       proc_trace_sys_enable: true
       comment_on_pr: true
   ```

Ensure:
- Preserve existing workflow structure
- Maintain proper YAML indentation
- Don't duplicate existing keys
- Test YAML validity after changes
```

If Dockerfile optimization selected, provide specific diff:

```markdown
Optimize [DOCKERFILE_PATH] for better layer caching:

Current issues:
- [ISSUE_1: e.g., "COPY . . before dependency installation"]
- [ISSUE_2: e.g., "Single-stage build includes build artifacts"]

Apply this optimization pattern:
[Provide specific Dockerfile diff based on detected language/framework]

Example for Go application:
- Separate dependency download layer (COPY go.mod go.sum)
- Separate build assets layer (COPY static/ templates/)
- Multi-stage build (builder + runtime stages)
- Order layers by change frequency (least to most)

See: [4]
```

### Phase 5: Completion

After editor agent completes:

1. Summarize changes made
2. Show expected improvements (duration reduction, cost savings)
3. Recommend follow-up:
   - Run workflow to verify improvements
   - Check Blacksmith dashboard after 5-10 runs for cache metrics
   - Consider applying same optimizations to other workflows

## Notes

- Command is read-only until user approves specific fixes in Phase 4
- Researcher agent triggered only for anomalies (not routine optimizations)
- All metrics calculated from `gh` CLI JSON output using `jq`
- Blacksmith dashboard features (cache hit rates, cost data) require manual review
- If workflow name ambiguous, list all workflows and ask for clarification
- For workflows with matrix jobs, analyze each matrix combination separately

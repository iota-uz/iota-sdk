# Verified SDK releases

SDK PRs run service-free checks on standard GitHub runners. Merging accepts a
change; it does not certify a production version. A consumer request causes one
immutable candidate to run the complete suite before a public version exists.

## Agent workflow

Install the existing Go CLI from a reviewed SDK checkout:

```sh
cd .claude/tools
GOWORK=off go install .
```

Every SDK PR adds an immutable `.changes/<descriptive-name>.json` file:

```json
{"bump":"minor","summary":"Add upload cancellation."}
```

Use `patch` for compatible fixes, `minor` for compatible features, `major` for
breaking changes, and `none` with a reason for changes needing no release.
Declarations are append-only. The greatest bump in the batch wins. During v0,
breaking changes advance the minor version; automation never declares v1 API
stability. After v1, a major bump stops for an explicit Go module-path migration.

In a consumer repository, after installing the consumer workflow below:

```sh
sdk-tools sdk use 1234 --go-dir back --web-dir frontend
# Develop and test the consumer, then commit .sdk/dependency.json with its PR.
sdk-tools sdk promote --pr 4567
sdk-tools sdk status
```

The numeric argument is an iota-uz/iota-sdk PR. Omit `--web-dir` for Go-only
consumers. `--go-dir` defaults to the repository root. The first implementation
supports a standalone pnpm package with its lockfile in `--web-dir`, not an
arbitrary nested pnpm workspace. Frontend preview packages are built by the
SDK's unprivileged `frontend-packages.yml` workflow at the exact same-repository
PR SHA. The local command only downloads that successful artifact and installs
it with lifecycle scripts and `.pnpmfile.cjs` disabled.

`use` writes `.sdk/dependency.json`, checks out the SDK in ignored `.sdk/cache/`,
and creates a managed, ignored `go.work`. It downloads and installs a SHA-stamped
frontend preview when requested, restoring the consumer's package manifest and
lockfile afterwards. Fork SDK PRs are rejected because their head cannot be
bound to the trusted artifact workflow. Only the dependency declaration is
committed. An existing unmanaged Go workspace is never overwritten. To refresh the preview, run
`sdk-tools sdk preview`; this is also the setup command for a consumer preview CI
job. Run your normal consumer build/tests after it with the workspace enabled.

`promote` changes the consumer PR's declaration to `release` and wakes its
workflow. It does not merge either PR or deploy the application. The durable
declaration survives a terminated agent session. The controller waits for the
SDK PR to merge, requests a release if needed, and commits exact Go/npm versions
and lockfiles to the consumer PR when ready. The GitHub App commit triggers the
consumer's normal CI. Local fallback: `sdk-tools sdk finalize`.

An already finalized PR stays on its verified version even if later SDK releases
appear. To request another SDK change, use its PR number and promote again.

## Install the consumer workflow

Create two least-privileged GitHub Apps for consumers. Install the consumer App
only on participating consumer repositories and grant Contents (read/write),
Pull requests (read), and Commit statuses (read/write). Install the requester App
only on `iota-uz/iota-sdk` and grant Contents (read), Pull requests (read), and
Actions (write). Store the two App IDs as `SDK_CONSUMER_APP_ID` and
`SDK_REQUESTER_APP_ID`, and their private keys as separate Actions secrets in
each participating consumer.

Put this file on the consumer's **default branch** as
`.github/workflows/sdk-dependency.yml`. Replace both `REVIEWED_SDK_SHA` entries
with the same reviewed full commit hash before installing it:

```yaml
name: SDK dependency
on:
  workflow_dispatch:
    inputs:
      pr:
        description: Consumer PR that requested promotion
        type: string
  schedule:
    - cron: '*/5 * * * *'
permissions:
  contents: read
jobs:
  reconcile:
    uses: iota-uz/iota-sdk/.github/workflows/sdk-consumer.yml@REVIEWED_SDK_SHA
    with:
      sdk-ref: REVIEWED_SDK_SHA
      consumer-app-id: ${{ vars.SDK_CONSUMER_APP_ID }}
      requester-app-id: ${{ vars.SDK_REQUESTER_APP_ID }}
    secrets:
      consumer-app-private-key: ${{ secrets.SDK_CONSUMER_APP_PRIVATE_KEY }}
      requester-app-private-key: ${{ secrets.SDK_REQUESTER_APP_PRIVATE_KEY }}
```

The controller reconciles all open same-repository PRs, so dispatches can be
coalesced without losing requests. Fork PRs are intentionally not modified.
The normal path starts immediately on `promote`; subsequent reconciliation is
scheduled every five minutes, subject to GitHub scheduling delays. No private
consumer names, URLs or source code are stored in the public SDK queue.

Require `sdk/release` plus the consumer's normal quality checks before merging a
production-bound PR. The controller verifies exact Go/npm versions, rejects Go
`replace` directives, and checks the pnpm lockfile. Production checks and builds
must use `GOWORK=off`. This SDK PR provides the reusable workflow; consumers must
adopt it in their own repositories. Existing SHA finalizers must be disabled in
that same consumer migration to avoid two bots updating the same dependency.

The scheduled privileged controller runs trusted SDK tooling and never executes
consumer npm lifecycle scripts or consumer test commands with App credentials.
Consumer tests run in the consumer's ordinary CI after the dependency commit.

## Release controller

`sdk-request.yml` durably registers only a public SDK PR number and dispatches
`release.yml`. Requests live in `state.json` on `sdk-release-state`. Optimistic
file-SHA updates preserve concurrent registrations. This branch is controller
state, never application source; do not delete or manually rewrite it.

`release.yml` wakes on a request, a main push, or its half-hour recovery schedule.
Without an unsatisfied request it runs only the small controller job. Pending
requests are batched into one candidate rooted at the current main SHA. The
controller creates a commit containing matching Go/npm versions and release
notes at `.sdk/release.json`, on `sdk-candidates/v<VERSION>-<SOURCE_PREFIX>`.
Version commits are not merged back into main: the published tag and ready state
are the release history; main remains the development source. `.changes` files
are accounted for relative to the previous release's source commit.

The release workflow calls `ci-full.yml` with the exact candidate SHA, builds and
verifies the canonical tarball, and only then allows publication. Full CI retains
integration tests, migration/seed checks, four E2E shards, Lens VR, coverage,
lint, generation and docs checks. Main advancing neither cancels the current
candidate nor changes its contents. An additional consumer requiring a later
merge waits for the next candidate.

Publication creates an immutable tag through the GitHub App, publishes the
already verified tarball using npm OIDC, checks registry integrity/provenance,
checks that the Go module is retrievable, then creates the GitHub Release and
marks the candidate ready. A tag alone is never sufficient for consumers.
Before npm publication it creates a draft GitHub Release and uploads the exact
verified manifest and tarball. On success that same draft becomes the public
completion record.

## Recovery and rollout

- Failed verification remains failed at that source SHA. A new main commit can
  produce a corrected candidate. Deliberate retry after an infrastructure fix:
  `gh workflow run release.yml -R iota-uz/iota-sdk --ref main -f retry=true`.
- Once publication starts, every retry uses the original SHA and artifact run,
  skips full CI and packaging, and never overwrites a tag or npm version. The
  Actions copy is retained for 90 days. If it expires, the workflow restores and
  verifies the exact manifest and tarball from the durable draft GitHub Release.
  Never substitute a rebuilt tarball. If the draft or either asset was manually
  deleted, recover those exact bytes from a repository backup before retrying.
- If the agent pushes while the consumer bot is finalizing, the bot's
  non-fast-forward update is rejected. The next reconciliation starts from the
  new PR head. It never force-pushes or merges the PR.
- Install the dedicated publisher App credentials in the SDK only before the
  first request. Grant it SDK Contents (write), without consumer access. Keep
  npm's
  trusted publisher bound to `iota-uz/iota-sdk`, `release.yml`; publishing remains
  on a standard GitHub runner in that workflow.
- Protect `v*` tag creation with a ruleset restricting creation to the publisher
  App; prohibit tag updates/deletions. On SDK `main`, require the
  `Release contract and tooling` check from `test.yml`.
  Protect controller/candidate branches from manual edits/deletion as appropriate.
- Existing production pins are not changed by merging this PR. Adopt the consumer
  workflow and SemVer policy per consumer; preview pins remain development-only.

For an explicit pre-merge full check, use
`gh workflow run ci-full.yml -R iota-uz/iota-sdk --ref main -f sha=<FULL_SHA>`.
For Linux Lens baseline candidates, add `-f lens_vr_update=true`. Such a run never
publishes anything and is not a substitute for the release workflow's gate.

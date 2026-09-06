# Verified SDK releases

PRs run service-free checks on standard GitHub runners. A release runs full CI
once on an immutable candidate before publishing matching Go/npm SemVer versions.
Merge does not launch a release. There are no scheduled release jobs, GitHub Apps,
consumer bots, or cross-repository CI secrets.

## Agent workflow

Install from a reviewed SDK checkout using your existing Go toolchain and gh login:

```sh
cd .claude/tools
GOWORK=off go install ./cmd/sdkctl
```

Each SDK PR adds an append-only `.changes/<name>.json` declaration:

```json
{"bump":"minor","summary":"Add upload cancellation."}
```

Use patch for compatible fixes, minor for features, major for breaking changes,
and none with a reason when no release is needed. The largest bump since the
previous release wins. During v0 a breaking change advances minor; after v1 a
major change requires an explicit Go module-path migration.

In the consumer repository:

```sh
sdkctl use 1234 --go-dir back --web-dir frontend
# Develop, test, and commit the consumer changes and .sdk/dependency.json.
# Merge SDK PR 1234 through its normal review process.
sdkctl promote
# Review local dependency changes, then commit and push normally.
```

Omit web-dir for Go-only consumers; go-dir defaults to `.`. Web consumers currently
require a standalone pnpm package and lockfile, not a nested pnpm workspace.
`sdk-tools sdk` remains an equivalent spelling of `sdkctl`.

`use` creates an ignored SDK checkout and managed go.work. Frontend preview is
an artifact from successful frontend-packages.yml at the exact same-repository
SDK PR head, installed without lifecycle scripts or pnpmfile execution. Production
lockfiles stay unchanged. Refresh with `sdkctl preview`. Fork SDK PRs and unmanaged
existing workspaces are rejected.

`promote` requires the SDK PR to be merged. It reuses a ready release containing
that merge, or joins an active release workflow, or dispatches release.yml using
your local gh authorization. Creating a release requires permission to dispatch
SDK Actions. An interrupted session is resumed by running the same command.
The remote release continues after the terminal closes.

After successful publication, promote updates local Go/npm locks and the release
identity in .sdk/dependency.json, verifies matching versions, runs GOWORK=off go vet,
and removes the managed preview workspace. Errors restore the dependency files.
Commit dependency files before promotion; dirty locks are never overwritten.
Run your normal consumer tests, review, commit and push. The normal push triggers
consumer CI. The command neither commits nor pushes nor merges a PR.
An already finalized dependency stays on its version even after newer releases.

## Consumer CI

No privileged consumer workflow is installed. Production checks must use
GOWORK=off and reject a preview declaration. A normal unprivileged CI job can run
`sdkctl verify` from reviewed tooling to check the committed release record, tag,
Go version (without replace), and exact npm version and pnpm lockfile. Make this
job required for production-bound PRs. Preview CI can use `sdkctl preview` and run
consumer tests with the workspace enabled; it does not make a preview releasable.
Disable existing automated SHA finalizers when adopting this model.

## Release workflow and recovery

Only workflow_dispatch starts release.yml. The input is the required public SDK
PR number; no consumer details are sent upstream. Requests and candidate state
live on sdk-release-state, with optimistic concurrency. Candidates contain matching
Go/npm versions and release metadata, on sdk-candidates/v<VERSION>-<SOURCE_PREFIX>.
Version commits are not merged into main. Full integration/E2E, migration, Lens VR,
coverage, lint and generation checks run on the candidate SHA.

Publication uses the SDK repository GITHUB_TOKEN for the immutable Git tag and
npm trusted publishing with OIDC. It verifies npm integrity/provenance and Go
module availability before writing the ready record into release state. Historical
ready records remain available for pinned consumers. GitHub Releases are not a
required part of publication. A tag by itself is insufficient proof of readiness.

If verification fails, fix the SDK and merge the fix. For an infrastructure failure
at unchanged source, explicitly run `sdkctl promote --retry`. Publication retries
always use the same SHA and original Actions artifact and never overwrite a tag
or npm version. The artifact is retained for 90 days. If it is deleted or expires
during an incomplete publication, recovery stops: restore the exact verified
artifact before retrying; do not substitute a new build or move the version tag.

For a manual release or recovery:

```sh
gh workflow run release.yml -R iota-uz/iota-sdk --ref main -f sdk_pr=1234 -f retry=true
```

For full pre-merge verification without publication:

```sh
gh workflow run ci-full.yml -R iota-uz/iota-sdk --ref main -f sha=<FULL_SHA>
```

## Rollout

Configure npm trusted publishing for iota-uz/iota-sdk / release.yml on a standard
GitHub runner. Allow the release workflow contents write. Protect v* against
updates/deletions and protect state/candidate branches from manual changes while
allowing the workflow to create and update its state. Require `Release contract
and tooling` from test.yml on main. No App credentials are needed.
Existing consumer pins remain unchanged until their own explicit promotion.

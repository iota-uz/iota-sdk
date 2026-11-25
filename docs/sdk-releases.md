# Verified SDK releases

PRs run service-free checks on standard GitHub runners. A release runs full CI
once on an immutable candidate before publishing matching Go/npm SemVer versions.

## Plot-managed consumer work

When a consumer task needs an SDK change, create an issue in `iota-uz/iota-sdk`
and enqueue it in Plot with `auto_merge: true`. Set the consumer card to wait with
`until issue_done` for the SDK issue. The Plot GitHub App merges the SDK card's
PR automatically once CI, QA, and code review pass.

When the SDK issue is done, set the consumer card to wait with
`until release_contains {repository: "iota-uz/iota-sdk", pr}`, where `pr` is the
merged SDK PR number. If no `vX.Y.Z` tag contains that PR, the Plot GitHub App
dispatches `release.yml` on demand with `sdk_pr` set to the PR number. Plot
dispatches at most once per PR. If the release fails, the waiting card escalates
when its deadline expires. A dispatch with no outstanding release work is a
no-op, for example when a ready release already contains the requested PR.

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

Use `patch` for compatible fixes, `minor` for features, and `major` for breaking
changes. Every SDK PR needs a releasable declaration. If the pending declarations
are all `none`, release preparation fails with `no releasable changes`. The
largest bump since the previous release wins. During v0 a breaking change
advances minor; after v1 a major change requires an explicit Go module-path
migration.

In the consumer repository:

```sh
sdkctl use 1234 --go-dir back --web-dir frontend
# Develop, test, and commit the consumer changes and .sdk/dependency.json.
# Once the SDK PR has merged:
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
that merge or joins an active release workflow, including one Plot dispatched.
It can also dispatch `release.yml` using local `gh` authorization with permission
to dispatch SDK Actions. Run the same command to resume an interrupted session.
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

Registry verification bypasses cached metadata and allows 36 checks, separated by
five seconds, for the package and its provenance to appear. Each registry request
has a ten-second deadline. `npm_visibility_timeout` means package integrity is
still absent; `npm_provenance_timeout` means the verified tarball is visible but
its provenance is not. Neither marks the release ready. A visible conflicting
integrity or provenance predicate fails immediately with `npm_integrity_mismatch`
or `npm_provenance_mismatch`; retries must never replace those published bytes.
Transient network, request/body timeouts and 5xx failures share the bounded
post-publication polling budget. Exhausting it reports `npm_registry_timeout`
with the transport cause. The initial lookup, non-404 4xx responses and malformed
successful JSON remain fatal; an unavailable registry never authorizes publication.

If verification fails, fix the SDK and merge the fix. For an infrastructure failure
at unchanged source, explicitly run `sdkctl promote --retry`. Publication retries
always use the same SHA and original Actions artifact and never overwrite a tag
or npm version. The artifact is retained for 90 days. If it is deleted or expires
during an incomplete publication, recovery stops: restore the exact verified
artifact before retrying; do not substitute a new build or move the version tag.

A partially published version whose Go module can never be served (the proxy
refuses to build its zip, or its content disagrees with the checksum database)
cannot be resumed: the tag is immutable, so every retry fails the same way. An
administrator retires it from `.claude/tools`, with a token that can write
sdk-release-state:

```sh
go run . sdk controller abandon <CANDIDATE_SHA>
```

The command first proves the failure: `go mod download -json` through
proxy.golang.org, with an empty module cache, must answer with a permanent error
(`create zip`, `malformed file path`, `checksum mismatch`, `SECURITY ERROR` or
`invalid version`). A module that downloads, or a network or proxy failure, is
refused. Only a `publishing` candidate is abandoned; a `testing` or `failed` one
is superseded by the next release. The candidate moves to `releases` with the
`abandoned` phase (the command prints the go command's error as the proof; record
it on the release issue), the candidate slot is cleared and `ready` stays on the last ready release. The tag and the npm version
stay as they are (deprecate the npm version separately). Consumers never resolve
an abandoned version, and the next release takes a version above every abandoned
one: the patch after it when its own bump would not clear it (0.55.0 abandoned,
the next release is 0.55.1). Then start the release as usual.

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
and tooling` from test.yml on main. The release workflow uses GitHub Actions
credentials for publication, and Plot uses its GitHub App for merge and dispatch.
Existing consumer pins remain unchanged until their own explicit promotion.

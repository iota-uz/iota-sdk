# Client host release train

Go contracts and frontend packages are one SDK release unit. A preview must use
the same SDK commit in both dependency planes:

```sh
node scripts/package-frontends.mjs /tmp/iota-sdk-frontends
go mod edit -replace github.com/iota-uz/iota-sdk=/absolute/path/to/iota-sdk
pnpm add /tmp/iota-sdk-frontends/iota-uz-client-host-0.0.0-sha-<sha>.tgz \
  /tmp/iota-sdk-frontends/iota-uz-lens-web-0.0.0-sha-<sha>.tgz
```

`frontend-artifacts.json` records the full SDK commit, protocol and SHA-256 of
each exact tarball. Consumer automation must stage `go.mod`, `go.sum`,
`package.json` and the package lock together and only replace the working files
after both Go resolution and `pnpm install --lockfile-only` succeed. A partial
resolution is discarded, never committed.

On a pull request, the workflow explicitly checks out and stamps
`github.event.pull_request.head.sha`, not GitHub's synthetic merge SHA. The
artifact is therefore named `frontend-<public-pr-head-sha>` and its manifest
must match the commit resolved by `just sdk preview <pr>` exactly. On a `main`
push, the same invariant uses `github.sha`. `SDK_SOURCE_SHA` is a guard as well
as an input: packaging aborts if it differs from the checked-out commit.

`@iota-uz/lens-web` declares the same-SHA `@iota-uz/client-host` as an exact
peer. It never asks a package registry for that private sibling: the consumer
installs both tarballs in one `pnpm add` transaction. CI proves this in a clean
temporary project with the `@iota-uz` registry pointed at an unreachable
address, React 19 installed as the host runtime, strict peer checks enabled,
and the installed package manifests compared with `frontend-artifacts.json`.

Direct React routes consume the SHA-stamped `@iota-uz/lens-web` tarball. This is
the canonical delivery model. The custom element remains a compatibility
adapter for hosts that have not migrated yet, but its generated
`pkg/lens/render/react/dist` files are ignored and never published by a bot or
committed after merge.

A clean Go clone therefore needs no Node installation. Importing the SDK and
compiling a direct-package host do not read the compatibility manifest. A
legacy host must run `just lens build` before compiling the Go binary, or set
`LENS_ASSETS_DIR` to an already-built dist at runtime; invoking the legacy
renderer/controller without either fails with an actionable error instead of
serving an empty dashboard.

Consumer exact-match enforcement depends on one atomic, verified installation
of both frontend tarballs and the Go module from the same immutable commit. It
does not depend on a release bot or an arbitrary number of unattended cycles.

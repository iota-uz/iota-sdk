# Canonical frontend distribution

The Go module and `@iota-uz/sdk` are one release unit. The public JavaScript
package deliberately exposes only bounded APIs built from separate private
workspace modules:

- `@iota-uz/sdk/identity`
- `@iota-uz/sdk/client-host`
- `@iota-uz/sdk/lens`
- `@iota-uz/sdk/lens/styles.css`

Version 0.5 is a clean break. The former root, BiChat, applet, Tailwind, asset,
`@iota-uz/client-host`, and `@iota-uz/lens-web` APIs are not compatibility
surfaces of this distribution.

## Identity contract

Every release uses one SemVer for its Git tag, Go identity, and npm package.
The frontend build also embeds the full source commit and protocol version.
The Go host sends all three values to the browser; the client validates them
before rendering. A release, commit, or incompatible protocol mismatch fails
closed with an explicit diagnostic.

## Pull-request preview

`scripts/package-frontends.mjs` builds one SHA-stamped package into an ephemeral
Actions artifact. The artifact is neither published nor committed and is not a
production dependency. Its manifest records the source commit, release version,
protocol version, tarball name, and SHA-256.

The verification script installs the tarball in an empty consumer while the
`@iota-uz` registry points to an unreachable address. It checks every export,
peer-only React resolution, and proves that importing client-host does not pull
Lens or ECharts into the initial bundle.

## Production release

The `v<version>` tag must point at the release commit and match both
`web/sdk/package.json` and `pkg/sdkidentity.ReleaseVersion`. One workflow tests
the Go and JavaScript graphs, builds the canonical tarball, publishes it through
npm trusted publishing with provenance, verifies registry metadata, and creates
the GitHub release.

Consumers update `go.mod`, `go.sum`, `package.json`, and the JavaScript lockfile
in one transaction. Production must never use preview tarballs, release-asset
URLs, a Go module-cache build, or revisions that differ between Go and npm.

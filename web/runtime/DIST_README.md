# Extension packages: `iota-ext` and the runtime API

`@iota-uz/sdk` ships the author tooling for iota-sdk extension packages.

```sh
npx iota-ext init loyalty --namespace acme.loyalty   # scaffold + the offline developer kit (core, crm)
cd loyalty && npm install                            # the kit lives in .iota/kit, which npm does not touch
npx iota-ext check      # typecheck, graph rules, manifest
npx iota-ext test       # run *.test.ts on the in-memory test host
npx iota-ext build      # artifacts + manifest.json in .iota/build
npx iota-ext kit pull --server https://erp.example.com --token-env IOTA_TOKEN   # the live developer kit
npx iota-ext kit status                              # verify the kit in .iota/kit (modules, files, hashes)
```

Import paths: `@iota-uz/sdk/runtime` (contracts), `@iota-uz/sdk/runtime/server` (handlers), `@iota-uz/sdk/runtime/client`
(pages, Solid), `@iota-uz/sdk/solid-ui` (the host's Solid UI kit, client graph only; styling with it, Tailwind utilities and
`*.module.css` is in the guide, "Styling"), `@iota-uz/sdk/runtime/testing` (test host, fixtures, fakes, `assert`; not `@iota-uz/sdk/testing`, the
Playwright package). `init` pins `@iota-uz/sdk` to the exact version of the CLI that ran it: the runtime ships from that
release on. Exit codes: 0 ok, 1 package errors or failing tests, 2 usage, 3 environment.

The full author guide is [`docs/authoring.md`](docs/authoring.md) (next to this file), with a complete example package in
[`docs/example`](docs/example). Contents of this directory: `cli/iota-ext.mjs` (the command), `build/` (the build worker
it drives), `host/` (the invocation runtime), `catalog/sdk-natives.json` (the SDK's native catalog the offline kit is
generated from) and the TypeScript declarations of the public subpaths.

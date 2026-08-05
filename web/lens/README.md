# Lens React runtime

`@iota-uz/sdk/lens` is the canonical Lens delivery surface. Standard React
hosts install one SHA-stamped `@iota-uz/sdk` artifact and import both the Lens
and client-host APIs from its bounded subpaths:

```ts
import { ClientHostProvider } from '@iota-uz/sdk/client-host'
import { LensDashboard } from '@iota-uz/sdk/lens'
import '@iota-uz/sdk/lens/styles.css'
```

`pkg/lens/render/react` remains a legacy custom-element adapter. `just lens
build` creates its self-contained Vite bundle under the ignored
`pkg/lens/render/react/dist`; generated chunks are not committed. Its Tailwind
pipeline is isolated from host applications: utilities are prefixed with
`lens-` and preflight is disabled.

```sh
pnpm install
just lens typegen
just lens check
just lens build
just lens dev --fixture
```

A clean clone can compile direct-package Go consumers without Node. A legacy Go
host must run `just lens build` before it builds the binary, or provide a built
directory with `LENS_ASSETS_DIR`; otherwise invoking the adapter fails with an
actionable compatibility-bundle error.

Without `--fixture`, the development page requests `/lens/document` through the
Vite proxy. Set `LENS_BACKEND_URL` to change the Go server from
`http://localhost:3200`.

`just lens fixture <url>` records a document as `fixtures/small.json`, which is
consumed directly by fixture mode, stories, and tests. Pass `--output <name.json>`
to keep another recording. Set `LENS_SESSION_COOKIE='sid=…'` or pass
`--cookie 'sid=…'` to forward a session cookie.

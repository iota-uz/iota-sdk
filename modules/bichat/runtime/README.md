# BiChat Bun runtime (Slice 1)

This runtime is a thin Bun pilot process for applet-engine slice 1.

It is enabled only when:

- `IOTA_APPLET_ENGINE_BICHAT=bun`

The Go engine starts this process with:

- `IOTA_APPLET_ID=bichat`
- `IOTA_ENGINE_SOCKET=<engine uds>`
- `IOTA_APPLET_SOCKET=<applet uds>`

Endpoints:

- `GET /__health` health probe for process manager
- `GET /__probe` validation path that exercises server-only `kv.*` and `db.*` methods

This path is intentionally non-user-facing in slice 1.

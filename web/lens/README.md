# Lens React runtime

The Lens custom element is built as a self-contained Vite bundle and embedded by
`pkg/lens/render/react`. Its Tailwind pipeline is isolated from host applications:
utilities are prefixed with `lens-` and preflight is disabled.

```sh
pnpm install
just lens check
just lens build
just lens dev --fixture
```

Without `--fixture`, the development page requests `/lens/document` through the
Vite proxy. Set `LENS_BACKEND_URL` to change the Go server from
`http://localhost:3200`.

`just lens fixture <url>` records a document as `fixtures/live.json`. Set
`LENS_SESSION_COOKIE='sid=…'` or pass `--cookie 'sid=…'` to forward a session
cookie.

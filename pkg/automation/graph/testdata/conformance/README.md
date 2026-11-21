# Conformance fixtures of the automation graph format

Data, not code: an interpreter in another language conforms when it gives the same answers.
`conformance_test.go` runs every file here.

- `plot/` — Plot's fixtures, copied once (see `plot/README.md`). `plot/catalog.json` is the catalog every
  fixture uses unless it brings its own.
- `sdk/` — the SDK's fixtures, format 2.

## Fixture kinds

- `trace`: `graph`, `context` and `inputs` (events `{kind, ...payload}`, or `{"timeout": true}` for the
  timeout of the wait the run sleeps on) → the expected `trace` of dry run. `context` holds `nodes`
  (`{output, outcome}` per node), `now` (RFC 3339), `tenant`, and the subject's variables.
- `validate`: `graph` → the expected `errors`, compared as a multiset of `path` (and of `path` + `code`
  when the expected errors give codes). Messages are not compared.
- `expressions`: `cases`, each `subject`, `event` (a kind or null), `source`, `type` (`bool`, `string`,
  `map` or null) and `context` → `value`, or `error`: `type` (refused by the check; `issue` pins
  `syntax`, `type` or `limit`), `missing_context` with its `path`, or `failed` with its `code`.
- `templates` (SDK): as `expressions`, `source` being a `{{ }}` template rendered to a string.
- `catalog` (SDK): a catalog (`catalog` / `catalog_patch`) → whether loading it fails (`error`: a text
  the error contains).

In an expected trace the string `"*"` matches any value: failure messages are not normative.

## Plot's fixtures in the SDK

Plot's graphs carry the fields of a version (`id`, `version`, `enabled`) and `attach`. The runner drops
them, and `attach` of kind `event`: version fields belong to the graph's owner, and every SDK graph is
what Plot calls an event graph. `subject: {repository}` has no meaning in the SDK and is ignored; `plot`
(whether Plot's own dry run also runs a fixture) is ignored.

## Format 2 (SDK)

- `format: 2`.
- `requires` / `unless`: capabilities the fixture needs or must not have, as `behavior:<name>`
  (`behavior:parallel` arrives with #1328). The runner skips a fixture whose `requires` are not met (or
  whose `unless` is) and prints the number skipped; nothing is skipped otherwise.
- `catalog`: a catalog of its own instead of `plot/catalog.json`.
- `catalog_patch`: a JSON merge patch (RFC 7386) over the catalog.
- `limits`: expression limits (`max_nodes`, `max_depth`, `max_literal`, `max_args`, `cost_limit`).
- `options`: `call_error` (every call request is refused with this message), `match_hooks` (hooks that
  take every event).
- `changed`: why a copied fixture was adapted.

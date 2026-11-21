# Conformance fixtures of the automation graph format

Data, not code: an interpreter in another language conforms when it gives the same answers.
`conformance_test.go` runs every file here.

- `plot/` — Plot's fixtures, copied once (see `plot/README.md`). `plot/catalog.json` is the catalog every
  fixture uses unless it brings its own.
- `sdk/` — the SDK's fixtures, format 2.

## Fixture kinds

- `trace`: `graph`, `context` and `inputs` (events `{kind, ...payload}`, or `{"timeout": true}` for the
  earliest pending timeout) → the expected `trace` of dry run. `context` holds `nodes`
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
  (`behavior:parallel`, `behavior:join`, `behavior:foreach`, `behavior:collect`). The runner skips a
  fixture whose `requires` are not met (or whose `unless` is) and prints the number skipped.
- `catalog`: a catalog of its own instead of `plot/catalog.json`.
- `catalog_patch`: a JSON merge patch (RFC 7386) over the catalog.
- `limits`: expression limits (`max_nodes`, `max_depth`, `max_literal`, `max_args`, `cost_limit`).
- `options`: `call_error` (every call request is refused with this message), `match_hooks` (hooks that
  take every event), `without_behaviors` (validate without these behaviors; the catalog still loads with
  all of them).
- `changed`: why a copied fixture was adapted.

## Tokens (flow behaviors)

`parallel`, `join`, `foreach` and `collect` run several tokens in one run. A token is named by its path:
`<parallel>.<branch>`, `<foreach>[<index>]`, nested ones joined by `/` (`split.legal/each[3]`); the main
path is the empty token. In a trace:

- A step of a token other than the main path has `token`. A parallel and a foreach that starts its items
  take no step of their own: their tokens' steps follow.
- A parallel's branches run in their order, a foreach's items in the order of `items` whatever its
  `concurrency`, each depth first until it waits, arrives or ends.
- `arrived` (`context_patch: {"value": ...}`) is a token reaching a join (the output of the branch's last
  node, `null` when it has none or it is `{}`) or a collect (its `value`, `null` without one).
- `cancelled` is a token a join cancelled (any, n_of, timeout), on the node it was at, in branch order.
- The join's step (`joined` or `timeout`, output `branches`) and the foreach's (`done`, `partial_failed`
  with output `results`, or `failed` with `code` and `error`) belong to the token that forked.
- An `end`, `ignore`, a failure without a failed outcome or a join that can no longer join (without
  timeout) ends only its token outside the main path (an item is then `ok: false`); on the main path it
  ends the run. `missing_context`, `MaxTokens` (code `LIMIT`) and a token taking 500 steps without
  waiting stop the whole trace.
- `foreach.<id>.item` and `.index` are read inside that foreach's body (its collect and the constructs
  nested in it); elsewhere they are an `expression_type` error.
- An event wakes every waiting token it matches, in the order the tokens were created. `{"timeout": true}`
  fires the earliest pending timer on the dry run's clock — a wait's from when it started waiting, a
  join's from its parallel's fork — the first one created on a tie.

Error codes of the structure: `parallel_structure`, `join_foreign_edge`, `foreach_escape`,
`cycle_through_foreach`, and `limit` for the branches, concurrency and max_items limits.

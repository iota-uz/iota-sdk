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
- `catalog_patch`: a JSON merge patch (RFC 7386) over the catalog. A fixture adds only the flow node types
  its `requires` name, so a runner with just those behaviors loads it.
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
- A parallel's branches run in their order, each depth first until it waits, arrives or ends. A
  foreach's items run in the order of `items` one after another — the next starts once the one before
  ended — whatever its `concurrency`, so the trace and its `MaxTokens` count do not depend on it.
- `arrived` (`context_patch: {"value": ...}`) is a token reaching a join (the output of the branch's last
  node — `{}` stays `{}` — or `null` when it has none) or a collect (its `value`, `null` without one).
- `cancelled` is a token a join cancelled (any, n_of, timeout), on the node it was at, in branch order.
- The join's step (`joined` or `timeout`, output `branches`) and the foreach's (`done`, `partial_failed`
  with output `results`, or `failed` with `code` and `error`) belong to the token that forked.
- Inside a branch or a foreach's body a node's failed outcome may be left without an edge: the token
  ends there and does not arrive (the item is not ok).
- A join's and a foreach's output keep `MaxValueBytes`: a larger one fails with `VALUE_TOO_LARGE`; a
  foreach takes its `failed` outcome with output `{"error", "code": "VALUE_TOO_LARGE"}`.
- A join's `timeout` outcome may be left without an edge: taking it then fails the run (or the token
  that forked) with `JOIN_INCOMPLETE`.
- An `end`, `ignore`, a failure without a failed outcome (a wait's filter included) or a join that can no longer join (without
  timeout) ends only its token outside the main path (an item is then `ok: false`); on the main path it
  ends the run. `missing_context`, `MaxTokens` (code `LIMIT`) and a token taking 500 steps without
  waiting stop the whole trace.
- `foreach.<id>.item` and `.index` are read inside that foreach's body (its collect and the constructs
  nested in it); elsewhere they are an `expression_type` error.
- An event is offered, in the order the tokens were created, to the tokens that were waiting when it
  arrived and are still waiting at their turn: a matching wait a join cancelled meanwhile (a branch
  before it arrived first) is skipped, and a wait started while the event is handled does not receive
  it. A trace that stops (`missing_context`, `LIMIT`, a failed run) ends with that step. `{"timeout": true}`
  fires the earliest pending timer on the dry run's clock — a wait's from when it started waiting, a
  join's from its parallel's fork — the first one created on a tie.

Error codes of the structure: `parallel_structure`, `join_foreign_edge`, `foreach_escape`,
`cycle_through_foreach`, and `limit` for the branches, concurrency and max_items limits.

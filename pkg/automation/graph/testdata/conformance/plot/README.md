# Copied conformance fixtures

Source: Plot automation-graph conformance @ 01fb439e0b0b

Copied once: `catalog.json` and the 21 files of `fixtures/`. They are not kept in sync with the source.

Adapted files carry a top-level `"changed"` with the reason; every other file is byte-for-byte the source:

| File | Why |
| --- | --- |
| `catalog.json` | No column/card graphs (`attach_kinds`, `column_event`, `column_subject` removed); a match field reads the payload (`{"payload", "fold"}` instead of `from: "repository"` / `from: {payload}`); `triggers: false` is `no_trigger: true`; `issue.labeled` has an optional `repository`. |
| `fixtures/validate-column-graph.json` | Column graphs are a Plot product concept. The graph is a plain SDK graph whose trigger still carries `column`, which the strict decoder refuses. |
| `fixtures/trace-trigger-match.json` | The SDK subject is `{entity, key}`, not a repository: the inputs carry the repository the subject had. The expected trace is unchanged. |

No fixture was removed.

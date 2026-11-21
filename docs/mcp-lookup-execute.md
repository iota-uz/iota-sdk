# MCP surface: `lookup` and `execute`

An application built on the SDK exposes its AI tools through two entry points
instead of one MCP tool per operation:

```ts
lookup<M extends ReadMethod>({method: M, input: InputOf<M>}): Result<OutputOf<M>>
execute({code: string, timeout_ms?: number, max_output_bytes?: number}): Result<ExecuteResult>
```

Both come from the shared registry in `pkg/mcp`. Operations (`describe`,
reports, search, `prepare_changes`, …) stay ordinary registry tools with one
schema, one policy and one handler; the entry points only route to them through
`mcp.Engine`, which re-checks identity, scopes, RBAC and the input schema on
every call, nested ones included.

## Wiring

```go
registry := mcp.NewRegistry()
engine, _ := mcp.NewEngine(registry, authorizer)
_ = registry.Register(appOperations...)

describe, _ := lookup.Describe(engine, lookup.DescribeConfig{
    Policy:    readPolicy,
    Selectors: map[string]*jsonschema.Schema{"report": {Type: "string"}}, // optional
    Select:    answerSelectors,                                          // optional
    Catalog:   domainCatalog,                                            // optional, under "domain"
})
entry, _ := lookup.New(engine, lookup.Config{Policy: connectPolicy, About: "What the app is."})
runner, _ := execute.NewRunner(execute.Config{Engine: engine, Manager: pythonManager, /* limits */})
_ = registry.Register(describe, entry, runner.Tool())

server, _ := mcpgateway.NewServer(engine, impl, opts) // tools/list: lookup, execute
```

`Spec.EntryPoint` marks a published tool. Once a registry holds an entry point,
`Registry.Published` — used by `mcpgateway.NewServer` and `chattools.Tools` —
returns only entry points; a `tools/call` of a former top-level tool is an
unknown-tool protocol error. Entry points are host-only, so Python code cannot
call `lookup` or `execute`.

## `lookup`

- `input` is required; `{}` for an operation without arguments.
- The method is classified by the server registry before anything runs:
  unknown, host-only or not permitted to the caller → `not_found`; an available
  operation without `Annotations.ReadOnly` → `invalid_input` with issue
  `/method` `side_effects` and a pointer to `execute`. The handler never runs.
- Otherwise the operation runs through `Engine.Call` and its `Result` is
  returned unchanged, so `lookup` and `await tools.<name>(...)` inside `execute`
  return identical JSON for the same caller and arguments.

## `describe`

- `{}` → `{operations: [{name, title, summary, side_effects, via}], usage, domain?}`:
  the operations the caller may call now (filtered by the same authorizer),
  with `via` listing `lookup` for reads and `execute` when the caller may run
  it. Writes are listed only when `execute` is available.
- `{tool: "<name>"}` → description, annotations, `input_schema` and
  `output_schema` (the `data` of the `Result`) from the registry.
- Application selectors (`report`, `entity`, …) are answered by `Select`.

## Python bindings

Operation arguments are keyword arguments. A parameter named after a Python
keyword is passed with `**`: `await tools.report(report="pnl", **{"from": "2026-01-01"})`.

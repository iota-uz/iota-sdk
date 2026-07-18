# Lens runtime

Create one `runtime.Runtime` per process and share it through every Lens path:
render, panel fragments, drill-down and export. `Request.DataScope` is required
host identity (tenant + permission/data scope); changing it guarantees a cache
miss. `Namespace` versions host-owned semantics.

```go
rt := runtime.New(runtime.Options{
    Store: runtime.NewMemorySnapshotStore(runtime.MemoryStoreOptions{
        TTL: 5 * time.Minute, MaxEntries: 128,
    }),
    CacheVersion: "analytics-v2",
})
engine := engine.New(rt)
```

Snapshots contain clone-safe dataset frames and safe provenance. Render and
export reuse the same snapshot identity. `MemoizeJSON` is the canonical bridge
for typed report assembly that must occur before a DashboardSpec exists; it
uses the same bounded store and singleflight domain.

The former request-scoped `Cache` / `NewMemoryCache` API was intentionally
removed. There is no compatibility path.

## Metric exploration fragments

Metric explorers stay out of `DashboardScope`. The prepared root dashboard
contains only the explorer manifest; it does not add embedded or lazy node
panels to the ordinary execution plan.

Transport adapters pass a prepared dashboard plus typed explorer state to
`ExplorationFragmentHandler.Handle`. The host-owned `ExplorationLoader`
resolves one node to a small `DashboardSpec` and panel ID. Lens then executes
that definition with `PanelScope(panelID)`, returning the resulting
`PanelResult` on `ExplorationFragmentResponse.Panel`.

This keeps these guarantees:

- opening the dashboard never materializes exploration-only datasets;
- a lazy node executes only its selected panel and dependencies;
- tenant/authz identity, variables, snapshots and cache semantics come from
  the normal `runtime.Request`;
- an edge-bearing lazy node must return a panel with `Fields.ID`, so clicks use
  the same stable point keys declared by the explorer edges;
- embedded nodes may reuse an already-materialized root dataset, while nodes
  needing independent data should use `LoadSpec` and the fragment contract.

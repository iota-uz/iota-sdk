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

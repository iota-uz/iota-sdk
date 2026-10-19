# Owned report caching

`serve.ReportCache[T]` combines parameter-keyed report reuse, snapshot-to-handler
registration, singleflight and derived document caching. The cache stores private
canonical wire bytes. Every returned document is independently decoded, including
concurrent cold misses and derived cache hits; callers can decorate it without
changing later requests.

`ReportKey` requires namespace, nonempty data scope and producer version. Include
normalized filter inputs and permission-dependent presentation in Params/Variant;
locale and timezone must match the runtime request. Bump Version when a producer's
specification or decoration semantics change. Keys are limited to64KiB.

Use a shared `resource.Budget` for report caches and document/runtime snapshot
stores. Inject one `serve.ReportBuildLimiter` into all related caches to bound
process-wide report builds and pending work. Per-cache options also limit entries,
individual structural/wire payload, derived children and coalesced waiters.
Nested synchronous builds inherit the admitted slot; their callback context must
not escape into unrelated asynchronous work.

`ReportDefinition` supplies serve Config, runtime Request, document BuildOptions,
an optional one-time Prepare callback and typed host State. Specifications are
structurally checked, deeply owned and accounted alongside wire bytes and keys.
Renderer icon callbacks cannot be retained by this cache; empty icons are allowed.
Handlers.BuildDocument provides typed construction without an internal HTTP/JSON
round trip. Ordinary HTTP document endpoints use the same build path.

Both CachedReport and ReportRegistration are leases: defer Release after the
complete operation. LookupSnapshotRegistration avoids document decoding on query,
panel and export routes. Spec() returns an independently owned specification for
host export conversion. An owned document may outlive its lease. Eviction removes
lookup indexes immediately; backing snapshots, registration state and reservations
are released after the last lease. OnEvict runs outside cache locks, including
rollback after an accepted definition fails. Close cancels builds and evicts
entries; outstanding leases still require Release. No finalizer substitutes for
correct ownership.

State is intentionally domain-owned. The SDK cannot inspect arbitrary closures,
executors or host metadata. Explicitly reserve retained evidence or other host
payload against the same Budget and release it in OnEvict. Avoid retaining a second
copy of the specification in State; use Spec().

The budget measures retained wire payload and conservative structural payload;
it is not a Go heap or process-memory guarantee. Executors and callbacks may
allocate before returning. Response copies, encoding temporaries, Go object
headers, datasource internals and opaque callback captures are outside retained
payload accounting. Context-aware query limits and the process memory limit
remain necessary. Structural preflight precedes encoding to reject cycles,
unsupported encoders and oversized payloads before a full duplicate allocation.
Budget errors map to413 for oversized payload and503 for admission pressure.

Minimal registration (the executor, tenant scope and normalized parameters are
provided by the host):

```go
budget := resource.NewBudget(64 << 20)
work := serve.NewReportBuildLimiter(4, 16)
snapshots := document.NewMemoryStoreWithOptions(document.MemoryStoreOptions{
    Budget: budget, MaxEntryBytes: 16 << 20, MaxEntries: 128,
})
cache, err := serve.NewReportCache[string](serve.ReportCacheOptions[string]{
    Budget: budget, BuildLimiter: work, MaxPendingBuilds: 16,
})
if err != nil { return err }
defer cache.Close()
view, err := cache.Materialize(ctx, serve.ReportKey{
    Namespace: "sales", DataScope: request.DataScope,
    Version: "producer-v2", Locale: request.Locale, Timezone: request.Timezone,
    Params: normalizedParameters,
}, func(ctx context.Context) (serve.ReportDefinition[string], error) {
    return serve.ReportDefinition[string]{
        Config: serve.Config{Spec: spec, Engine: executor, Snapshots: snapshots},
        Request: request,
    }, nil
})
if err != nil { return err }
defer view.Release()
// view.Document is request-owned; host decoration cannot mutate the cache.
```

Derived ownership uses the following lifetime graph:

```text
parent cached registration -> derived wire document -> child ownership receipt
                                               receipt -> child registration
external child request lease -> child registration + parent request lease
```

Nested materialization automatically includes the parent's snapshot identity and
inherits its execution registry. Child registrations remain addressable after
child LRU eviction while a parent receipt owns them. Each coalesced caller adopts
its own receipt before releasing the shared computation pin. Failed builders
release unused receipts. Parent eviction invalidates indexes immediately but
existing child operations retain the backing data until their leases end.
Wire-only inline documents follow the parent lifetime, including custom export
URLs; SDK query/panel documents require backing snapshots. Cache resource
rejections are counted by Stats().Rejections.

Leases defer this report cache's own deletion and state cleanup. A backing store
still applies its independent TTL, LRU and shared-budget admission rules; it may
expire a snapshot during an operation. Handle snapshot-gone responses by retrying
materialization. A lease does not pin arbitrary external stores indefinitely.

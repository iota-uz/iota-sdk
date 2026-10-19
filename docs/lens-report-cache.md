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

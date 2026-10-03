# Owned test databases

`WithMigrationAdvisoryLockContext(ctx, config, fn)` takes the existing migration
session lock on the cluster's maintenance `postgres` database. Use it around
migration replay when concurrent isolated databases share cluster-global roles.
Acquisition is required: connection failures and cancellation never run `fn`
unlocked. The acquisition wait is bounded by the caller's context and five
minutes; `fn` must honor that context itself. The lock remains held until `fn`
returns, including when cancellation occurs during migration.

Legacy `WithMigrationAdvisoryLock(config, fn)` retains its best-effort behavior.
Both functions use the same lock key, so strict callers serialize with legacy
callers that successfully acquire their lock.

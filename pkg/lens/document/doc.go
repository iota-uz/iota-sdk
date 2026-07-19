// Package document defines the versioned wire contract consumed by Lens React
// runtimes.
//
// A snapshot freezes dashboard parameters and filter state together with the
// aggregate frames materialized by one dashboard execution. Aggregate levels
// deeper than the document's inline depth are resolved with those frozen
// parameters and appended to the snapshot, so an already materialized level is
// not executed again.
//
// Evidence pages are intentionally different: paginated policy, transaction,
// and similar leaf rows are queried live using the snapshot's frozen
// parameters. A snapshot therefore guarantees a consistent query context for
// evidence, not an immutable copy of row-level data.
//
// Snapshot stores return ErrSnapshotGone for expired and unknown IDs. A client
// receiving that error must fetch a fresh dashboard document.
package document

const ContractVersion = "1.0.0"

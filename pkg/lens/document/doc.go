// Package document defines the versioned wire contract consumed by Lens React
// runtimes.
//
// A snapshot freezes dashboard parameters and filter state together with the
// aggregate frames materialized by one dashboard execution. Aggregate levels
// deeper than the document's inline depth are resolved with those frozen
// parameters and appended to the snapshot, so an already materialized level is
// not executed again.
//
// Lens frames must carry major-unit money values; a cube that SUMs a minor-unit
// (e.g. tiyin) column must convert before it reaches a frame. MinorUnits remains
// part of the wire contract for serve-layer and hand-built documents.
//
// Evidence pages are intentionally different: paginated policy, transaction,
// and similar leaf rows are queried live using the snapshot's frozen
// parameters. A snapshot therefore guarantees a consistent query context for
// evidence, not an immutable copy of row-level data.
//
// Snapshot store TTLs slide on Get and Append. Stores return ErrSnapshotGone for
// expired and unknown IDs; a client receiving that error must fetch a fresh
// dashboard document.
package document

const ContractVersion = "1.0.0"

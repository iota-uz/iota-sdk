# Reusable Lens reports

A Definition stores a permanent source reference, source contract version, view and parameters. It never stores SQL, an executed frame or a snapshot ID. A Placement references that definition and owns its caption, width and parameter overrides. A Composition stores ordered rows of placements.

The host resolves authorized definitions and sources. Catalog is the code-authored adapter; persistence implements the same Host interface. SourceFunc binds domain data to existing Lens DashboardSpec blocks. Compose namespaces each block's datasets, panels, variables and explorers and lowers it into the existing runtime. It does not add a request manager or executor. Grid groups preserve compound blocks, their titles and nested row geometry on the wire.

Parameters are applied in definition → composition → placement order. Unsupported parameters and views fail explicitly. Sources validate parameter values and bind them before building a spec. Calendar/timezone, dimension semantics, SQL and tenant/user scope remain host-owned. Composition.Context supplies immutable host calendar and dashboard/card/section/group layers. Definition.Filters owns saved report defaults. Periods override by scope; dimension constraints intersect with full provenance. Unsupported or conflicting contexts block source execution. ContextSource must translate the resolved context explicitly; no fallback executes a broader query. The wire group carries the same effective context and identity.

Source.Version is a contract compatibility boundary. Source.Revision and Definition.Revision identify current implementations. Resolve freezes the opened definition and source descriptor; Changed reports when a newer host version exists. Reopening resolves the latest compatible source. An incompatible source contract returns FailedPrecondition; no silent fallback.

The Solid createReportDraft API separates edits from acknowledged storage. Cancel restores the saved base; save/publish is an explicit host call carrying expectedRevision. Failure keeps edits; edits made during an earlier save are retained. Storage, ownership, publishing and conflict policy are host responsibilities.

Independent report copies get a new ID. Placement duplicates keep their report reference. A saved relative period remains a relative value in the source's parameter schema; sources resolve it at execution time.

Public boundaries: github.com/iota-uz/iota-sdk/pkg/lens/report and @iota-uz/sdk/lens. Go owns generated TypeScript/Zod definitions under src/reports/contract. No new npm distribution.

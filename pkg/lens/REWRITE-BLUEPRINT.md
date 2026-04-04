# Lens Rewrite Blueprint

## Context

This package is being reshaped around a document-first architecture with these assumptions:

- Lens should support multiple consumers, including SDK and EAI-owned dashboards.
- Backwards compatibility is a migration goal while the document model is rolled out.
- JSON-backed dashboard definitions are the primary authoring format for semantic dashboards.
- Manual/static dashboards still compile through the same document pipeline.

## Target Shape

Lens is split into four layers:

1. `spec`
   Versioned dashboard documents and authoring structs.
2. `compile`
   Deterministic compilation from documents into executable `lens.DashboardSpec` values, plus semantic cube metadata when present.
3. `engine`
   Execution entry point that prepares documents and runs them through runtime.
4. `registry`
   Loading helpers for embedded preset documents.

## Design Rules

- One document model supports both semantic dashboards and manual/static dashboards.
- Semantic dashboards compile into `cube.CubeSpec` first, then into executable dashboard specs.
- Manual dashboards compile directly into executable dashboard specs.
- Runtime execution only receives compiled specs.
- Dynamic values are injected explicitly through compile options.
- EAI controllers can inspect compiled semantic metadata for leaf redirects and drill filter display values before execution.

## Current Implementation

Implemented in this phase:

- `pkg/lens/spec`
  Localized text, versioned documents, manual body definitions, and legacy conversion helpers.
- `pkg/lens/compile`
  Placeholder resolution, `$ref` value binding, semantic compilation, manual compilation, and document merging.
- `pkg/lens/engine`
  Prepare and run helpers over compiled documents.
- `pkg/lens/registry`
  Embedded document loading helpers.

EAI migration in this phase:

- Sales report uses document compilation for its semantic dashboard and drill redirect flow.
- CRM summary report uses document compilation for its semantic dashboard plus prepended KPI and daily-revenue fragments.
- Claims report, referral report, and raw drill dashboards now enter the engine through document wrappers.

## Next Steps

1. Move remaining manual document builders from legacy conversion helpers to native `spec` structs.
2. Add schema validation for document integrity beyond basic version and title checks.
3. Introduce persistent dashboard storage and user-authored documents.
4. Add parser-backed SQL validation for semantic SQL dashboards.
5. Add richer compile-time diagnostics for invalid references, icons, and actions.

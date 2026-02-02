# Artifact Handler Integration Guide

## Overview

The `ArtifactHandler` automatically creates artifacts from tool execution events. This handler **MUST be wired to the EventBus** for artifacts to be persisted.

## Required Wiring

Add the following code during your application initialization (where services are created):

```go
import (
    "github.com/iota-uz/iota-sdk/pkg/bichat/hooks"
    "github.com/iota-uz/iota-sdk/pkg/bichat/hooks/handlers"
)

// 1. Create the artifact handler with the chat repository
artifactHandler := handlers.NewArtifactHandler(chatRepo)

// 2. Subscribe to tool completion events
eventBus.Subscribe(artifactHandler, hooks.EventToolComplete)
```

## Context Requirements

The handler expects the context to have:
- **Tenant ID**: Set via `composables.WithTenantID(ctx, tenantID)`
- **Database connection**: The repository should have access to the database pool

The handler will:
1. Listen for `ToolCompleteEvent` from tools like:
   - `code_interpreter` - Creates `code_output` artifacts
   - `draw_chart` - Creates `chart` artifacts
   - `export_query_to_excel` - Creates `export` artifacts
   - `export_data_to_excel` - Creates `export` artifacts

2. Parse the tool output JSON
3. Create domain.Artifact entities
4. Persist via ChatRepository.SaveArtifact()

## Verification

To verify the handler is working:

1. Execute a tool that generates outputs (e.g., code_interpreter)
2. Query artifacts via GraphQL: `session { artifacts { id type name } }`
3. Check that artifacts are returned

## Troubleshooting

**Artifacts not being created:**
- Verify the handler is subscribed to EventBus
- Check that tools are publishing `ToolCompleteEvent`
- Verify the context has tenant ID set
- Check repository SaveArtifact() for errors

**Context errors:**
- Ensure the context passed to events has database access
- The repository methods should handle transactions internally

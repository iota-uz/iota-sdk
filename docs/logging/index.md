---
layout: default
title: Logging
nav_order: 12
has_children: true
description: "Logging utilities and configuration for IOTA SDK"
---

# Logging

The IOTA SDK provides comprehensive logging utilities with structured logging, context-aware field tagging, and automatic source location tracking.

## Overview

The Logging module (`/logging`) provides:

- **Structured Logging**: JSON-formatted logs with typed fields
- **Context-Aware Fields**: Automatic tenant ID, user ID, and operation tracking
- **Source Location Tagging**: Automatic file and line number tracking via SourceHook
- **Multi-Level Support**: Debug, Info, Warn, Error log levels
- **Field Mapping**: Type-safe field definitions with validation
- **Integration**: Seamless integration with IOTA SDK's request/response cycle

## Key Features

### Structured Output
All logs are emitted in JSON format with consistent field naming, making them ideal for log aggregation and analysis systems.

### Automatic Context Tracking
Logs automatically include:
- `tenant_id`: Current tenant context
- `user_id`: Current user context
- `request_id`: Request identifier for correlation
- `operation`: Named operation for tracking

### Source Location Tracking
The SourceHook implementation automatically captures:
- `source_file`: File path where log was created
- `source_line`: Line number
- `source_function`: Function name

## Quick Start

```go
import "github.com/iota-uz/iota-sdk/pkg/composables"

// Get logger from context
logger := composables.UseLogger(ctx)

// Log with different levels
logger.Debug("Debug message")
logger.Info("Info message")
logger.Warn("Warning message")
logger.Error("Error message")

// Add structured fields
logger.WithField("user_id", userID).Info("User action recorded")
logger.WithFields(map[string]interface{}{
    "action": "create",
    "entity": "invoice",
}).Info("Entity created")
```

## Module Structure

```
modules/logging/
├── permissions/
│   └── constants.go          # RBAC permissions for logging operations
├── infrastructure/
│   └── persistence/
│       ├── logging_repository.go      # Log storage interface
│       └── schema/
│           └── logging-schema.sql     # Database schema
└── module.go                 # Module registration
```

## Next Steps

- Read the [Technical Guide](./technical.md) for implementation details and usage patterns
- Learn about logger configuration in [Configuration](../config.md)
- Integrate logging into your controllers and services

---

For more information, visit the [IOTA SDK GitHub repository](https://github.com/iota-uz/iota-sdk).

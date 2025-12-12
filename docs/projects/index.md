---
layout: default
title: Projects
nav_order: 6
has_children: true
description: "Project Management Module - Track and manage business projects with stages and milestones"
---

# Projects Module

The Projects module provides comprehensive project management capabilities for tracking business projects, organizing them into stages, and managing project payments and deliverables. This module is designed for managing complex projects with multiple phases and tracking financial progress.

## Overview

The Projects module enables organizations to:

- **Create and manage projects** with counterparty (client) associations
- **Organize projects into stages** with individual budgets and timelines
- **Track project progress** through planned and actual dates
- **Manage stage payments** and link them to financial transactions
- **Maintain project history** with complete audit trails

## Key Concepts

### Projects
A project represents a comprehensive business engagement with a specific counterparty (client). Each project can contain multiple stages, each representing distinct phases of work.

**Attributes:**
- `ID`: Unique project identifier (UUID)
- `TenantID`: Multi-tenant isolation
- `CounterpartyID`: Associated client/counterparty
- `Name`: Project name (unique per tenant)
- `Description`: Detailed project information
- `CreatedAt / UpdatedAt`: Temporal tracking

### Project Stages
Stages divide a project into manageable phases, each with its own budget, timeline, and deliverables. Stages are sequentially numbered within a project.

**Attributes:**
- `ID`: Unique stage identifier (UUID)
- `ProjectID`: Parent project reference
- `StageNumber`: Sequential order within project (unique per project)
- `Description`: Stage details
- `TotalAmount`: Stage budget in cents
- `StartDate`: Planned start date
- `PlannedEndDate`: Planned completion date
- `FactualEndDate`: Actual completion date (nullable)
- `CreatedAt / UpdatedAt`: Temporal tracking

### Project Stage Payments
Payments linked to project stages, creating an association between financial transactions and project phases.

**Attributes:**
- `ID`: Unique payment link identifier (UUID)
- `ProjectStageID`: Parent stage reference
- `PaymentID`: Associated payment from finance module
- `CreatedAt`: Creation timestamp

## Module Architecture

The Projects module follows Domain-Driven Design (DDD) principles with clear separation of concerns:

```
modules/projects/
├── domain/
│   ├── aggregates/
│   │   ├── project/
│   │   │   ├── project.go              # Project aggregate interface
│   │   │   ├── project_impl.go         # Project implementation
│   │   │   ├── project_repository.go   # Repository interface
│   │   │   └── project_events.go       # Domain events
│   │   └── project_stage/
│   │       ├── project_stage.go
│   │       ├── project_stage_impl.go
│   │       ├── project_stage_repository.go
│   │       └── project_stage_events.go
│   └── value_objects/
├── infrastructure/
│   ├── persistence/
│   │   ├── project_repository.go       # Repository implementation
│   │   ├── project_stage_repository.go
│   │   ├── projects_mappers.go         # Domain <-> Persistence mapping
│   │   ├── models/
│   │   │   └── models.go               # Persistence models
│   │   └── queries/
│   └── query/
├── services/
│   ├── project_service.go              # Business logic for projects
│   └── project_stage_service.go        # Business logic for stages
├── presentation/
│   ├── controllers/                    # HTTP handlers
│   ├── viewmodels/                     # Data transformation
│   ├── templates/                      # UI templates (Templ)
│   ├── locales/                        # Translations (i18n)
│   └── mappers/                        # DTO mappings
├── permissions/
│   └── constants.go                    # RBAC permissions
└── module.go                           # Module registration
```

## Integration Points

### With Finance Module
Projects integrate with the Finance module through:
- **Counterparties**: Projects are linked to specific counterparties (clients)
- **Payments**: Project stages can be associated with payment transactions
- **Financial Tracking**: Enable project-based financial reporting

### With Core Module
- **Multi-tenant**: Complete tenant isolation
- **RBAC**: Permission-based access control
- **Audit Logging**: Action logging for compliance

### Event Bus
Projects publish domain events for:
- Project creation, update, and deletion
- Stage creation, update, and deletion
- Payment linking and unlinking

## Documentation Structure

- **[Business Requirements](./business.md)** - Problem statement, workflows, and business rules
- **[Technical Architecture](./technical.md)** - Implementation details, patterns, and API contracts
- **[Data Model](./data-model.md)** - Database schema, ERD diagrams, and relationships

## Common Tasks

### Creating a Project
1. Ensure the counterparty (client) exists in the Finance module
2. Call `ProjectService.Save()` with a new Project aggregate
3. System publishes `ProjectCreatedEvent`
4. UI updates in real-time via HTMX

### Managing Project Stages
1. Create stages with `ProjectStageService.Save()`
2. Set sequential stage numbers and budgets
3. Link payments as work progresses
4. Update actual end dates upon completion

### Querying Projects
- `ProjectService.GetAll()` - All projects for tenant
- `ProjectService.GetPaginated()` - Paginated results with sorting
- `ProjectService.GetByCounterpartyID()` - Projects for specific client

## Permissions

The Projects module enforces role-based access control:

- `project.view` - View projects and stages
- `project.create` - Create new projects
- `project.edit` - Modify project and stage details
- `project.delete` - Archive/delete projects
- `project.manage` - Full project administration

## Next Steps

Explore the [Business Requirements](./business.md) to understand workflows and constraints, then review [Technical Architecture](./technical.md) for implementation details.

---
layout: default
title: HRM
nav_order: 7
has_children: true
description: "Human Resource Management Module - Manage employees, positions, and organizational structure"
---

# HRM Module

The HRM (Human Resource Management) module provides comprehensive employee and organizational management capabilities. It enables organizations to manage their workforce, track employee information, assign positions, and maintain organizational structure.

## Overview

The HRM module enables organizations to:

- **Manage employees** with complete personal and professional information
- **Define and assign positions** within the organization
- **Track employment history** including hire dates and resignation dates
- **Manage employee metadata** such as languages, tax identifiers, and personal documents
- **Maintain salary and compensation** information
- **Support multi-language** employee communication preferences

## Key Concepts

### Employees

An employee represents a person employed by the organization. Each employee has:

**Core Information:**
- `ID`: Unique employee identifier (numeric)
- `TenantID`: Multi-tenant isolation
- `FirstName`, `LastName`, `MiddleName`: Name components
- `Email`: Employee email (unique per tenant)
- `Phone`: Contact phone number

**Professional Details:**
- `HireDate`: Employment start date
- `ResignationDate`: Optional employment end date
- `Salary`: Monthly/periodic compensation amount
- `HourlyRate`: Hourly rate for hourly employees
- `Coefficient`: Calculation coefficient for compensation

**Personal Information:**
- `BirthDate`: Date of birth
- `AvatarID`: Reference to employee photo

**Identifiers & Documents:**
- `Tin`: Tax Identification Number (TIN)
- `Pin`: Personal Identification Number (PIN)
- `Passport`: Passport document reference
- `Languages`: Primary and secondary language preferences

**Notes & Metadata:**
- `Notes`: Additional employee information

### Positions

A position represents a role or job title within the organization. Positions can be:

**Attributes:**
- `ID`: Unique position identifier (numeric)
- `TenantID`: Multi-tenant isolation
- `Name`: Position title (unique per tenant)
- `Description`: Role responsibilities and requirements
- `CreatedAt / UpdatedAt`: Temporal tracking

### Employee-Position Assignment

Employees can be assigned to one or more positions, enabling:
- Role-based organization
- Clear responsibility assignment
- Flexible position assignment
- Support for multiple roles

## Module Architecture

```
modules/hrm/
├── domain/
│   ├── aggregates/
│   │   └── employee/
│   │       ├── employee.go             # Employee aggregate interface
│   │       ├── employee_impl.go        # Implementation
│   │       ├── employee_repository.go  # Repository interface
│   │       ├── employee_events.go      # Domain events
│   │       ├── employee_create_dto.go  # Creation DTO
│   │       ├── employee_update_dto.go  # Update DTO
│   │       └── language_impl.go        # Language value object
│   └── entities/
│       └── position/
│           ├── position.go             # Position entity
│           ├── position_repository.go  # Repository interface
│           └── position_impl.go        # Implementation
├── infrastructure/
│   ├── persistence/
│   │   ├── employee_repository.go      # PostgreSQL implementation
│   │   ├── position_repository.go      # Position persistence
│   │   ├── hrm_mappers.go              # Domain <-> Persistence mapping
│   │   ├── models/
│   │   │   └── models.go               # Persistence models
│   │   └── queries/
│   └── providers/
├── services/
│   ├── employee_service.go             # Business logic for employees
│   └── position_service.go             # Business logic for positions
├── presentation/
│   ├── controllers/
│   │   ├── employee_controller.go      # HTTP handlers
│   │   └── position_controller.go
│   ├── viewmodels/
│   │   └── viewmodels.go               # Data transformation
│   ├── templates/pages/
│   │   └── employees/                  # Employee UI
│   ├── mappers/
│   │   └── mappers.go                  # DTO transformations
│   ├── locales/
│   │   ├── en.toml
│   │   ├── ru.toml
│   │   └── uz.toml
│   └── forms/                          # Form DTOs
├── permissions/
│   └── constants.go                    # RBAC permissions
├── links.go                            # Module route registration
└── module.go                           # Module initialization
```

## Integration Points

### With Finance Module
- Employee salary information integrated with payroll
- Compensation tracking and reporting
- Currency support for salary amounts

### With Core Module
- Multi-tenant support with tenant isolation
- User accounts linked to employees
- RBAC for HR operations
- Audit logging for changes

### With Passport Module
- Passport document references
- Legal identification tracking
- Compliance and verification

### Event Bus
Employees module publishes domain events for:
- Employee creation, update, and deletion
- Position assignments
- Status changes (resignation, etc.)

## Common Tasks

### Creating an Employee
1. Prepare employee information (name, email, hire date, etc.)
2. Create create DTO with employee details
3. Call `EmployeeService.Create(ctx, dto)`
4. System assigns unique employee ID
5. Publishes `EmployeeCreatedEvent`

### Assigning Positions
1. Ensure position exists in organization
2. Link employee to position
3. Track assignment history
4. Update organizational structure

### Managing Employee Status
1. Track hire date (employment start)
2. Set resignation date when employee leaves
3. Update system with status changes
4. Maintain historical records

### Querying Employees
- `EmployeeService.GetAll()` - All employees for tenant
- `EmployeeService.GetPaginated()` - Paginated results with filtering
- `EmployeeService.GetByID()` - Specific employee details

## Permissions

The HRM module enforces role-based access control:

- `hrm.view` - View employees and positions
- `hrm.create` - Create new employees
- `hrm.edit` - Modify employee details
- `hrm.delete` - Remove employees
- `hrm.export` - Export employee data
- `hrm.positions.manage` - Manage positions
- `hrm.bulk` - Bulk operations

## Documentation Structure

- **[Business Requirements](./business.md)** - Problem statement, workflows, and business rules
- **[Technical Architecture](./technical.md)** - Implementation details, patterns, and API contracts
- **[Data Model](./data-model.md)** - Database schema, ERD diagrams, and relationships

## Next Steps

Explore the [Business Requirements](./business.md) to understand HR workflows and constraints, then review [Technical Architecture](./technical.md) for implementation details.

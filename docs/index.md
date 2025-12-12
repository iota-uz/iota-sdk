---
layout: default
title: IOTA SDK Documentation
nav_order: 0
description: "Multi-tenant business management platform SDK for Go"
---

# IOTA SDK Documentation

Welcome to the IOTA SDK documentation. IOTA SDK is a comprehensive, modular business management platform written in Go, designed for multi-tenant deployments with enterprise-grade features for financial management, CRM, warehouse operations, project management, and human resources.

## Platform Overview

IOTA SDK provides a production-ready foundation for building scalable business applications with built-in support for:

- **Multi-tenant Architecture**: Complete tenant isolation with organization-level granularity
- **Modular Design**: Composable domain-driven modules that can be mixed and matched
- **Enterprise Security**: Role-based access control (RBAC), permission-based authorization, and secure session management
- **Database-First Approach**: PostgreSQL with comprehensive migration support
- **Type-Safe Templates**: Templ-based templates with HTMX integration for reactive UI
- **Comprehensive Testing**: Integration test framework (ITF) with full coverage utilities

## Technology Stack

| Category | Technology | Version |
|----------|-----------|---------|
| **Language** | Go | 1.23.2 |
| **Database** | PostgreSQL | 13+ |
| **Frontend Framework** | HTMX | Latest |
| **Reactive Framework** | Alpine.js | Latest |
| **Templating** | Templ | 0.3.857+ |
| **Styling** | Tailwind CSS | 3.4.13+ |
| **Testing** | Playwright | Latest |
| **Linting** | golangci-lint | 1.64.8+ |

## Core Modules

### Core Module (`/core`)
Foundation module providing essential platform functionality:
- **Users**: User management, authentication, profile settings
- **Roles**: Role definition and assignment
- **Groups**: User grouping and organization
- **Settings**: System and tenant settings
- **Dashboard**: Main dashboard view
- **Account**: User account management

### Finance Module (`/finance`)
Comprehensive financial management for transactions, payments, and accounting:
- **Payments**: Payment processing and management
- **Expenses**: Expense tracking and categorization
- **Transactions**: General ledger and transaction records
- **Money Accounts**: Bank and financial account management
- **Debts**: Debt tracking and management
- **Counterparties**: Vendor and customer management
- **Reports**: Financial reporting and analytics

### CRM Module (`/crm`)
Customer relationship management and communication:
- **Clients**: Customer/client database and profiles
- **Chats**: Internal and external messaging
- **Message Templates**: Pre-defined communication templates

### Warehouse Module (`/warehouse`)
Inventory and warehouse management:
- **Inventory**: Stock tracking and management
- **Products**: Product catalog and specifications
- **Orders**: Purchase and sales orders
- **Positions**: Product positions and variants
- **Units**: Measurement units and conversions

### Projects Module (`/projects`)
Project tracking and management:
- **Projects**: Project creation and tracking
- **Project Stages**: Stage/phase management within projects

### HRM Module (`/hrm`)
Human resource management:
- **Employees**: Employee database and records

### Billing Module (`/billing`)
Subscription and billing management:
- **Billing Dashboard**: Subscription overview and management
- **Payment Processing**: Stripe integration for payment processing

### SuperAdmin Module (`/superadmin`)
Platform-wide administration and analytics:
- **Dashboard**: Platform analytics and monitoring
- **Tenants**: Tenant management and configuration

### BiChat Module (`/bichat`)
Real-time chat functionality:
- Chat infrastructure and messaging

### Website Module (`/website`)
Public-facing website and pages:
- Landing pages and public content

### Logging Module (`/logging`)
Application logging utilities and configuration

### Testkit Module (`/testkit`)
Testing utilities and integration test framework (ITF)

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    Presentation Layer                       │
│  ┌─────────────────┬──────────────────┬──────────────────┐  │
│  │  Controllers    │   ViewModels     │   Templates      │  │
│  │  (HTTP)         │   (Data Mapping) │   (Templ)        │  │
│  └─────────────────┴──────────────────┴──────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                              ▲
                              │
┌─────────────────────────────────────────────────────────────┐
│                    Service Layer                            │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Business Logic, Validation, Permissions            │  │
│  │  Domain Services with Repository Dependencies       │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                              ▲
                              │
┌─────────────────────────────────────────────────────────────┐
│                   Domain Layer (DDD)                        │
│  ┌──────────────┬──────────────┬──────────────────────────┐ │
│  │  Aggregates  │  Entities    │  Value Objects           │ │
│  │  & Rules     │  & Behaviors │  & Domain Logic          │ │
│  └──────────────┴──────────────┴──────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                              ▲
                              │
┌─────────────────────────────────────────────────────────────┐
│                  Repository Layer                           │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Repository Interfaces  │  PostgreSQL Implementation │  │
│  │  (Domain Layer)         │  (Infrastructure Layer)    │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                              ▲
                              │
┌─────────────────────────────────────────────────────────────┐
│                   Data Access Layer                         │
│  ┌──────────────────────────────────────────────────────┐  │
│  │        PostgreSQL Database with Migrations           │  │
│  │        Multi-tenant Support with Isolation           │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

## Quick Links

- **GitHub Repository**: [iota-uz/iota-sdk](https://github.com/iota-uz/iota-sdk)
- **Issue Tracker**: [GitHub Issues](https://github.com/iota-uz/iota-sdk/issues)
- **Releases**: [GitHub Releases](https://github.com/iota-uz/iota-sdk/releases)

## Getting Started

Start with the [Getting Started](./getting-started/) section to learn how to install and set up IOTA SDK for development.

---

For more information or questions, please visit our [GitHub repository](https://github.com/iota-uz/iota-sdk) or open an issue.

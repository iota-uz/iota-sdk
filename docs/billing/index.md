---
layout: default
title: Billing
nav_order: 8
has_children: true
description: "Billing Module - Payment processing, transactions, and multi-gateway integration"
---

# Billing Module

The Billing module provides comprehensive payment processing and transaction management capabilities. It supports multiple payment gateways (Stripe, Click, Payme, Octo), flexible payment methods (cash, custom integrators), and complete transaction lifecycle management with reconciliation and refund support.

## Overview

The Billing module enables organizations to:

- **Process payments** through multiple payment gateways
- **Manage transactions** with complete lifecycle (pending, completed, canceled, refunded)
- **Support multiple gateways**:
  - **Stripe**: Subscription billing and one-time payments
  - **Click**: Uzbek payment system (Click UZ)
  - **Payme**: Uzbek payment aggregator
  - **Octo**: Card processing
  - **Cash**: Manual cash payments
  - **Integrator**: Custom payment integrations

- **Track payment status** with detailed transaction records
- **Handle refunds** and cancellations
- **Maintain audit trails** for compliance
- **Enable reconciliation** with payment processors

## Key Concepts

### Transaction

A transaction represents a single payment event. Each transaction has:

**Core Information:**
- `ID`: Unique transaction identifier (UUID)
- `TenantID`: Multi-tenant isolation
- `Status`: Payment status (Pending, Completed, Canceled, Refunded, PartiallyRefunded)
- `Quantity`: Amount in minor currency units (cents)
- `Currency`: ISO 4217 currency code
- `Gateway`: Payment processor (stripe, click, payme, octo, cash, integrator)
- `Details`: Gateway-specific details (JSON)
- `CreatedAt / UpdatedAt`: Temporal tracking

### Payment Gateway Abstraction

The module provides a gateway abstraction supporting:

1. **Stripe Gateway**:
   - Subscription billing
   - Session-based payments
   - Customer management
   - Trial periods

2. **Click Gateway**:
   - Prepare/confirm flow
   - Merchant/shop configuration
   - Status tracking

3. **Payme Gateway**:
   - Transaction states
   - Receiver accounts
   - Reason codes for failures

4. **Octo Gateway**:
   - Card type and country tracking
   - Auto-capture options
   - Risk assessment

5. **Cash Gateway**:
   - Simple cash transactions
   - No external integration
   - Manual reconciliation

6. **Integrator Gateway**:
   - Custom payment processor
   - Extensible error handling
   - Provider-specific data

### Payment Details

Gateway-specific transaction details stored as JSON:
- `ClickDetails`: Merchant IDs, prepare/confirm IDs, payment status
- `PaymeDetails`: Transaction states, account info, receivers
- `OctoDetails`: Card info, RRN, risk levels
- `StripeDetails`: Session/subscription IDs, items, URLs
- `CashDetails`: Custom data map
- `IntegratorDetails`: Custom data with error codes

## Module Architecture

```
modules/billing/
├── domain/
│   ├── aggregates/
│   │   ├── billing/
│   │   │   ├── billing.go             # Transaction aggregate interface
│   │   │   ├── billing_impl.go        # Implementation
│   │   │   ├── billing_repository.go  # Repository interface
│   │   │   ├── billing_events.go      # Domain events
│   │   │   ├── gateway.go             # Gateway abstraction
│   │   │   └── provider.go            # Provider interface
│   │   └── details/
│   │       ├── details.go             # Details interfaces
│   │       ├── click_details_impl.go  # Click implementation
│   │       ├── payme_details_impl.go  # Payme implementation
│   │       ├── octo_details_impl.go   # Octo implementation
│   │       ├── stripe_details_impl.go # Stripe implementation
│   │       ├── cash_details_impl.go   # Cash implementation
│   │       └── integrator_details_impl.go
├── infrastructure/
│   ├── persistence/
│   │   ├── billing_repository.go      # PostgreSQL implementation
│   │   ├── billing_mappers.go         # Domain <-> Persistence mapping
│   │   ├── models/
│   │   │   └── models.go              # Persistence models
│   │   └── queries/
│   ├── providers/
│   │   ├── stripe_provider.go         # Stripe integration
│   │   ├── click_provider.go          # Click integration
│   │   ├── payme_provider.go          # Payme integration
│   │   └── octo_provider.go           # Octo integration
│   └── callbacks/
├── services/
│   ├── billing_service.go             # Transaction management
│   └── setup_test.go
├── presentation/
│   ├── controllers/
│   │   └── billing_controller.go      # HTTP handlers (webhooks, status)
│   └── locales/
│       ├── en.toml
│       ├── ru.toml
│       └── uz.toml
├── permissions/
│   └── constants.go                   # RBAC permissions
├── module.go                          # Module initialization
└── links.go                           # Route registration
```

## Integration Points

### With Finance Module
- Payment transactions feed into accounting
- Currency conversions handled
- Financial reporting integration
- Reconciliation support

### With Projects Module
- Payments linked to project stages
- Project-based payment tracking
- Milestone payment support

### Event Bus
Billing publishes domain events for:
- Transaction creation, updates, deletion
- Status changes (completed, refunded, canceled)
- Provider-specific events (subscription changes)
- Refund events

## Common Tasks

### Creating a Transaction

1. Prepare payment details for gateway
2. Create transaction with amount, currency, gateway
3. Call `BillingService.Create(ctx, command)`
4. System integrates with payment processor
5. Publishes `TransactionCreatedEvent`

### Processing Payments

1. **Stripe**: Create checkout session, customer redirects
2. **Click**: Prepare transaction, receive callback notification
3. **Payme**: Accept transaction, handle payment confirm
4. **Octo**: Initialize payment, handle redirect
5. **Cash**: Record manual payment, mark complete

### Handling Refunds

1. Get transaction by ID
2. Call `BillingService.Refund(ctx, command)`
3. Provider processes refund (if applicable)
4. Status updated (Refunded or PartiallyRefunded)
5. Publishes `TransactionUpdatedEvent`

### Webhook Handling

Payment gateways send webhooks for:
- Payment confirmation
- Refund notifications
- Subscription events
- Error notifications

### Querying Transactions

- `BillingService.GetByID()` - Single transaction
- `BillingService.GetPaginated()` - Paginated results
- `BillingService.GetByDetailsFields()` - Query by gateway-specific fields
- `BillingService.Count()` - Transaction count

## Permissions

The Billing module enforces role-based access control:

- `billing.view` - View transactions
- `billing.create` - Create transactions
- `billing.refund` - Process refunds
- `billing.cancel` - Cancel transactions
- `billing.reconcile` - Reconciliation operations
- `billing.gateways.manage` - Configure payment gateways

## Documentation Structure

- **[Business Requirements](./business.md)** - Problem statement, payment workflows, and business rules
- **[Technical Architecture](./technical.md)** - Implementation details, gateway integration patterns
- **[Data Model](./data-model.md)** - Database schema, ERD diagrams, and transaction states

## Next Steps

Explore the [Business Requirements](./business.md) to understand payment workflows and gateway integration, then review [Technical Architecture](./technical.md) for implementation details.

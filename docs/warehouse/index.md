---
layout: default
title: Warehouse Module
nav_order: 5
has_children: true
permalink: /docs/warehouse
---

# Warehouse Module

Comprehensive inventory and warehouse management system for IOTA SDK providing product tracking, position management, order processing, and inventory verification.

## Overview

The Warehouse module enables businesses to:

- **Product Management**: Track individual products with RFID tags and status
- **Position Cataloging**: Maintain warehouse positions (SKUs) with titles, barcodes, and images
- **Inventory Control**: Count and verify actual vs. expected inventory
- **Order Management**: Process warehouse orders (inbound/outbound)
- **Unit Definition**: Define measurement units for quantities
- **RFID Tracking**: Track products using radio-frequency identification

## Key Features

| Feature | Description |
|---------|-------------|
| **Position Management** | Create and maintain warehouse positions (SKUs) with images and barcodes |
| **Product Tracking** | Individual product records with RFID tags and status tracking |
| **Inventory Checks** | Periodic inventory verification with variance reporting |
| **Orders** | Process inbound/outbound warehouse orders |
| **Units** | Configurable measurement units (kg, pcs, meters, etc.) |
| **Status Tracking** | Products and orders with lifecycle status management |
| **Batch Operations** | Import positions and manage bulk updates |
| **GraphQL API** | Schema for programmatic access to warehouse data |
| **Event System** | Publish events for inventory and order changes |

## Architecture Boundaries

```
┌──────────────────────────────────────────────────┐
│        Presentation Layer                         │
│  Controllers → ViewModels → Templates → DTOs    │
├──────────────────────────────────────────────────┤
│        Service Layer                              │
│  PositionService → ProductService →              │
│  OrderService → InventoryService → UnitService  │
│                   ↓ Event Publishing             │
├──────────────────────────────────────────────────┤
│        Repository Layer                           │
│  PositionRepository → ProductRepository →        │
│  OrderRepository → InventoryRepository →         │
│  UnitRepository                                  │
├──────────────────────────────────────────────────┤
│        Domain Layer                               │
│  Position ← Products                             │
│  Order ← Items ← Products                        │
│  Inventory Check ← Results                       │
│  Unit                                            │
└──────────────────────────────────────────────────┘
```

## Core Entities

| Entity | Purpose | Relationships |
|--------|---------|---------------|
| **Position** | Warehouse SKU/catalog item | Contains multiple Products |
| **Product** | Individual tracked item | Belongs to Position |
| **Order** | Warehouse order (in/out) | Contains multiple Items |
| **Item** | Order line item | References Product and Position |
| **Inventory Check** | Physical count verification | Contains multiple Results |
| **Check Result** | Expected vs. actual count | References Position |
| **Unit** | Measurement unit definition | Referenced by Positions |

## Integration Points

| Component | Integration | Purpose |
|-----------|-------------|---------|
| **Position Service** | Upload Service | Store position images |
| **Inventory Service** | User Service | Track who performed count |
| **Order Service** | Event Bus | Publish order events |
| **GraphQL API** | Warehouse Resolver | Programmatic access |
| **Event Bus** | Event Publishing | Domain event distribution |

## Document Map

- [Business Requirements](./business.md) - Problem statement, workflows, and business rules
- [Technical Architecture](./technical.md) - Code structure, layer separation, and implementation patterns
- [Data Model](./data-model.md) - Database schema, entity relationships, and constraints
- [User Experience](./ux.md) - UI workflows, page structure, and interaction patterns

## Quick Links

- **Package**: `github.com/iota-uz/iota-sdk/modules/warehouse`
- **Routes**: `/warehouse/products`, `/warehouse/positions`, `/warehouse/orders`, `/warehouse/inventory`, `/warehouse/units`
- **Services**: `ProductService`, `PositionService`, `OrderService`, `InventoryService`, `UnitService`
- **Repositories**: `ProductRepository`, `PositionRepository`, `OrderRepository`, `InventoryRepository`, `UnitRepository`
- **GraphQL**: `/warehouse/graphql` with resolvers for products, positions, orders
- **Permissions**: `ProductRead`, `ProductCreate`, `PositionRead`, `PositionCreate`, `OrderRead`, `OrderCreate`, `InventoryRead`, `InventoryCreate`, etc.

## Multi-Tenant Support

All warehouse entities include tenant isolation:

- **Position**: Scoped to tenant via `tenant_id`
- **Product**: Scoped to tenant via `tenant_id`
- **Order**: Scoped to tenant via `tenant_id`
- **Unit**: Scoped to tenant via `tenant_id`
- **Inventory Check**: Scoped to tenant via `tenant_id`

Query filters automatically include tenant isolation via repository implementations.

## Event System

The Warehouse module publishes domain events:

- `PositionCreatedEvent` - When a new position is added to catalog
- `PositionUpdatedEvent` - When position details are modified
- `ProductCreatedEvent` - When a new product instance is created
- `ProductStatusChangedEvent` - When product status changes
- `OrderCreatedEvent` - When a new order is initiated
- `OrderCompletedEvent` - When an order is marked complete
- `InventoryCheckCreatedEvent` - When inventory count is recorded
- `InventoryVarianceEvent` - When actual vs. expected differ significantly

Events include full entity data for integration with other modules.

## Data Model Overview

### Products
- Stored with RFID tags for tracking
- Associated with Positions (catalog items)
- Status lifecycle: Available, Reserved, Damaged, Missing
- Timestamps for creation and updates

### Positions
- Warehouse SKU/catalog entry
- Contains title, barcode, unit
- Supports multiple product images
- Tracks total quantity available

### Orders
- Type: Inbound (receiving) or Outbound (shipping)
- Status: Draft, Processing, Completed
- Contains multiple items (positions + products)
- Timestamps for lifecycle tracking

### Inventory Checks
- Periodic physical count verification
- Records expected vs. actual quantities
- Tracks variance and differences
- Records who performed the count and when

### Units
- Custom measurement units (kg, pcs, boxes, etc.)
- Title and short abbreviation
- Tenant-scoped for consistency

## Key Workflows

1. **Add Product to Inventory**
   - Create Position (SKU)
   - Add images for Position
   - Create Products with RFID tags
   - Set initial status (Available)

2. **Process Inbound Order**
   - Create Order (Inbound)
   - Add items (Positions + Products)
   - Update product status
   - Mark order complete

3. **Process Outbound Order**
   - Create Order (Outbound)
   - Select Products to ship
   - Update product status (Reserved → Shipped)
   - Mark order complete

4. **Verify Inventory**
   - Create Inventory Check
   - Scan or count each Position
   - Compare expected vs. actual
   - Record variances
   - Generate report

5. **Manage Catalog**
   - Create/Update Positions
   - Upload position images
   - Define measurement units
   - Configure barcode formats

## Performance Characteristics

- Product lookup by RFID: < 50ms (indexed)
- Position list with pagination: < 200ms
- Inventory check calculation: < 1s (1000+ items)
- GraphQL query resolution: < 500ms
- Supports 10,000+ products per tenant
- Supports 100+ concurrent inventory checks

## Scalability

- Horizontal scaling via GraphQL layer
- Batch import for bulk position creation
- Pagination for large result sets
- Indexed queries for RFID lookup
- Event-driven integration for other modules

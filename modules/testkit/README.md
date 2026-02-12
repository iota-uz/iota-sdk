# TestKit Module

The TestKit module provides dedicated test endpoints for E2E testing that allow controlled data population and database management during test execution.

## Overview

This module creates a separate, secure testing interface that is only active when the `ENABLE_TEST_ENDPOINTS=true` environment variable is set. It provides REST endpoints for:

- **Database Reset**: Clean truncation and optional reseeding
- **Data Population**: Flexible JSON-based data creation with reference resolution
- **Preset Scenarios**: Pre-built data configurations for common test cases

## Security

⚠️ **IMPORTANT**: Test endpoints are only available when `ENABLE_TEST_ENDPOINTS=true` is set in the environment. This module includes multiple safety layers:

- Environment variable check at module registration
- Runtime checks in controllers
- Logging of all test endpoint usage
- Designed for CI/CD and local testing only

## Endpoints

### `POST /__test__/reset`

Truncates all database tables (except migrations) and optionally reseeds with minimal data.

**Request Body:**
```json
{
  "reseedMinimal": true
}
```

**Response:**
```json
{
  "success": true,
  "message": "Database reset successfully",
  "reseedMinimal": true
}
```

### `POST /__test__/populate`

Populates database with custom data using JSON specification with reference resolution.

**Request Body:**
```json
{
  "version": "1.0",
  "tenant": {
    "id": "00000000-0000-0000-0000-000000000001",
    "name": "Test Tenant",
    "domain": "test.localhost"
  },
  "data": {
    "users": [{
      "email": "test@example.com",
      "password": "TestPass123!",
      "firstName": "Test",
      "lastName": "User",
      "language": "en",
      "_ref": "testUser"
    }],
    "finance": {
      "moneyAccounts": [{
        "name": "Test Account",
        "currency": "USD",
        "balance": 1000.00,
        "type": "cash",
        "_ref": "testAccount"
      }],
      "paymentCategories": [{
        "name": "Sales Revenue",
        "type": "income",
        "_ref": "salesCategory"
      }],
      "payments": [{
        "amount": 500.00,
        "date": "2024-01-15",
        "accountRef": "@moneyAccounts.testAccount",
        "categoryRef": "@paymentCategories.salesCategory",
        "comment": "Test payment"
      }]
    }
  },
  "options": {
    "clearExisting": false,
    "returnIds": true,
    "validateReferences": true,
    "stopOnError": true
  }
}
```

**Response:**
```json
{
  "success": true,
  "message": "Data populated successfully",
  "data": {
    "users": [{"email": "test@example.com", "ref": "testUser"}],
    "moneyAccounts": [{"name": "Test Account", "ref": "testAccount"}]
  }
}
```

### `POST /__test__/seed`

Seeds database with predefined scenarios.

**Request Body:**
```json
{
  "scenario": "minimal"
}
```

**Available Scenarios:**
- `minimal`: Basic setup with default tenant and test user
- `finance`: Finance module focused setup with accounts, categories, and transactions
- `warehouse`: Warehouse module setup with units, products, and inventory
- `comprehensive`: Full setup with data across all modules

### `GET /__test__/seed`

Lists available scenarios with descriptions.

**Response:**
```json
{
  "success": true,
  "scenarios": [
    {
      "name": "minimal",
      "description": "Basic setup with default tenant and test user"
    },
    {
      "name": "finance",
      "description": "Finance module focused setup with accounts, categories, and sample transactions"
    }
  ]
}
```

### `GET /__test__/health`

Health check endpoint for test infrastructure.

**Response:**
```json
{
  "success": true,
  "message": "Test endpoints are healthy",
  "config": {
    "enableTestEndpoints": true,
    "environment": "test"
  }
}
```

## Reference System

The populate endpoint supports a reference system for linking related entities:

### Defining References

Add a `_ref` field to any entity to create a reference:

```json
{
  "moneyAccounts": [{
    "name": "Main Account",
    "currency": "USD",
    "_ref": "mainAccount"
  }]
}
```

### Using References

Reference other entities using `@category.referenceKey` syntax:

```json
{
  "payments": [{
    "amount": 100.00,
    "accountRef": "@moneyAccounts.mainAccount",
    "categoryRef": "@paymentCategories.salesRevenue"
  }]
}
```

## E2E Testing Integration

The module provides test data fixtures and utilities for E2E testing:

```typescript
// Reset database
await resetTestDatabase({ reseedMinimal: true });

// Populate custom data
await populateTestData(dataSpecification);

// Seed predefined scenario
await seedScenario("finance");

// Get available scenarios
const scenarios = await getAvailableScenarios();

// Health check
await checkTestEndpointsHealth();
```

### Test Data Fixtures

Convenient fixtures for common test data:

```typescript
import { createUser, createFinanceScenario } from '../fixtures/test-data';

// Create user specification
const user = createUser({
  email: "custom@test.com",
  firstName: "Custom"
});

// Create complete finance scenario
const financeData = createFinanceScenario({
  data: {
    users: [user]
  }
});

await populateTestData(financeData);
```

## Environment Setup

### Local Development

Set environment variable in your e2e development server:

```bash
ENABLE_TEST_ENDPOINTS=true just e2e dev
```

### CI/CD Integration

The environment variable is automatically set in `.github/workflows/test.yml` for E2E test jobs.

## Data Structure Support

### Core Module
- Users with permissions and language settings
- Tenants with domain configuration

### Finance Module
- Money accounts (cash, bank) with balances
- Payment/expense categories
- Payments and expenses with attachments
- Counterparties (individuals, companies)
- Debts (receivable, payable)

### CRM Module
- Clients with contact information

### Warehouse Module
- Units of measurement
- Products with pricing

## Development Notes

### Adding New Entity Support

1. Add entity specification to `domain/schemas/populate_schema.go`
2. Implement creation logic in `services/populate_service.go`
3. Add preset scenarios in `services/test_data_service.go`
4. Create TypeScript fixtures in `e2e/fixtures/test-data.ts`

### Error Handling

All endpoints use transactions and roll back on any error. Detailed error messages are returned for debugging.

### Performance Considerations

- Database operations use transactions for atomicity
- Reference resolution is done in memory for speed
- Bulk operations are preferred over individual inserts

## Best Practices

1. **Use References**: Prefer `_ref` system over hardcoded IDs
2. **Scenario Testing**: Use predefined scenarios for consistent testing
3. **Clean State**: Always reset database between test suites
4. **Error Validation**: Test both success and error scenarios
5. **Security**: Never use in production environments

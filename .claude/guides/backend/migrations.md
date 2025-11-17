# Database Migrations Guide

**PostgreSQL schema migrations using sql-migrate for the IOTA SDK platform.**

## Overview

### Purpose

Use migrations for:
- Schema changes (CREATE/ALTER/DROP tables, columns, indexes, constraints)
- PL/pgSQL functions and procedures
- Data backfills after schema changes
- Index creation for performance

### Critical Rules

1. **NEVER edit existing migration files** - immutable once created
2. **ALWAYS include tenant_id** for multi-tenant isolation (except system tables)
3. **ALWAYS provide Down migrations** that fully reverse Up changes
4. **ALWAYS use Unix timestamp** in filename: `migrations/changes-{timestamp}.sql`
5. **NEVER use raw SQL in application code** - all schema changes via migrations
6. **NEVER use anonymous code blocks (DO $$ ... $$)** in migrations - not supported
7. **NEVER use BEGIN/COMMIT/ROLLBACK** in migrations - transactions handled by migration tool

## Migration File Format

### Basic Structure

```sql
-- Migration: [Brief description]
-- Date: YYYY-MM-DD
-- Purpose: [Detailed explanation]

-- +migrate Up
[SQL STATEMENTS];

-- +migrate Down
[REVERSE SQL STATEMENTS];
```

### Directives

| Directive                    | Purpose                 | Usage                              |
|------------------------------|-------------------------|------------------------------------|
| `-- +migrate Up`             | Forward migration       | Schema changes                     |
| `-- +migrate Down`           | Rollback migration      | MUST exactly reverse Up            |
| `-- +migrate StatementBegin` | Start complex statement | PL/pgSQL functions with semicolons |
| `-- +migrate StatementEnd`   | End complex statement   | Close the block                    |
| `-- +migrate notransaction`  | Disable transaction     | Concurrent indexes only            |

### PL/pgSQL Functions

Use `StatementBegin/End` for functions containing semicolons:

```sql
-- +migrate Up

-- +migrate StatementBegin
CREATE OR REPLACE FUNCTION money_display(amount bigint, currency_code text)
    RETURNS text
    LANGUAGE plpgsql
    IMMUTABLE
AS
$$
BEGIN
    IF amount IS NULL OR amount = 0 THEN
        RETURN '0,00 ' || COALESCE(currency_code, 'UZS');
    END IF;

    RETURN CONCAT(
            REPLACE(REPLACE(TO_CHAR(amount / 100.0, 'FM999,999,999,999.00'), ',', ' '), '.', ','),
            ' ',
            COALESCE(currency_code, 'UZS')
           );
END;
$$;
-- +migrate StatementEnd

COMMENT ON FUNCTION money_display(bigint, text) IS
    'Formats monetary amount to localized display string';

-- +migrate Down
DROP FUNCTION IF EXISTS money_display(bigint, text);
```

## Naming & Git Workflow

### Naming Convention

Format: `changes-{unix_timestamp}.sql`

```bash
# Generate timestamp
date +%s
# Output: 1762857604

# Create migration
touch migrations/changes-1762857604.sql
```

### Git-Based Safety

```bash
# Check if migration is committed
git log --oneline -- migrations/changes-1762857604.sql

# If output is empty → Uncommitted → Safe to edit
# If output shows commits → Committed → Create new migration
```

**Rules**:
- Uncommitted migrations: Safe to edit
- Committed migrations: Create new migration instead

## Multi-Tenant Patterns

### Tenant Isolation (CRITICAL)

```sql
-- All tables use tenant_id for multi-tenant isolation
SELECT * FROM users WHERE tenant_id = $1;
SELECT * FROM products WHERE tenant_id = $1;
SELECT * FROM clients WHERE tenant_id = $1;
```

### Standard Table Structure

```sql
CREATE TABLE module_entities (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,

    -- Business fields
    name VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL,

    -- Audit fields (mandatory)
    created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    created_by uuid REFERENCES users(id),
    updated_by uuid REFERENCES users(id),
    deleted_at TIMESTAMPTZ,

    CONSTRAINT fk_tenant FOREIGN KEY (tenant_id)
        REFERENCES tenants(id) ON DELETE CASCADE
);

-- Required indexes
CREATE INDEX idx_module_entities_tenant_id ON module_entities(tenant_id);
CREATE INDEX idx_module_entities_deleted_at ON module_entities(deleted_at);
CREATE INDEX idx_module_entities_status ON module_entities(status) WHERE deleted_at IS NULL;
```

## Domain-Specific Patterns

### Business Entities

**Modules**:
- **Warehouse**: products, inventory, orders, positions, units
- **Finance**: payments, expenses, debts, transactions, counterparties, money_accounts
- **CRM**: clients, chats, message_templates
- **Projects**: projects, project_stages
- **HRM**: employees

### Financial Fields

```sql
amount DECIMAL(10,2) NOT NULL DEFAULT 0.00
rate DECIMAL(10,4) -- per-unit rates
percentage DECIMAL(5,2) CHECK (percentage >= 0 AND percentage <= 100)
```

### Status Enums

```sql
CREATE TYPE order_status AS ENUM ('pending', 'processing', 'completed', 'cancelled');
CREATE TYPE payment_status AS ENUM ('pending', 'processing', 'completed', 'failed', 'refunded');
CREATE TYPE user_status AS ENUM ('active', 'inactive', 'suspended');
```

## Migration Examples

### Adding Tables with Relationships

```sql
-- +migrate Up
CREATE TABLE product_attachments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    product_id uuid NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    file_name VARCHAR(255) NOT NULL,
    file_path VARCHAR(500) NOT NULL,
    file_type VARCHAR(50) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, product_id, file_name)
);

CREATE INDEX idx_product_attachments_tenant_id ON product_attachments(tenant_id);
CREATE INDEX idx_product_attachments_product_id ON product_attachments(product_id);

-- +migrate Down
DROP TABLE product_attachments;
```

### Adding Columns Safely

```sql
-- +migrate Up
ALTER TABLE employees
    ADD COLUMN hire_date DATE,
    ADD COLUMN termination_date DATE;

ALTER TABLE employees
    ADD CONSTRAINT chk_termination_dates
    CHECK (termination_date IS NULL OR termination_date >= hire_date);

-- +migrate Down
ALTER TABLE employees DROP CONSTRAINT chk_termination_dates;
ALTER TABLE employees
    DROP COLUMN termination_date,
    DROP COLUMN hire_date;
```

### Data Migrations with Safety

```sql
-- +migrate Up
ALTER TABLE orders ADD COLUMN is_urgent BOOLEAN DEFAULT FALSE;

UPDATE orders
SET is_urgent = TRUE
WHERE priority = 'high'
AND tenant_id = orders.tenant_id;

-- +migrate Down
ALTER TABLE orders DROP COLUMN is_urgent;
```

## Validation Checklists

### Before Creating Migrations

- [ ] Timestamp: `date +%s` for filename
- [ ] Structure: Both Up and Down sections
- [ ] Reversibility: Down exactly reverses Up
- [ ] Multi-tenant: tenant_id present and cascading
- [ ] Indexes: Created for FKs and common queries
- [ ] Constraints: CHECK, UNIQUE, NOT NULL appropriate
- [ ] Testing: Verified Up→Down→Up cycle works

## Error Prevention

- Verify foreign key targets exist before adding references
- Check naming conflicts with existing tables/columns
- Consider cascade effects (CASCADE, SET NULL, RESTRICT)
- Use CONCURRENTLY for index creation on large tables
- Never DROP columns with data without confirmation
- Ensure application code handles new enum values

## Connection Management

### Strategy

1. Try default local credentials first
2. Check .env file if connection fails
3. Build connection from environment variables
4. Retry with updated credentials

### Connection Examples

```bash
# Default local
PGPASSWORD=postgres psql -h localhost -p 5432 -U postgres -d iota_erp

# Environment-based
if [ -f .env ]; then
  export $(cat .env | grep -E '^DB_' | xargs)
  PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME
fi

# Staging Database (use database-connection skill)
```

## Migration Commands

```bash
# Apply migrations
make db migrate up

# Rollback migrations
make db migrate down

# Check status
make db migrate status
```

## Testing Migrations

Use ITF framework for testing schema changes:

```go
func TestRepository(t *testing.T) {
    env := itf.Setup(t)  // Isolated test database
    repo := env.Repository("EntityRepository").(*EntityRepository)
    // Test with clean, isolated database
}
```

**ITF Test Environment**:
- `itf.Setup(t)` provides isolated test database
- Each test gets clean database state
- Use `env.Repository()` to get repository instances
- Automatic cleanup after test completion

## Best Practices

### Always

1. Use parameterized queries ($1, $2, never concatenate)
2. Include proper JOINs to prevent N+1
3. Add WHERE deleted_at IS NULL for soft deletes
4. Test reversibility (Up→Down→Up cycle)
5. Document complex migrations with clear comments

### Never

1. Edit committed migrations
2. Use raw SQL concatenation
3. Skip tenant isolation checks
4. Forget Down migration
5. Use anonymous code blocks (DO $$ ... $$)

## Integration Points

- **Migrations**: `migrations/` (read-only after creation)
- **Repository interfaces**: `modules/{module}/domain/*/repository.go`
- **Repository implementations**: `modules/{module}/infrastructure/persistence/`
- **Query builders**: `pkg/repo` for SQL construction
- **Error handling**: `pkg/serrors` for database errors
- **Testing**: ITF provides test database isolation via `itf.Setup(t)`

## Security Checklist

- [ ] No raw SQL concatenation
- [ ] All queries parameterized
- [ ] Tenant isolation verified
- [ ] Connection strings secure
- [ ] No credentials in code
- [ ] SQL injection prevention verified

## Remember

- Production multi-tenant system
- Data isolation is CRITICAL
- Performance impacts real business operations
- Test migrations on staging first
- Document complex queries
- Migrations are immutable once created

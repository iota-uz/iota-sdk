---
layout: default
title: Data Model
parent: HRM
nav_order: 3
---

# Data Model

## Entity Relationship Diagram

```mermaid
erDiagram
    TENANTS ||--o{ EMPLOYEES : "many"
    TENANTS ||--o{ POSITIONS : "many"
    UPLOADS ||--o{ EMPLOYEES : "zero or one"
    CURRENCIES ||--o{ EMPLOYEES : "zero or one"
    PASSPORTS ||--o{ EMPLOYEE_META : "zero or one"
    EMPLOYEES ||--o{ EMPLOYEE_META : "one"
    EMPLOYEES ||--o{ EMPLOYEE_POSITIONS : "many"
    POSITIONS ||--o{ EMPLOYEE_POSITIONS : "many"

    TENANTS {
        uuid id PK
        string name UK
    }

    EMPLOYEES {
        bigint id PK
        uuid tenant_id FK
        string first_name
        string last_name
        string middle_name
        string email UK
        string phone UK
        decimal salary
        string salary_currency_id FK
        decimal hourly_rate
        float8 coefficient
        bigint avatar_id FK
        timestamp created_at
        timestamp updated_at
    }

    EMPLOYEE_META {
        bigint employee_id PK_FK
        string primary_language
        string secondary_language
        string tin
        string pin
        text notes
        date birth_date
        date hire_date
        date resignation_date
    }

    POSITIONS {
        bigint id PK
        uuid tenant_id FK
        string name UK
        text description
        timestamp created_at
        timestamp updated_at
    }

    EMPLOYEE_POSITIONS {
        bigint employee_id FK
        bigint position_id FK
    }

    UPLOADS {
        bigint id PK
    }

    CURRENCIES {
        varchar code PK
    }

    PASSPORTS {
        bigint id PK
    }
```

## Database Schema

### Employees Table

**Purpose**: Store employee core information and basic compensation

```sql
CREATE TABLE employees (
    id serial8 PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    first_name varchar(255) NOT NULL,
    last_name varchar(255) NOT NULL,
    middle_name varchar(255),
    email varchar(255) NOT NULL,
    phone varchar(255),
    salary decimal(9,2) NOT NULL,
    salary_currency_id varchar(3) REFERENCES currencies(code) ON DELETE SET NULL,
    hourly_rate decimal(9,2) NOT NULL,
    coefficient float8 NOT NULL,
    avatar_id bigint REFERENCES uploads(id) ON DELETE SET NULL,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    UNIQUE(tenant_id, email),
    UNIQUE(tenant_id, phone)
);

CREATE INDEX employees_tenant_id_idx ON employees(tenant_id);
CREATE INDEX employees_email_idx ON employees(email);
CREATE INDEX employees_phone_idx ON employees(phone);
CREATE INDEX employees_first_name_idx ON employees(first_name);
CREATE INDEX employees_last_name_idx ON employees(last_name);
```

**Columns**:
- `id` (SERIAL8): Unique employee identifier
  - Auto-incrementing 64-bit integer
  - Immutable after creation
  - Referenced by foreign keys

- `tenant_id` (UUID): Multi-tenant isolation
  - Required (NOT NULL)
  - Foreign key to `tenants` table
  - Cascade delete: Removing tenant removes employees

- `first_name` (VARCHAR 255): Employee's first name
  - Required
  - Indexed for search performance

- `last_name` (VARCHAR 255): Employee's last name
  - Required
  - Indexed for search performance

- `middle_name` (VARCHAR 255): Optional middle name
  - Can be NULL
  - Supports various naming conventions

- `email` (VARCHAR 255): Contact email
  - Required
  - Indexed for fast lookups
  - Unique constraint with tenant_id

- `phone` (VARCHAR 255): Contact phone
  - Optional (can be NULL)
  - Indexed for lookups
  - Unique constraint with tenant_id

- `salary` (DECIMAL 9,2): Monthly/periodic compensation
  - Required (NOT NULL)
  - Precision: up to 9 digits, 2 decimals
  - In organizational currency

- `salary_currency_id` (VARCHAR 3): Currency code
  - Optional (can be NULL)
  - Foreign key to `currencies` table
  - SET NULL if currency deleted

- `hourly_rate` (DECIMAL 9,2): Hourly compensation
  - Required
  - For hourly employees
  - In organizational currency

- `coefficient` (FLOAT8): Compensation calculation coefficient
  - Required
  - Used for flexible compensation models
  - Example: 1.0 = base, 1.5 = 150%

- `avatar_id` (BIGINT): Reference to employee photo
  - Optional (can be NULL)
  - Foreign key to `uploads` table
  - SET NULL if photo deleted

- `created_at` / `updated_at` (TIMESTAMPTZ): Temporal tracking
  - Automatic timestamps
  - Support audit trails

**Constraints**:
- Primary Key: `id`
- Foreign Keys: `tenant_id`, `salary_currency_id`, `avatar_id`
- Unique: `(tenant_id, email)`, `(tenant_id, phone)`
- Indexes: `tenant_id`, `email`, `phone`, `first_name`, `last_name`

### Employee Meta Table

**Purpose**: Store extended employee information (optional/sensitive data)

```sql
CREATE TABLE employee_meta (
    employee_id bigint PRIMARY KEY REFERENCES employees(id) ON DELETE CASCADE,
    primary_language varchar(10),
    secondary_language varchar(10),
    tin varchar(50),
    pin varchar(50),
    notes text,
    birth_date date,
    hire_date date,
    resignation_date date
);
```

**Columns**:
- `employee_id` (BIGINT): Primary key and FK to employees
  - One-to-one relationship with employee
  - Cascade delete: Removing employee removes meta

- `primary_language` (VARCHAR 10): Primary communication language
  - Optional
  - Language code (e.g., "uz", "ru", "en")
  - NULL if not specified

- `secondary_language` (VARCHAR 10): Secondary language
  - Optional
  - For bilingual/multilingual support

- `tin` (VARCHAR 50): Tax Identification Number
  - Optional (nullable)
  - Compliance and payroll requirement
  - May vary by country

- `pin` (VARCHAR 50): Personal Identification Number
  - Optional (nullable)
  - Document/passport identifier
  - Country-specific format

- `notes` (TEXT): Additional employee information
  - Optional
  - Free-form notes
  - Unlimited length

- `birth_date` (DATE): Date of birth
  - Optional
  - Used for age calculation, retirement planning
  - Date only (no time component)

- `hire_date` (DATE): Employment start date
  - Optional (usually set)
  - Date of hire
  - Date only

- `resignation_date` (DATE): Employment end date
  - Optional (NULL for active employees)
  - When resignation_date is set, employee is considered resigned
  - Date only

**Design Rationale**:
- Separated from main employees table to reduce join overhead for common queries
- Allows selective NULL values without wasting space
- Sensitive data (TIN, PIN) kept separate for access control
- Optional fields don't impact main employee queries

### Positions Table

**Purpose**: Define organizational positions/roles

```sql
CREATE TABLE positions (
    id serial8 PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name varchar(255) NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    UNIQUE(tenant_id, name)
);

CREATE INDEX positions_tenant_id_idx ON positions(tenant_id);
```

**Columns**:
- `id` (SERIAL8): Unique position identifier
  - Auto-incrementing 64-bit integer

- `tenant_id` (UUID): Multi-tenant isolation
  - Required
  - Foreign key to `tenants`
  - Cascade delete

- `name` (VARCHAR 255): Position title
  - Required
  - Unique constraint with tenant_id
  - Examples: "Software Engineer", "Sales Manager"

- `description` (TEXT): Position details
  - Optional
  - Responsibilities, requirements, etc.

- `created_at` / `updated_at` (TIMESTAMPTZ): Temporal tracking
  - Automatic timestamps

**Constraints**:
- Primary Key: `id`
- Foreign Key: `tenant_id`
- Unique: `(tenant_id, name)`
- Indexes: `tenant_id`

### Employee Positions Assignment Table

**Purpose**: Track which positions employees hold (many-to-many)

```sql
CREATE TABLE employee_positions (
    employee_id bigint REFERENCES employees(id) ON DELETE CASCADE,
    position_id bigint REFERENCES positions(id) ON DELETE CASCADE,
    PRIMARY KEY(employee_id, position_id)
);
```

**Columns**:
- `employee_id` (BIGINT): Employee reference
  - Foreign key to `employees` table
  - Part of composite primary key
  - Cascade delete

- `position_id` (BIGINT): Position reference
  - Foreign key to `positions` table
  - Part of composite primary key
  - Cascade delete

**Constraints**:
- Primary Key: `(employee_id, position_id)` - Prevents duplicate assignments
- Foreign Keys: Both cascade delete
- No additional indexes (composite key indexed automatically)

**Design Pattern**:
- Many-to-many junction table
- Supports employees with multiple positions
- Simple boolean relationship (either assigned or not)
- No historical tracking of position changes (current assignments only)

## Data Relationships

### Multi-Tenant Isolation

```
Tenant A
  ├── Employees: 150
  │   └── Unique email/phone per tenant
  └── Positions: 25
      └── Unique names per tenant

Tenant B
  ├── Employees: 300
  │   └── Can have email "john@example.com" (different tenant)
  └── Positions: 40
```

### Employee Lifecycle

```
CREATE EMPLOYEE
  ├── Insert into employees (id auto-generated)
  ├── Optional: Insert into employee_meta (languages, TIN, PIN, hire_date)
  └── Optional: Insert into employee_positions (assign to positions)

ACTIVE EMPLOYEE
  ├── No resignation_date in employee_meta
  ├── Can be queried as "active"
  └── Can update positions

RESIGNED EMPLOYEE
  ├── resignation_date is set in employee_meta
  ├── Still queryable (historical record)
  └── Typically not for new assignments

ARCHIVED EMPLOYEE
  ├── May be soft-deleted (flag in meta or positions set to null)
  └── Historical data preserved for reporting
```

## Data Type Decisions

### SERIAL8 vs. UUID for Employee IDs

**Decision**: Use SERIAL8 (auto-incrementing 64-bit integer)

**Rationale**:
- Simpler primary key (8 bytes vs 16 bytes)
- Faster joins and indexes
- Easier to work with in UI (shorter IDs)
- Suitable for per-tenant uniqueness (tenant_id + id)

### DECIMAL for Salary Amounts

**Decision**: DECIMAL(9,2) for salary values

**Rationale**:
- Exact decimal arithmetic (no floating-point errors)
- Supports up to 9,999,999.99 per currency unit
- Suitable for payroll and compensation calculations
- SQL SUM/AVG aggregate functions work correctly

### DATE vs. TIMESTAMPTZ

**Decision**: DATE for hire/resignation dates, TIMESTAMPTZ for audit fields

**Rationale**:
- Employment dates are calendar dates (no time component)
- Audit timestamps need timezone awareness
- Simpler queries for date-based reports

### NULL vs. Separate Table for Meta

**Decision**: Separate `employee_meta` table for optional/sensitive fields

**Rationale**:
- Keeps main table lean for common queries
- Optional fields don't waste space
- Sensitive data (TIN, PIN) can have separate access control
- Easier to add new optional fields later
- Reduces NULL values in common queries

## Indexing Strategy

### Primary Indexes (Essential)

```sql
-- Tenant isolation (every query filters by tenant_id)
CREATE INDEX employees_tenant_id_idx ON employees(tenant_id);
CREATE INDEX positions_tenant_id_idx ON positions(tenant_id);

-- Unique constraint indexes (automatically created)
-- employees (tenant_id, email)
-- employees (tenant_id, phone)
-- positions (tenant_id, name)
```

### Secondary Indexes (Performance)

```sql
-- Search and filtering
CREATE INDEX employees_email_idx ON employees(email);
CREATE INDEX employees_phone_idx ON employees(phone);
CREATE INDEX employees_first_name_idx ON employees(first_name);
CREATE INDEX employees_last_name_idx ON employees(last_name);

-- Position assignments
CREATE INDEX employee_positions_position_id_idx ON employee_positions(position_id);
```

## Storage Estimates

### Typical Record Sizes

**Employee**: ~200 bytes
- id: 8 bytes
- tenant_id: 16 bytes
- Names: ~100 bytes
- Email/phone: ~50 bytes
- Salary/rate: 16 bytes
- Timestamps: 16 bytes

**Employee Meta**: ~150 bytes
- Languages: 20 bytes
- Tax IDs: 100 bytes
- Dates: 24 bytes
- Notes: variable

**Position**: ~150 bytes
- id: 8 bytes
- tenant_id: 16 bytes
- Name: ~50 bytes
- Description: ~75 bytes
- Timestamps: 16 bytes

### Growth Projections

| Metric | 100 Orgs | 1,000 Orgs | 10,000 Orgs |
|--------|----------|-----------|------------|
| Employees | 10,000 | 100,000 | 1,000,000 |
| Positions | 500 | 5,000 | 50,000 |
| Assignments | 15,000 | 150,000 | 1,500,000 |
| **Total Size** | **~8 MB** | **~80 MB** | **~800 MB** |

With indexes: ~2-3x the base size (~24 MB - 2.4 GB)

## Access Patterns

### Query Patterns

**Frequent Queries**:
1. List employees for tenant (paginated)
   - Uses: `employees_tenant_id_idx`
   - Filter: `tenant_id = ?`
   - Order: `last_name, first_name`

2. Find employee by email
   - Uses: `employees_email_idx`
   - Filter: `email = ?`
   - Join: Tenant validation

3. Search by name
   - Uses: `employees_first_name_idx`, `employees_last_name_idx`
   - Filter: `first_name LIKE ?` OR `last_name LIKE ?`

4. Find active employees
   - Uses: `employees_tenant_id_idx`
   - Join: `employee_meta`
   - Filter: `resignation_date IS NULL`

5. List employees by position
   - Uses: `employee_positions_position_id_idx`
   - Join: `employees`, `positions`
   - Filter: `position_id = ?`

## Migration Reference

See `migrations/changes-1740741698.sql` for schema creation:
- Creates employees, positions tables
- Establishes foreign keys and constraints
- Creates performance indexes
- Includes up/down migration pairs

## Compliance & Privacy

### Data Retention

- Employee records maintained indefinitely
- Resignation marks employee as inactive
- Historical data preserved for compliance
- Consider GDPR right-to-be-forgotten requirements

### Sensitive Data

- TIN, PIN in separate table
- Can be accessed with permission check
- Consider encryption for sensitive fields
- Access logging recommended

### Multi-Tenancy

- Complete data isolation by `tenant_id`
- Email/phone unique per tenant (not globally)
- All queries filter by tenant_id
- No cross-tenant data access possible

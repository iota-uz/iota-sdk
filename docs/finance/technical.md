---
layout: default
title: Technical Architecture
parent: Finance Module
nav_order: 2
description: "Technical implementation details, service patterns, and API contracts"
---

# Technical Architecture: Finance Module

## Layer Separation

The Finance Module follows Domain-Driven Design with financial domain expertise:

```
┌─────────────────────────────────────────────────────────┐
│              PRESENTATION LAYER                          │
│  Controllers → ViewModels → Templates                    │
│  /finance/overview, /payments, /expenses, /accounts      │
└──────────────────┬──────────────────────────────────────┘
                   │
┌──────────────────▼──────────────────────────────────────┐
│               SERVICE LAYER                              │
│  PaymentService, ExpenseService, AccountService, etc.    │
│  - Financial business logic                              │
│  - Transaction coordination                              │
│  - Balance calculation & updates                         │
│  - Report generation                                     │
│  - Permission validation                                │
└──────────────────┬──────────────────────────────────────┘
                   │
┌──────────────────▼──────────────────────────────────────┐
│              DOMAIN LAYER                                │
│  Payment, Expense, Debt, MoneyAccount Aggregates         │
│  - Financial business rules                              │
│  - Value objects (Transaction, Counterparty)             │
│  - Repository interfaces                                 │
│  - Domain events (PaymentCreated, ExpenseRecorded)       │
└──────────────────┬──────────────────────────────────────┘
                   │
┌──────────────────▼──────────────────────────────────────┐
│          INFRASTRUCTURE LAYER                            │
│  PostgreSQL Repositories, Query Repositories             │
│  - Database access with transactional support            │
│  - Advanced queries for reporting                        │
│  - Transaction handling via composables.UseTx()         │
│  - Tenant isolation via composables.UseTenantID()       │
└──────────────────┬──────────────────────────────────────┘
                   │
              PostgreSQL DB
```

## Directory Structure

```
modules/finance/
├── domain/
│   ├── aggregates/
│   │   ├── payment/
│   │   │   ├── payment.go                # Payment aggregate
│   │   │   ├── payment_impl.go           # Private implementation
│   │   │   ├── payment_repository.go     # Repository interface
│   │   │   ├── payment_events.go         # Domain events
│   │   │   └── payment_dto.go            # Data transfer objects
│   │   ├── expense/
│   │   │   ├── expense.go
│   │   │   ├── expense_repository.go
│   │   │   └── expense_events.go
│   │   ├── debt/
│   │   │   ├── debt.go
│   │   │   ├── debt_impl.go
│   │   │   ├── debt_repository.go
│   │   │   └── debt_events.go
│   │   ├── money_account/
│   │   │   ├── account.go
│   │   │   ├── account_interface.go
│   │   │   ├── account_repository.go
│   │   │   └── account_events.go
│   │   ├── payment_category/
│   │   │   ├── payment_category.go
│   │   │   ├── payment_category_repository.go
│   │   │   └── payment_category_events.go
│   │   └── expense_category/
│   │       ├── expense_category.go
│   │       ├── expense_category_repository.go
│   │       └── expense_category_events.go
│   ├── entities/
│   │   ├── transaction/
│   │   │   ├── transaction.go            # Transaction entity
│   │   │   ├── transaction_interface.go
│   │   │   ├── transaction_repository.go
│   │   │   ├── transaction_errors.go
│   │   │   └── value_objects.go          # TransactionType, etc.
│   │   ├── counterparty/
│   │   │   ├── counterparty.go
│   │   │   ├── counterparty_implementation.go
│   │   │   ├── counterparty_repository.go
│   │   │   └── value_objects.go
│   │   └── inventory/
│   │       ├── inventory.go
│   │       ├── inventory_interface.go
│   │       └── inventory_repository.go
│   └── value_objects/
│       ├── income_statement.go           # P&L statement
│       ├── cashflow_statement.go         # Cash flow report
│       └── (other financial calculations)
│
├── infrastructure/
│   ├── persistence/
│   │   ├── schema/
│   │   │   └── finance-schema.sql        # Migrations
│   │   ├── payment_repository.go         # Payment repository impl
│   │   ├── expense_repository.go         # Expense repository impl
│   │   ├── money_account_repository.go   # Account repository impl
│   │   ├── debt_repository.go            # Debt repository impl
│   │   ├── counterparty_repository.go    # Counterparty repository impl
│   │   ├── transaction_repository.go     # Transaction repository impl
│   │   ├── finance_mappers.go            # DB → Domain mapping
│   │   └── models/models.go              # Database models
│   └── query/
│       ├── financial_reports_query_repository.go  # Report queries
│       ├── transaction_query_repository.go        # Transaction queries
│       └── account_statement_query_repository.go  # Account queries
│
├── services/
│   ├── payment_service.go                # Payment processing
│   ├── payment_service_test.go
│   ├── expense_service.go                # Expense management
│   ├── transaction_service.go            # Transaction handling
│   ├── money_account_service.go          # Account management
│   ├── debt_service.go                   # Debt tracking
│   ├── debt_service_test.go
│   ├── counterparty_service.go           # Counterparty management
│   ├── inventory_service.go              # Inventory management
│   ├── financial_report_service.go       # Report generation
│   ├── financial_report_service_integration_test.go
│   ├── payment_category_service.go       # Category management
│   ├── expense_category_service.go
│   └── setup_test.go
│
├── presentation/
│   ├── controllers/
│   │   ├── payment_controller.go
│   │   ├── expense_controller.go
│   │   ├── debt_controller.go
│   │   ├── money_account_controller.go
│   │   ├── financial_overview_controller.go
│   │   ├── financial_report_controller.go
│   │   ├── counterparties_controller.go
│   │   ├── inventory_controller.go
│   │   ├── cashflow_controller.go
│   │   └── (more controllers with _test.go files)
│   ├── templates/pages/
│   │   ├── payments/
│   │   ├── expenses/
│   │   ├── debts/
│   │   ├── financial_overview/
│   │   ├── moneyaccounts/
│   │   └── (more template directories)
│   └── locales/
│       ├── en.json
│       ├── ru.json
│       └── uz.json
│
└── permissions/
    └── constants.go                     # Permission definitions
```

## Key Implementation Patterns

### Payment Aggregate

```go
// Payment aggregate interface
type Payment interface {
    ID() uuid.UUID
    Account() MoneyAccount
    Counterparty() Counterparty
    Amount() int64
    Category() PaymentCategory
    TransactionID() uuid.UUID
    CreatedAt() time.Time

    SetCategory(category PaymentCategory) Payment
    // More setter methods (immutable pattern)
}

// Private struct implementation
type payment struct {
    id            uuid.UUID
    account       MoneyAccount
    counterparty  Counterparty
    amount        int64
    category      PaymentCategory
    transactionID uuid.UUID
}

// Factory method with options
func New(opts ...Option) Payment {
    p := &payment{}
    for _, opt := range opts {
        opt(p)
    }
    return p
}

func WithAccount(acc MoneyAccount) Option {
    return func(p *payment) {
        p.account = acc
    }
}
```

### Service with Transaction Coordination

```go
// PaymentService with balance updates
type PaymentService struct {
    repo           payment.Repository
    publisher      eventbus.EventBus
    accountService *MoneyAccountService
    uploadRepo     upload.Repository
}

// Create payment with automatic balance update
func (s *PaymentService) Create(
    ctx context.Context,
    entity payment.Payment,
) (payment.Payment, error) {
    if err := composables.CanUser(ctx, permissions.PaymentCreate); err != nil {
        return nil, err
    }

    var createdEntity payment.Payment
    err := composables.InTx(ctx, func(txCtx context.Context) error {
        var err error
        // Create payment (creates transaction internally)
        createdEntity, err = s.repo.Create(txCtx, entity)
        if err != nil {
            return err
        }

        // Update account balance in same transaction
        return s.accountService.RecalculateBalance(
            txCtx,
            createdEntity.Account().ID(),
        )
    })

    if err == nil {
        s.publisher.Publish(payment.NewCreatedEvent(ctx, createdEntity, entity))
    }
    return createdEntity, err
}
```

### Multi-currency Transaction Handling

```go
// Transaction with exchange rate support
type Transaction interface {
    ID() uuid.UUID
    Amount() int64                    // In origin currency
    DestinationAmount() int64         // In destination currency (optional)
    ExchangeRate() *decimal.Decimal   // Exchange rate used
    TransactionType() TransactionType // income, expense, transfer, exchange
    TransactionDate() time.Time
    // ...
}

// Service handles multi-currency calculations
func (s *TransactionService) CreateExchangeTransaction(
    ctx context.Context,
    originAccount MoneyAccount,
    destAccount MoneyAccount,
    amount int64,
    exchangeRate float64,
) (Transaction, error) {
    // Calculate destination amount
    destAmount := decimal.NewFromInt(amount).
        Mul(decimal.NewFromFloat(exchangeRate)).
        IntPart()

    // Create transaction with exchange rate tracking
    return s.repo.Create(ctx, Transaction{
        Type:              "exchange",
        Amount:            amount,
        DestinationAmount: destAmount,
        ExchangeRate:      exchangeRate,
    })
}
```

### Report Generation

```go
// Financial report service
type FinancialReportService struct {
    queryRepo query.FinancialReportsQueryRepository
    publisher eventbus.EventBus
}

// Generate income statement for period
func (s *FinancialReportService) GenerateIncomeStatement(
    ctx context.Context,
    startDate, endDate time.Time,
) (IncomeStatement, error) {
    if err := composables.CanUser(ctx, permissions.ReportRead); err != nil {
        return nil, err
    }

    // Query transactions for period
    transactions, err := s.queryRepo.GetTransactionsByPeriod(
        ctx, startDate, endDate,
    )
    if err != nil {
        return nil, err
    }

    // Calculate financial metrics
    revenue := s.sumTransactionsByType(transactions, "income")
    expenses := s.sumTransactionsByType(transactions, "expense")
    netIncome := revenue - expenses

    return NewIncomeStatement(revenue, expenses, netIncome), nil
}
```

### Repository with Tenant Isolation

```go
// Payment repository with multi-tenancy
func (r *paymentRepository) GetByID(
    ctx context.Context,
    id uuid.UUID,
) (payment.Payment, error) {
    const op = "payment.Repository.GetByID"

    // Get tenant from context
    tenantID, err := composables.UseTenantID(ctx)
    if err != nil {
        return nil, serrors.E(op, err)
    }

    // Query with tenant isolation
    row := composables.UseTx(ctx, r.db).QueryRow(
        `SELECT id, tenant_id, account_id, counterparty_id, amount
         FROM payments
         WHERE id = $1 AND tenant_id = $2`,
        id, tenantID,
    )

    var p payment.Payment
    if err := scanPayment(row, &p); err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, serrors.E(op, serrors.KindNotFound)
        }
        return nil, serrors.E(op, err)
    }

    return p, nil
}
```

## Permission Matrix

### Payment Permissions

| Permission | Description |
|------------|-------------|
| `payments:create:all` | Create payments for any account |
| `payments:read:all` | View all payments |
| `payments:read:own` | View payments for own accounts |
| `payments:update:all` | Modify any payment |
| `payments:delete:all` | Delete any payment |

### Expense Permissions

| Permission | Description |
|------------|-------------|
| `expenses:create:all` | Record any expense |
| `expenses:read:all` | View all expenses |
| `expenses:read:own` | View own expenses |
| `expenses:update:all` | Modify any expense |
| `expenses:delete:all` | Delete any expense |

### Account Permissions

| Permission | Description |
|------------|-------------|
| `accounts:create:all` | Create accounts |
| `accounts:read:all` | View all accounts |
| `accounts:update:all` | Modify accounts |
| `accounts:delete:all` | Delete accounts |

### Debt Permissions

| Permission | Description |
|------------|-------------|
| `debts:create:all` | Record debts |
| `debts:read:all` | View all debts |
| `debts:update:all` | Modify debts |
| `debts:delete:all` | Delete debts |

### Report Permissions

| Permission | Description |
|------------|-------------|
| `reports:read:all` | View financial reports |
| `reports:create:all` | Generate custom reports |

## API Contracts

### Payment Endpoints

```
GET    /finance/payments              # List payments (paginated)
GET    /finance/payments/:id         # Get payment details
POST   /finance/payments             # Create payment
PUT    /finance/payments/:id         # Update payment
DELETE /finance/payments/:id         # Delete payment
```

### Expense Endpoints

```
GET    /finance/expenses             # List expenses
POST   /finance/expenses             # Record expense
GET    /finance/expenses/:id        # Get expense details
PUT    /finance/expenses/:id        # Update expense
DELETE /finance/expenses/:id        # Delete expense
```

### Account Endpoints

```
GET    /finance/accounts            # List accounts
GET    /finance/accounts/:id        # Get account details
POST   /finance/accounts            # Create account
PUT    /finance/accounts/:id        # Update account
GET    /finance/accounts/:id/transactions  # Account transactions
```

### Debt Endpoints

```
GET    /finance/debts               # List debts
POST   /finance/debts               # Create debt
GET    /finance/debts/:id           # Get debt details
PUT    /finance/debts/:id           # Update debt
POST   /finance/debts/:id/settle    # Settle debt
```

### Report Endpoints

```
GET    /finance/reports/income-statement          # P&L report
GET    /finance/reports/cashflow                  # Cash flow report
GET    /finance/reports/account-statement/:id     # Account statement
```

## Error Handling

All services use `serrors` package:

```go
const op serrors.Op = "PaymentService.Create"

// Validation errors
if invalidAmount {
    return nil, serrors.E(op, serrors.KindValidation, "amount must be positive")
}

// Business rule errors
if insufficientBalance {
    return nil, serrors.E(op, "insufficient account balance")
}

// Not found errors
if payment not found {
    return nil, serrors.E(op, serrors.KindNotFound)
}
```

## Multi-tenancy Implementation

### Data Isolation

```sql
-- All finance tables include tenant_id
CREATE TABLE payments (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    -- ...
);

-- Queries always filter by tenant
SELECT * FROM payments WHERE tenant_id = $1 AND id = $2
```

### Currency Configuration

- Currencies are global (not tenant-specific)
- Tenant can select which currencies to use
- Each account has one base currency
- Multi-currency transactions tracked with exchange rates

## Performance Considerations

### Database Indexes

```sql
-- Account queries
CREATE INDEX money_accounts_tenant_id_idx
  ON money_accounts(tenant_id);

-- Transaction lookups
CREATE INDEX transactions_tenant_id_idx
  ON transactions(tenant_id);
CREATE INDEX transactions_account_id_idx
  ON transactions(origin_account_id, destination_account_id);

-- Payment tracking
CREATE INDEX payments_counterparty_id_idx
  ON payments(counterparty_id);
CREATE INDEX payments_account_id_idx
  ON payments(account_id);

-- Debt aging
CREATE INDEX debts_due_date_idx
  ON debts(due_date);
CREATE INDEX debts_status_idx
  ON debts(status);
```

### Query Optimization

1. **Batch Loading**: Load accounts with balances in single query
2. **Report Caching**: Cache generated reports with invalidation on new transactions
3. **Pagination**: All listing endpoints paginated
4. **Index Usage**: Foreign key indexes for relationship queries

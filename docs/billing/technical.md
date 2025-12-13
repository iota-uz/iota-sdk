---
layout: default
title: Technical Architecture
parent: Billing
nav_order: 2
---

# Technical Architecture

## Module Structure

```
modules/billing/
├── domain/
│   ├── aggregates/
│   │   ├── billing/
│   │   │   ├── billing.go
│   │   │   ├── billing_impl.go
│   │   │   ├── billing_repository.go
│   │   │   ├── billing_events.go
│   │   │   ├── gateway.go               # Payment gateway enum
│   │   │   ├── status.go                # Transaction status enum
│   │   │   └── provider.go              # Provider interface
│   │   └── details/
│   │       ├── details.go               # Interface definitions
│   │       ├── click_details_impl.go
│   │       ├── payme_details_impl.go
│   │       ├── octo_details_impl.go
│   │       ├── stripe_details_impl.go
│   │       ├── cash_details_impl.go
│   │       └── integrator_details_impl.go
├── infrastructure/
│   ├── persistence/
│   │   ├── billing_repository.go
│   │   ├── billing_mappers.go
│   │   ├── models/
│   │   │   └── models.go
│   │   └── queries/
│   ├── providers/
│   │   ├── provider.go                  # Base provider interface
│   │   ├── stripe_provider.go           # Stripe integration
│   │   ├── click_provider.go            # Click integration
│   │   ├── payme_provider.go            # Payme integration
│   │   ├── octo_provider.go             # Octo integration
│   │   └── integrator_provider.go
│   └── callbacks/
│       ├── stripe_callback.go
│       ├── click_callback.go
│       ├── payme_callback.go
│       └── octo_callback.go
├── services/
│   ├── billing_service.go
│   ├── billing_service_test.go
│   └── setup_test.go
├── presentation/
│   ├── controllers/
│   │   ├── billing_controller.go        # Transaction queries
│   │   ├── webhook_controller.go        # Webhook handlers
│   │   └── callback_controller.go       # Provider callbacks
│   └── locales/
│       ├── en.toml
│       ├── ru.toml
│       └── uz.toml
├── permissions/
│   └── constants.go
├── module.go
└── links.go
```

## Domain Layer

### Transaction Aggregate

**Interface** (`billing.go`):
```go
type Transaction interface {
    ID() uuid.UUID
    TenantID() uuid.UUID

    Amount() *money.Money      // Quantity + Currency
    Quantity() float64         // Amount in cents
    Currency() Currency         // ISO 4217 code

    Status() Status             // Payment status
    SetStatus(Status) Transaction

    Gateway() Gateway           // Payment processor
    Details() details.Details   // Gateway-specific data
    SetDetails(details.Details) Transaction

    Events() []interface{}      // Domain events
    ClearEvents()

    CreatedAt() time.Time
    UpdatedAt() time.Time
}
```

**Key Principles**:
- **Immutable Creation**: ID, TenantID, Gateway, Currency cannot change
- **Status Management**: Controlled state transitions
- **Event Publishing**: Tracks all domain events
- **Details Handling**: Supports multiple gateway types

### Payment Gateway Types

```go
type Gateway string

const (
    GatewayStripe     Gateway = "stripe"
    GatewayClick      Gateway = "click"
    GatewayPayme      Gateway = "payme"
    GatewayOcto       Gateway = "octo"
    GatewayCash       Gateway = "cash"
    GatewayIntegrator Gateway = "integrator"
)
```

### Transaction Status

```go
type Status string

const (
    Pending            Status = "pending"
    Completed          Status = "completed"
    Canceled           Status = "canceled"
    Refunded           Status = "refunded"
    PartiallyRefunded  Status = "partially_refunded"
)
```

### Provider Interface

```go
type Provider interface {
    Gateway() Gateway

    Create(ctx context.Context, transaction Transaction) (Transaction, error)
    Cancel(ctx context.Context, transaction Transaction) (Transaction, error)
    Refund(ctx context.Context, transaction Transaction, amount float64) (Transaction, error)
}
```

Implementations:
- `StripeProvider`: Stripe API integration
- `ClickProvider`: Click UZ API integration
- `PaymeProvider`: Payme API integration
- `OctoProvider`: Octo API integration
- No provider for Cash/Integrator (local handling)

### Domain Events

```go
type CreatedEvent struct {
    Result Transaction
    // Metadata
}

type UpdatedEvent struct {
    Result Transaction
    // Metadata
}

type RefundedEvent struct {
    Original       Transaction
    RefundAmount   float64
    RefundedResult Transaction
}

type DeletedEvent struct {
    Result Transaction
}
```

## Service Layer

### BillingService

**Responsibilities**:
- Transaction CRUD
- Provider coordination
- Event publishing
- Callback handling

**Transaction Handling**:
```go
func (s *BillingService) Create(ctx context.Context, cmd *CreateTransactionCommand) (Transaction, error) {
    entity := billing.New(...)

    provider := s.providers[entity.Gateway()]

    var createdTransaction Transaction
    err := composables.InTx(ctx, func(txCtx context.Context) error {
        // If provider exists, use it (Stripe, Click, Payme, Octo)
        if provider != nil {
            providedTransaction, err := provider.Create(txCtx, entity)
            if err != nil {
                return err
            }
            createdTransaction, err = s.repo.Save(txCtx, providedTransaction)
            return err
        }

        // For Cash/Integrator, save directly
        createdTransaction, err = s.repo.Save(txCtx, entity)
        return err
    })

    if err != nil {
        return nil, err
    }

    event, _ := billing.NewCreatedEvent(ctx, createdTransaction)
    s.publisher.Publish(event)
    return createdTransaction, nil
}
```

**Refund Handling**:
```go
func (s *BillingService) Refund(ctx context.Context, cmd *RefundTransactionCommand) (Transaction, error) {
    entity, err := s.repo.GetByID(ctx, cmd.TransactionID)
    if err != nil {
        return nil, err
    }

    provider := s.providers[entity.Gateway()]

    var updatedTransaction Transaction
    err = composables.InTx(ctx, func(txCtx context.Context) error {
        if provider != nil {
            providedTransaction, err := provider.Refund(txCtx, entity, cmd.Quantity)
            if err != nil {
                return err
            }
            updatedTransaction, err = s.repo.Save(txCtx, providedTransaction)
            return err
        }

        // For Cash/Integrator, just update status
        if cmd.Quantity >= entity.Amount().Quantity() {
            entity = entity.SetStatus(billing.Refunded)
        } else {
            entity = entity.SetStatus(billing.PartiallyRefunded)
        }
        updatedTransaction, err = s.repo.Save(txCtx, entity)
        return err
    })

    if err != nil {
        return nil, err
    }

    event, _ := billing.NewUpdatedEvent(ctx, updatedTransaction)
    s.publisher.Publish(event)
    return updatedTransaction, nil
}
```

## Repository Layer

### Transaction Repository Interface

```go
type Repository interface {
    Count(ctx context.Context, params *FindParams) (int64, error)
    GetByID(ctx context.Context, id uuid.UUID) (Transaction, error)
    GetByDetailsFields(ctx context.Context, gateway Gateway, filters []DetailsFieldFilter) ([]Transaction, error)
    GetPaginated(ctx context.Context, params *FindParams) ([]Transaction, error)
    Save(ctx context.Context, t Transaction) (Transaction, error)
    Delete(ctx context.Context, id uuid.UUID) error
}

type DetailsFieldFilter struct {
    Field string      // e.g., "click.merchant_id"
    Value interface{} // Value to match
}
```

### Repository Implementation

**Key Implementation Details**:

1. **Tenant Isolation**:
   ```go
   tenantID := composables.UseTenantID(ctx)
   const getByIDSQL = `
       SELECT id, tenant_id, status, quantity, currency, gateway, details,
              created_at, updated_at
       FROM billing_transactions
       WHERE id = $1 AND tenant_id = $2
   `
   ```

2. **JSON Details Storage**:
   ```go
   var detailsJSON json.RawMessage
   err := row.Scan(&model.ID, &model.TenantID, ..., &detailsJSON)
   model.Details = detailsJSON
   ```

3. **Dynamic Queries for Details Fields**:
   Uses `pkg/repo` for dynamic filtering on JSON fields:
   ```go
   // Query transactions by Click merchant ID
   WHERE gateway = 'click' AND details->>'merchant_id' = $1
   ```

4. **Error Wrapping**:
   ```go
   const op serrors.Op = "BillingRepository.GetByID"
   if err != nil {
       return nil, serrors.E(op, err)
   }
   ```

## Gateway Integration Patterns

### Provider Base Implementation

```go
type BaseProvider struct {
    gateway Gateway
    client  *http.Client
}

func (p *BaseProvider) Gateway() Gateway {
    return p.gateway
}
```

### Stripe Provider Implementation

```go
type StripeProvider struct {
    BaseProvider
    apiKey string
}

func (p *StripeProvider) Create(ctx context.Context, transaction Transaction) (Transaction, error) {
    // 1. Create Stripe session
    sessionReq := &stripe.SessionParams{
        BillingReason: transaction.Details().(details.StripeDetails).BillingReason(),
        // ... other fields
    }

    session, err := p.createSession(ctx, sessionReq)
    if err != nil {
        return nil, err
    }

    // 2. Update transaction with session details
    stripeDetails := transaction.Details().(details.StripeDetails)
    stripeDetails = stripeDetails.
        SetSessionID(session.ID).
        SetURL(session.URL)

    return transaction.SetDetails(stripeDetails), nil
}
```

### Click Provider Implementation

```go
type ClickProvider struct {
    BaseProvider
    merchantID string
    serviceID  int64
}

func (p *ClickProvider) Create(ctx context.Context, transaction Transaction) (Transaction, error) {
    // 1. Call Click Prepare API
    prepareResp, err := p.prepare(ctx, &ClickPrepareRequest{
        MerchantID:   p.merchantID,
        ServiceID:    p.serviceID,
        Amount:       int64(transaction.Quantity()),
        TransID:      uuid.New().String(),
    })
    if err != nil {
        return nil, err
    }

    // 2. Update transaction with prepare result
    clickDetails := transaction.Details().(details.ClickDetails)
    clickDetails = clickDetails.
        SetMerchantPrepareID(prepareResp.PrepareID).
        SetLink(prepareResp.PaymentLink)

    return transaction.SetDetails(clickDetails), nil
}

func (p *ClickProvider) Confirm(ctx context.Context, transaction Transaction) (Transaction, error) {
    // Called after customer pays on Click
    // Updates transaction status to completed
}
```

## Persistence Models

### Transaction Table

```sql
CREATE TABLE billing_transactions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    status varchar(50) NOT NULL,
    quantity float8 NOT NULL,
    currency varchar(3) NOT NULL,
    gateway varchar(50) NOT NULL,
    details jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    CHECK (gateway IN ('click', 'payme', 'octo', 'stripe', 'cash', 'integrator')),
    CHECK (status IN ('pending', 'completed', 'canceled', 'refunded', 'partially_refunded'))
);

CREATE INDEX billing_transactions_tenant_id_idx ON billing_transactions(tenant_id);
CREATE INDEX billing_transactions_status_idx ON billing_transactions(status);
CREATE INDEX billing_transactions_gateway_idx ON billing_transactions(gateway);
CREATE INDEX billing_transactions_created_at_idx ON billing_transactions(created_at);

-- For detailed field searches
CREATE INDEX billing_transactions_details_gin ON billing_transactions USING gin(details);
```

## Database Models

```go
type Transaction struct {
    ID        string
    TenantID  string
    Status    string
    Quantity  float64
    Currency  string
    Gateway   string
    Details   json.RawMessage
    CreatedAt time.Time
    UpdatedAt time.Time
}

type ClickDetails struct {
    ServiceID         int64          `json:"service_id"`
    MerchantID        int64          `json:"merchant_id"`
    MerchantUserID    int64          `json:"merchant_user_id"`
    MerchantTransID   string         `json:"merchant_trans_id"`
    MerchantPrepareID int64          `json:"merchant_prepare_id"`
    MerchantConfirmID int64          `json:"merchant_confirm_id"`
    PayDocId          int64          `json:"pay_doc_id"`
    PaymentID         int64          `json:"payment_id"`
    PaymentStatus     int32          `json:"payment_status"`
    SignTime          string         `json:"sign_time"`
    SignString        string         `json:"sign_string"`
    ErrorCode         int32          `json:"error_code"`
    ErrorNote         string         `json:"error_note"`
    Link              string         `json:"link"`
    Params            map[string]any `json:"params"`
}

type StripeDetails struct {
    Mode              string                  `json:"mode"`
    BillingReason     string                  `json:"billing_reason"`
    SessionID         string                  `json:"session_id"`
    ClientReferenceID string                  `json:"client_reference_id"`
    InvoiceID         string                  `json:"invoice_id"`
    SubscriptionID    string                  `json:"subscription_id"`
    CustomerID        string                  `json:"customer_id"`
    SubscriptionData  *StripeSubscriptionData `json:"subscription_data"`
    Items             []StripeItem            `json:"items"`
    SuccessURL        string                  `json:"success_url"`
    CancelURL         string                  `json:"cancel_url"`
    URL               string                  `json:"url"`
}
```

## Webhook/Callback Handling

### Stripe Webhook Handler

```go
func (c *WebhookController) StripeWebhook(w http.ResponseWriter, r *http.Request) {
    // 1. Validate Stripe signature
    body, _ := ioutil.ReadAll(r.Body)
    valid := stripe.ValidateWebhookSignature(body, r.Header.Get("Stripe-Signature"))
    if !valid {
        w.WriteHeader(http.StatusUnauthorized)
        return
    }

    // 2. Parse event
    event := stripe.Event{}
    json.Unmarshal(body, &event)

    // 3. Handle event type
    switch event.Type {
    case "payment_intent.succeeded":
        // Update transaction status
    case "charge.refunded":
        // Handle refund
    }
}
```

### Click Callback Handler

```go
func (c *WebhookController) ClickCallback(w http.ResponseWriter, r *http.Request) {
    // 1. Parse Click callback
    callback := &ClickCallback{}
    json.NewDecoder(r.Body).Decode(callback)

    // 2. Validate signature
    if !validateClickSignature(callback) {
        return clickError(403)
    }

    // 3. Update transaction
    if callback.Action == "confirm" {
        transaction, _ := c.service.GetByDetailsFields(
            r.Context(),
            billing.GatewayClick,
            []billing.DetailsFieldFilter{
                {Field: "click.merchant_prepare_id", Value: callback.PrepareID},
            },
        )
        c.service.Save(r.Context(), transaction.SetStatus(billing.Completed))
    }
}
```

## Error Handling

All errors use `serrors` package:

```go
const op serrors.Op = "BillingService.Create"

if err != nil {
    return nil, serrors.E(op, err)
}
```

Error types:
- `KindValidation`: Invalid transaction data
- `KindNotFound`: Transaction not found
- `KindPermission`: Unauthorized access
- `KindConflict`: Invalid state transition
- `KindExternal`: Provider API errors

## Testing Strategy

### Service Tests
- Happy path: Create, refund, cancel
- Provider errors: Simulate provider failures
- Transaction states: Verify state transitions
- Event publishing: Verify events published

### Repository Tests
- CRUD operations
- Tenant isolation
- Details field filtering
- Transaction status queries

### Provider Tests
- Mock provider responses
- Error scenarios
- Idempotency checks
- Detail data transformation

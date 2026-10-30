package services_test

import (
	"context"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/stretchr/testify/require"
)

func TestFinanceHistory_PaymentsAndExpenses(t *testing.T) {
	t.Parallel()
	env := setupTest(t, permissions.Permissions...)
	require.NoError(t, corepersistence.NewCurrencyRepository().Create(env.Ctx, currency.USD))
	events, err := composition.Resolve[businessevent.Repository](env.Container)
	require.NoError(t, err)
	f := financeHistoryFixture{
		env:               env,
		accounts:          getAccountService(env),
		counterparties:    itf.GetService[services.CounterpartyService](env),
		expenseCategories: itf.GetService[services.ExpenseCategoryService](env),
		paymentCategories: getPaymentCategoryService(env),
		events:            events,
	}
	payments := getPaymentService(env)
	expenses := itf.GetService[services.ExpenseService](env)
	day := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
	newPayment := func(t *testing.T) payment.Payment {
		t.Helper()
		created, err := payments.Create(env.Ctx, payment.New(money.New(1500, "USD"), f.newPaymentCategory(t),
			payment.WithTenantID(env.TenantID()), payment.WithAccount(f.newAccount(t)),
			payment.WithCounterpartyID(f.newCounterparty(t).ID()),
			payment.WithTransactionDate(day), payment.WithAccountingPeriod(day),
		))
		require.NoError(t, err)
		return created
	}
	newExpense := func(t *testing.T) expense.Expense {
		t.Helper()
		created, err := expenses.Create(env.Ctx, expense.New(money.New(700, "USD"), f.newAccount(t), f.newExpenseCategory(t), day,
			expense.WithTenantID(env.TenantID()), expense.WithAccountingPeriod(day),
		))
		require.NoError(t, err)
		return created
	}
	newUpload := func(t *testing.T) uint {
		t.Helper()
		created, err := corepersistence.NewUploadRepository().Create(env.Ctx, upload.New(
			"hash-"+strconv.FormatInt(time.Now().UnixNano(), 10), "uploads/receipt.pdf", "receipt.pdf", "", 1024,
			mimetype.Lookup("application/pdf"),
		))
		require.NoError(t, err)
		return created.ID()
	}

	tests := []struct {
		name      string
		act       func(t *testing.T, ctx context.Context) []businessevent.Subject
		eventType string
		fields    []string
	}{
		{
			name: "payment created",
			act: func(t *testing.T, ctx context.Context) []businessevent.Subject {
				t.Helper()
				created, err := payments.Create(ctx, payment.New(money.New(2500, "USD"), f.newPaymentCategory(t),
					payment.WithTenantID(env.TenantID()), payment.WithAccount(f.newAccount(t)),
					payment.WithCounterpartyID(f.newCounterparty(t).ID()),
					payment.WithTransactionDate(day), payment.WithAccountingPeriod(day), payment.WithComment("Invoice 7"),
				))
				require.NoError(t, err)
				return []businessevent.Subject{{Resource: services.PaymentResource, ID: created.ID().String()}}
			},
			eventType: services.PaymentCreated,
			fields:    []string{"account_id", "accounting_period", "amount", "category_id", "comment", "counterparty_id", "transaction_date"},
		},
		{
			name: "payment amount corrected",
			act: func(t *testing.T, ctx context.Context) []businessevent.Subject {
				t.Helper()
				p := newPayment(t)
				_, err := payments.Update(ctx, p.UpdateAmount(money.New(1800, "USD")))
				require.NoError(t, err)
				return []businessevent.Subject{{Resource: services.PaymentResource, ID: p.ID().String()}}
			},
			eventType: services.PaymentUpdated,
			fields:    []string{"amount"},
		},
		{
			name: "payment deleted",
			act: func(t *testing.T, ctx context.Context) []businessevent.Subject {
				t.Helper()
				p := newPayment(t)
				_, err := payments.Delete(ctx, p.ID())
				require.NoError(t, err)
				return []businessevent.Subject{{Resource: services.PaymentResource, ID: p.ID().String()}}
			},
			eventType: services.PaymentDeleted,
			fields:    []string{"account_id", "accounting_period", "amount", "category_id", "comment", "counterparty_id", "transaction_date"},
		},
		{
			name: "file attached to payment",
			act: func(t *testing.T, ctx context.Context) []businessevent.Subject {
				t.Helper()
				p, uploadID := newPayment(t), newUpload(t)
				require.NoError(t, payments.AttachFileToPayment(ctx, p.ID(), uploadID))
				return []businessevent.Subject{
					{Resource: services.UploadResource, ID: strconv.FormatUint(uint64(uploadID), 10)},
					{Resource: services.PaymentResource, ID: p.ID().String()},
				}
			},
			eventType: services.PaymentFileAttached,
		},
		{
			name: "file detached from payment",
			act: func(t *testing.T, ctx context.Context) []businessevent.Subject {
				t.Helper()
				p, uploadID := newPayment(t), newUpload(t)
				require.NoError(t, payments.AttachFileToPayment(env.Ctx, p.ID(), uploadID))
				require.NoError(t, payments.DetachFileFromPayment(ctx, p.ID(), uploadID))
				return []businessevent.Subject{
					{Resource: services.UploadResource, ID: strconv.FormatUint(uint64(uploadID), 10)},
					{Resource: services.PaymentResource, ID: p.ID().String()},
				}
			},
			eventType: services.PaymentFileDetached,
		},
		{
			name: "expense created",
			act: func(t *testing.T, ctx context.Context) []businessevent.Subject {
				t.Helper()
				created, err := expenses.Create(ctx, expense.New(money.New(900, "USD"), f.newAccount(t), f.newExpenseCategory(t), day,
					expense.WithTenantID(env.TenantID()), expense.WithAccountingPeriod(day), expense.WithComment("Taxi"),
				))
				require.NoError(t, err)
				return []businessevent.Subject{{Resource: services.ExpenseResource, ID: created.ID().String()}}
			},
			eventType: services.ExpenseCreated,
			fields:    []string{"account_id", "accounting_period", "amount", "category_id", "comment", "date"},
		},
		{
			name: "expense comment changed",
			act: func(t *testing.T, ctx context.Context) []businessevent.Subject {
				t.Helper()
				e := newExpense(t)
				_, err := expenses.Update(ctx, e.SetComment("Taxi to the airport"))
				require.NoError(t, err)
				return []businessevent.Subject{{Resource: services.ExpenseResource, ID: e.ID().String()}}
			},
			eventType: services.ExpenseUpdated,
			fields:    []string{"comment"},
		},
		{
			name: "expense deleted",
			act: func(t *testing.T, ctx context.Context) []businessevent.Subject {
				t.Helper()
				e := newExpense(t)
				_, err := expenses.Delete(ctx, e.ID())
				require.NoError(t, err)
				return []businessevent.Subject{{Resource: services.ExpenseResource, ID: e.ID().String()}}
			},
			eventType: services.ExpenseDeleted,
			fields:    []string{"account_id", "accounting_period", "amount", "category_id", "comment", "date"},
		},
		{
			name: "file attached to expense",
			act: func(t *testing.T, ctx context.Context) []businessevent.Subject {
				t.Helper()
				e, uploadID := newExpense(t), newUpload(t)
				require.NoError(t, expenses.AttachFileToExpense(ctx, e.ID(), uploadID))
				return []businessevent.Subject{
					{Resource: services.UploadResource, ID: strconv.FormatUint(uint64(uploadID), 10)},
					{Resource: services.ExpenseResource, ID: e.ID().String()},
				}
			},
			eventType: services.ExpenseFileAttached,
		},
		{
			name: "file detached from expense",
			act: func(t *testing.T, ctx context.Context) []businessevent.Subject {
				t.Helper()
				e, uploadID := newExpense(t), newUpload(t)
				require.NoError(t, expenses.AttachFileToExpense(env.Ctx, e.ID(), uploadID))
				require.NoError(t, expenses.DetachFileFromExpense(ctx, e.ID(), uploadID))
				return []businessevent.Subject{
					{Resource: services.UploadResource, ID: strconv.FormatUint(uint64(uploadID), 10)},
					{Resource: services.ExpenseResource, ID: e.ID().String()},
				}
			},
			eventType: services.ExpenseFileDetached,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := itf.TestExecution(env.Ctx)
			subjects := tt.act(t, ctx)

			recorded := f.operationEvents(t, ctx)
			require.Len(t, recorded, 1)
			e := recorded[0]
			require.Equal(t, tt.eventType, e.Type)
			require.Equal(t, subjects, e.Subjects)
			if tt.fields == nil {
				require.Empty(t, e.Changes)
				return
			}
			require.Len(t, e.Changes, 1)
			fields := make([]string, 0, len(e.Changes[0].Fields))
			for _, d := range e.Changes[0].Fields {
				fields = append(fields, d.Field)
			}
			sort.Strings(fields)
			require.Equal(t, tt.fields, fields)
		})
	}
}

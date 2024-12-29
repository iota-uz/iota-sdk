package permissions

import (
	"github.com/google/uuid"
	permission2 "github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

const (
	ResourceExpense         permission2.Resource = "expense"
	ResourcePayment         permission2.Resource = "payment"
	ResourceExpenseCategory permission2.Resource = "expense_category"
)

var (
	PaymentCreate = permission2.Permission{
		ID:       uuid.MustParse("e3aa7c6b-1bb5-4e26-9817-a7787011eb7a"),
		Name:     "Payment.Create",
		Resource: ResourcePayment,
		Action:   permission2.ActionCreate,
		Modifier: permission2.ModifierAll,
	}
	PaymentRead = permission2.Permission{
		ID:       uuid.MustParse("3423a1f0-9528-4461-be9f-1be0bf6706e1"),
		Name:     "Payment.Read",
		Resource: ResourcePayment,
		Action:   permission2.ActionRead,
		Modifier: permission2.ModifierAll,
	}
	PaymentUpdate = permission2.Permission{
		ID:       uuid.MustParse("032db88f-18b7-4548-b3ef-ec6916ad4e91"),
		Name:     "Payment.Update",
		Resource: ResourcePayment,
		Action:   permission2.ActionUpdate,
		Modifier: permission2.ModifierAll,
	}
	PaymentDelete = permission2.Permission{
		ID:       uuid.MustParse("cb84dbb8-2acd-406a-b3e5-098ed57d304d"),
		Name:     "Payment.Delete",
		Resource: ResourcePayment,
		Action:   permission2.ActionDelete,
		Modifier: permission2.ModifierAll,
	}
	ExpenseCreate = permission2.Permission{
		ID:       uuid.MustParse("95ffb8ae-f448-436f-b132-99d1e061978c"),
		Name:     "Expense.Create",
		Resource: ResourceExpense,
		Action:   permission2.ActionCreate,
		Modifier: permission2.ModifierAll,
	}
	ExpenseRead = permission2.Permission{
		ID:       uuid.MustParse("628dda9c-10f6-4e7c-aa08-479e23cf0310"),
		Name:     "Expense.Read",
		Resource: ResourceExpense,
		Action:   permission2.ActionRead,
		Modifier: permission2.ModifierAll,
	}
	ExpenseUpdate = permission2.Permission{
		ID:       uuid.MustParse("265e0ba4-ea82-4760-95e1-1af55f9f5c99"),
		Name:     "Expense.Update",
		Resource: ResourceExpense,
		Action:   permission2.ActionUpdate,
		Modifier: permission2.ModifierAll,
	}
	ExpenseDelete = permission2.Permission{
		ID:       uuid.MustParse("052d00c5-2f39-4fcb-b1ba-dff0d0be021d"),
		Name:     "Expense.Delete",
		Resource: ResourceExpense,
		Action:   permission2.ActionDelete,
		Modifier: permission2.ModifierAll,
	}
	ExpenseCategoryCreate = permission2.Permission{
		ID:       uuid.MustParse("c75c9bc8-f13f-4612-980b-68288c3a87be"),
		Name:     "ExpenseCategory.Create",
		Resource: ResourceExpenseCategory,
		Action:   permission2.ActionCreate,
		Modifier: permission2.ModifierAll,
	}
	ExpenseCategoryRead = permission2.Permission{
		ID:       uuid.MustParse("16b438d2-3d13-42e1-972f-0d490947998e"),
		Name:     "ExpenseCategory.Read",
		Resource: ResourceExpenseCategory,
		Action:   permission2.ActionRead,
		Modifier: permission2.ModifierAll,
	}
	ExpenseCategoryUpdate = permission2.Permission{
		ID:       uuid.MustParse("2850c599-ef16-43bd-814c-477b45c2937d"),
		Name:     "ExpenseCategory.Update",
		Resource: ResourceExpenseCategory,
		Action:   permission2.ActionUpdate,
		Modifier: permission2.ModifierAll,
	}
	ExpenseCategoryDelete = permission2.Permission{
		ID:       uuid.MustParse("4e4d249c-b726-4ccf-859a-8df51285a78a"),
		Name:     "ExpenseCategory.Delete",
		Resource: ResourceExpenseCategory,
		Action:   permission2.ActionDelete,
		Modifier: permission2.ModifierAll,
	}
)

var Permissions = []permission2.Permission{
	PaymentCreate,
	PaymentRead,
	PaymentUpdate,
	PaymentDelete,
	ExpenseCreate,
	ExpenseRead,
	ExpenseUpdate,
	ExpenseDelete,
	ExpenseCategoryCreate,
	ExpenseCategoryRead,
	ExpenseCategoryUpdate,
	ExpenseCategoryDelete,
}

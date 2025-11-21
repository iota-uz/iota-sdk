package permissions

import (
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

const (
	ResourceExpense         permission.Resource = "expense"
	ResourcePayment         permission.Resource = "payment"
	ResourceExpenseCategory permission.Resource = "expense_category"
	ResourceDebt            permission.Resource = "debt"
)

var (
	PaymentCreate = permission.MustCreate(
		uuid.MustParse("e3aa7c6b-1bb5-4e26-9817-a7787011eb7a"),
		"Payment.Create",
		ResourcePayment,
		permission.ActionCreate,
		permission.ModifierAll,
	)
	PaymentRead = permission.MustCreate(
		uuid.MustParse("3423a1f0-9528-4461-be9f-1be0bf6706e1"),
		"Payment.Read",
		ResourcePayment,
		permission.ActionRead,
		permission.ModifierAll,
	)
	PaymentUpdate = permission.MustCreate(
		uuid.MustParse("032db88f-18b7-4548-b3ef-ec6916ad4e91"),
		"Payment.Update",
		ResourcePayment,
		permission.ActionUpdate,
		permission.ModifierAll,
	)
	PaymentDelete = permission.MustCreate(
		uuid.MustParse("cb84dbb8-2acd-406a-b3e5-098ed57d304d"),
		"Payment.Delete",
		ResourcePayment,
		permission.ActionDelete,
		permission.ModifierAll,
	)
	ExpenseCreate = permission.MustCreate(
		uuid.MustParse("95ffb8ae-f448-436f-b132-99d1e061978c"),
		"Expense.Create",
		ResourceExpense,
		permission.ActionCreate,
		permission.ModifierAll,
	)
	ExpenseRead = permission.MustCreate(
		uuid.MustParse("628dda9c-10f6-4e7c-aa08-479e23cf0310"),
		"Expense.Read",
		ResourceExpense,
		permission.ActionRead,
		permission.ModifierAll,
	)
	ExpenseUpdate = permission.MustCreate(
		uuid.MustParse("265e0ba4-ea82-4760-95e1-1af55f9f5c99"),
		"Expense.Update",
		ResourceExpense,
		permission.ActionUpdate,
		permission.ModifierAll,
	)
	ExpenseDelete = permission.MustCreate(
		uuid.MustParse("052d00c5-2f39-4fcb-b1ba-dff0d0be021d"),
		"Expense.Delete",
		ResourceExpense,
		permission.ActionDelete,
		permission.ModifierAll,
	)
	ExpenseCategoryCreate = permission.MustCreate(
		uuid.MustParse("c75c9bc8-f13f-4612-980b-68288c3a87be"),
		"ExpenseCategory.Create",
		ResourceExpenseCategory,
		permission.ActionCreate,
		permission.ModifierAll,
	)
	ExpenseCategoryRead = permission.MustCreate(
		uuid.MustParse("16b438d2-3d13-42e1-972f-0d490947998e"),
		"ExpenseCategory.Read",
		ResourceExpenseCategory,
		permission.ActionRead,
		permission.ModifierAll,
	)
	ExpenseCategoryUpdate = permission.MustCreate(
		uuid.MustParse("2850c599-ef16-43bd-814c-477b45c2937d"),
		"ExpenseCategory.Update",
		ResourceExpenseCategory,
		permission.ActionUpdate,
		permission.ModifierAll,
	)
	ExpenseCategoryDelete = permission.MustCreate(
		uuid.MustParse("4e4d249c-b726-4ccf-859a-8df51285a78a"),
		"ExpenseCategory.Delete",
		ResourceExpenseCategory,
		permission.ActionDelete,
		permission.ModifierAll,
	)
	DebtCreate = permission.MustCreate(
		uuid.MustParse("7a8b9c1d-2e3f-4a5b-6c7d-8e9f0a1b2c3d"),
		"Debt.Create",
		ResourceDebt,
		permission.ActionCreate,
		permission.ModifierAll,
	)
	DebtRead = permission.MustCreate(
		uuid.MustParse("8b9c1d2e-3f4a-5b6c-7d8e-9f0a1b2c3d4e"),
		"Debt.Read",
		ResourceDebt,
		permission.ActionRead,
		permission.ModifierAll,
	)
	DebtUpdate = permission.MustCreate(
		uuid.MustParse("9c1d2e3f-4a5b-6c7d-8e9f-0a1b2c3d4e5f"),
		"Debt.Update",
		ResourceDebt,
		permission.ActionUpdate,
		permission.ModifierAll,
	)
	DebtDelete = permission.MustCreate(
		uuid.MustParse("1d2e3f4a-5b6c-7d8e-9f0a-1b2c3d4e5f6a"),
		"Debt.Delete",
		ResourceDebt,
		permission.ActionDelete,
		permission.ModifierAll,
	)
)

var Permissions = []permission.Permission{
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
	DebtCreate,
	DebtRead,
	DebtUpdate,
	DebtDelete,
}

export const Debt: import("@iota-uz/sdk/runtime").Entity<"finance.debt", {
    type: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    status: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    counterpartyId: import("@iota-uz/sdk/runtime").Field<string, "req", "uuid">;
    originalAmount: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
    originalCurrency: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    outstandingAmount: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
    outstandingCurrency: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    dueDate: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODate | null, "opt", "date">;
    projectId: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "uuid">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">;
export const Cancelled: import("@iota-uz/sdk/runtime").Event<"finance.debt.cancelled.v1", import("@iota-uz/sdk/runtime").Entity<"finance.debt", {
    type: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    status: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    counterpartyId: import("@iota-uz/sdk/runtime").Field<string, "req", "uuid">;
    originalAmount: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
    originalCurrency: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    outstandingAmount: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
    outstandingCurrency: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    dueDate: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODate | null, "opt", "date">;
    projectId: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "uuid">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">, import("@iota-uz/sdk/runtime").ObjectSchema<{
    debtId: import("@iota-uz/sdk/runtime").Schema<string, "req", "uuid">;
    type: import("@iota-uz/sdk/runtime").Schema<"receivable" | "payable", "req", "enum">;
    counterpartyId: import("@iota-uz/sdk/runtime").Schema<string, "req", "uuid">;
    originalAmount: import("@iota-uz/sdk/runtime").ObjectSchema<{
        amount: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
        currency: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    }>;
}>>;
export const DebtCreated: import("@iota-uz/sdk/runtime").Event<"finance.debt.created.v1", import("@iota-uz/sdk/runtime").Entity<"finance.debt", {
    type: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    status: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    counterpartyId: import("@iota-uz/sdk/runtime").Field<string, "req", "uuid">;
    originalAmount: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
    originalCurrency: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    outstandingAmount: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
    outstandingCurrency: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    dueDate: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODate | null, "opt", "date">;
    projectId: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "uuid">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">, import("@iota-uz/sdk/runtime").ObjectSchema<{
    debtId: import("@iota-uz/sdk/runtime").Schema<string, "req", "uuid">;
    type: import("@iota-uz/sdk/runtime").Schema<"receivable" | "payable", "req", "enum">;
    counterpartyId: import("@iota-uz/sdk/runtime").Schema<string, "req", "uuid">;
    originalAmount: import("@iota-uz/sdk/runtime").ObjectSchema<{
        amount: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
        currency: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    }>;
    dueDate: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").ISODate, "opt", "date">;
    projectId: import("@iota-uz/sdk/runtime").Schema<string, "opt", "uuid">;
}>>;
export const Settled: import("@iota-uz/sdk/runtime").Event<"finance.debt.settled.v1", import("@iota-uz/sdk/runtime").Entity<"finance.debt", {
    type: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    status: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    counterpartyId: import("@iota-uz/sdk/runtime").Field<string, "req", "uuid">;
    originalAmount: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
    originalCurrency: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    outstandingAmount: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
    outstandingCurrency: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    dueDate: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODate | null, "opt", "date">;
    projectId: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "uuid">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">, import("@iota-uz/sdk/runtime").ObjectSchema<{
    debtId: import("@iota-uz/sdk/runtime").Schema<string, "req", "uuid">;
    type: import("@iota-uz/sdk/runtime").Schema<"receivable" | "payable", "req", "enum">;
    status: import("@iota-uz/sdk/runtime").Schema<"partial" | "settled", "req", "enum">;
    counterpartyId: import("@iota-uz/sdk/runtime").Schema<string, "req", "uuid">;
    originalAmount: import("@iota-uz/sdk/runtime").ObjectSchema<{
        amount: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
        currency: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    }>;
    outstandingAmount: import("@iota-uz/sdk/runtime").ObjectSchema<{
        amount: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
        currency: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    }>;
    settledAmount: import("@iota-uz/sdk/runtime").ObjectSchema<{
        amount: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
        currency: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    }>;
    settlementTransactionId: import("@iota-uz/sdk/runtime").Schema<string, "opt", "uuid">;
    dueDate: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").ISODate, "opt", "date">;
}>>;
export const Written_off: import("@iota-uz/sdk/runtime").Event<"finance.debt.written_off.v1", import("@iota-uz/sdk/runtime").Entity<"finance.debt", {
    type: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    status: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    counterpartyId: import("@iota-uz/sdk/runtime").Field<string, "req", "uuid">;
    originalAmount: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
    originalCurrency: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    outstandingAmount: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
    outstandingCurrency: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    dueDate: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODate | null, "opt", "date">;
    projectId: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "uuid">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">, import("@iota-uz/sdk/runtime").ObjectSchema<{
    debtId: import("@iota-uz/sdk/runtime").Schema<string, "req", "uuid">;
    type: import("@iota-uz/sdk/runtime").Schema<"receivable" | "payable", "req", "enum">;
    counterpartyId: import("@iota-uz/sdk/runtime").Schema<string, "req", "uuid">;
    originalAmount: import("@iota-uz/sdk/runtime").ObjectSchema<{
        amount: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
        currency: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    }>;
}>>;
export const Expense: import("@iota-uz/sdk/runtime").Entity<"finance.expense", {
    categoryId: import("@iota-uz/sdk/runtime").Field<string, "req", "uuid">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">;
export const ExpenseCreated: import("@iota-uz/sdk/runtime").Event<"finance.expense.created.v1", import("@iota-uz/sdk/runtime").Entity<"finance.expense", {
    categoryId: import("@iota-uz/sdk/runtime").Field<string, "req", "uuid">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">, import("@iota-uz/sdk/runtime").ObjectSchema<{
    expenseId: import("@iota-uz/sdk/runtime").Schema<string, "req", "uuid">;
    amount: import("@iota-uz/sdk/runtime").ObjectSchema<{
        amount: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
        currency: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    }>;
    accountId: import("@iota-uz/sdk/runtime").Schema<string, "req", "uuid">;
    categoryId: import("@iota-uz/sdk/runtime").Schema<string, "req", "uuid">;
    date: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").ISODate, "req", "date">;
}>>;
export const Payment: import("@iota-uz/sdk/runtime").Entity<"finance.payment", {
    counterpartyId: import("@iota-uz/sdk/runtime").Field<string, "req", "uuid">;
    categoryId: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "uuid">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">;
export const PaymentCreated: import("@iota-uz/sdk/runtime").Event<"finance.payment.created.v1", import("@iota-uz/sdk/runtime").Entity<"finance.payment", {
    counterpartyId: import("@iota-uz/sdk/runtime").Field<string, "req", "uuid">;
    categoryId: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "uuid">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">, import("@iota-uz/sdk/runtime").ObjectSchema<{
    paymentId: import("@iota-uz/sdk/runtime").Schema<string, "req", "uuid">;
    amount: import("@iota-uz/sdk/runtime").ObjectSchema<{
        amount: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
        currency: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    }>;
    accountId: import("@iota-uz/sdk/runtime").Schema<string, "req", "uuid">;
    categoryId: import("@iota-uz/sdk/runtime").Schema<string, "opt", "uuid">;
    counterpartyId: import("@iota-uz/sdk/runtime").Schema<string, "opt", "uuid">;
    transactionDate: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").ISODate, "req", "date">;
    comment: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
}>>;
export const Updated: import("@iota-uz/sdk/runtime").Event<"finance.payment.updated.v1", import("@iota-uz/sdk/runtime").Entity<"finance.payment", {
    counterpartyId: import("@iota-uz/sdk/runtime").Field<string, "req", "uuid">;
    categoryId: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "uuid">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">, import("@iota-uz/sdk/runtime").ObjectSchema<{
    paymentId: import("@iota-uz/sdk/runtime").Schema<string, "req", "uuid">;
    amount: import("@iota-uz/sdk/runtime").ObjectSchema<{
        amount: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
        currency: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    }>;
    changed: import("@iota-uz/sdk/runtime").Schema<("counterpartyId" | "amount" | "categoryId" | "accountId" | "transactionDate" | "comment" | "accountingPeriod")[], "req", "array">;
}>>;

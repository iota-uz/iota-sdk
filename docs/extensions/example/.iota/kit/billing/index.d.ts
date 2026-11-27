export const Transaction: import("@iota-uz/sdk/runtime").Entity<"billing.transaction", {
    status: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    currency: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    gateway: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime, "req", "datetime">;
    updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime, "req", "datetime">;
}, "native">;
export const Status_changed: import("@iota-uz/sdk/runtime").Event<"billing.transaction.status_changed.v1", import("@iota-uz/sdk/runtime").Entity<"billing.transaction", {
    status: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    currency: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    gateway: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime, "req", "datetime">;
    updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime, "req", "datetime">;
}, "native">, import("@iota-uz/sdk/runtime").ObjectSchema<{
    transactionId: import("@iota-uz/sdk/runtime").Schema<string, "req", "uuid">;
    gateway: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    from: import("@iota-uz/sdk/runtime").Schema<"created" | "pending" | "completed" | "failed" | "canceled" | "refunded" | "partially-refunded" | "expired", "req", "enum">;
    to: import("@iota-uz/sdk/runtime").Schema<"created" | "pending" | "completed" | "failed" | "canceled" | "refunded" | "partially-refunded" | "expired", "req", "enum">;
    amount: import("@iota-uz/sdk/runtime").ObjectSchema<{
        amount: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
        currency: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    }>;
}>>;

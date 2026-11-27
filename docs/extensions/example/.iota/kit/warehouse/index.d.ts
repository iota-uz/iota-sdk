export const Order: import("@iota-uz/sdk/runtime").Entity<"warehouse.order", {
    type: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    status: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">;
export const Completed: import("@iota-uz/sdk/runtime").Event<"warehouse.order.completed.v1", import("@iota-uz/sdk/runtime").Entity<"warehouse.order", {
    type: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    status: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">, import("@iota-uz/sdk/runtime").ObjectSchema<{
    orderId: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
    type: import("@iota-uz/sdk/runtime").Schema<"in" | "out", "req", "enum">;
}>>;
export const Created: import("@iota-uz/sdk/runtime").Event<"warehouse.order.created.v1", import("@iota-uz/sdk/runtime").Entity<"warehouse.order", {
    type: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    status: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">, import("@iota-uz/sdk/runtime").ObjectSchema<{
    orderId: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
    type: import("@iota-uz/sdk/runtime").Schema<"in" | "out", "req", "enum">;
    status: import("@iota-uz/sdk/runtime").Schema<"pending" | "complete", "req", "enum">;
}>>;

export const Action_failed: import("@iota-uz/sdk/runtime").DomainError<"automation.action_failed", import("@iota-uz/sdk/runtime").ObjectSchema<{}>>;
export const Forbidden: import("@iota-uz/sdk/runtime").DomainError<"automation.forbidden", import("@iota-uz/sdk/runtime").ObjectSchema<{}>>;
export const Invalid_output: import("@iota-uz/sdk/runtime").DomainError<"automation.invalid_output", import("@iota-uz/sdk/runtime").ObjectSchema<{}>>;
export const Task: import("@iota-uz/sdk/runtime").Entity<"automation.task", {
    status: import("@iota-uz/sdk/runtime").Field<"open" | "decided" | "expired" | "cancelled", "req", "enum">;
    dueAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime, "req", "datetime">;
    decidedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">;
export const Assigned: import("@iota-uz/sdk/runtime").Event<"automation.task.assigned.v1", import("@iota-uz/sdk/runtime").Entity<"automation.task", {
    status: import("@iota-uz/sdk/runtime").Field<"open" | "decided" | "expired" | "cancelled", "req", "enum">;
    dueAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime, "req", "datetime">;
    decidedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">, import("@iota-uz/sdk/runtime").ObjectSchema<{
    task_id: import("@iota-uz/sdk/runtime").Schema<string, "req", "uuid">;
    run_id: import("@iota-uz/sdk/runtime").Schema<string, "req", "uuid">;
    automation_id: import("@iota-uz/sdk/runtime").Schema<string, "req", "uuid">;
    node_id: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    title: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    reason: import("@iota-uz/sdk/runtime").Schema<"created" | "delegated" | "escalated", "req", "enum">;
    user_ids: import("@iota-uz/sdk/runtime").Schema<number[], "req", "array">;
    delegated_by: import("@iota-uz/sdk/runtime").Schema<number, "opt", "int">;
    due_at: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").ISODateTime, "opt", "datetime">;
}>>;
export const Transport_unavailable: import("@iota-uz/sdk/runtime").DomainError<"automation.transport_unavailable", import("@iota-uz/sdk/runtime").ObjectSchema<{}>>;
export const User_not_found: import("@iota-uz/sdk/runtime").DomainError<"automation.user_not_found", import("@iota-uz/sdk/runtime").ObjectSchema<{}>>;

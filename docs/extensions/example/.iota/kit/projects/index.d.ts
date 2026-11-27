export const Acceptance: import("@iota-uz/sdk/runtime").Entity<"projects.acceptance", {
    projectId: import("@iota-uz/sdk/runtime").Field<string, "req", "uuid">;
    kind: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    number: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    date: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODate, "req", "date">;
    signedOn: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODate | null, "opt", "date">;
    amount: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
    currency: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    status: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">;
export const Signed: import("@iota-uz/sdk/runtime").Event<"projects.acceptance.signed.v1", import("@iota-uz/sdk/runtime").Entity<"projects.acceptance", {
    projectId: import("@iota-uz/sdk/runtime").Field<string, "req", "uuid">;
    kind: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    number: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    date: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODate, "req", "date">;
    signedOn: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODate | null, "opt", "date">;
    amount: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
    currency: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    status: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">, import("@iota-uz/sdk/runtime").ObjectSchema<{
    acceptanceId: import("@iota-uz/sdk/runtime").Schema<string, "req", "uuid">;
    projectId: import("@iota-uz/sdk/runtime").Schema<string, "req", "uuid">;
    kind: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    number: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    amount: import("@iota-uz/sdk/runtime").ObjectSchema<{
        amount: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
        currency: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    }>;
    signedOn: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").ISODate, "req", "date">;
}>>;

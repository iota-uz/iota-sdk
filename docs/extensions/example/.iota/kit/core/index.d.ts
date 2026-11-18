export const User: import("@iota-uz/sdk/runtime").Entity<"core.user", {
    firstName: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    lastName: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    middleName: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    email: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    phone: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    uiLanguage: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime, "req", "datetime">;
    updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime, "req", "datetime">;
}, "native">;
export const Created: import("@iota-uz/sdk/runtime").Event<"core.user.created.v1", import("@iota-uz/sdk/runtime").Entity<"core.user", {
    firstName: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    lastName: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    middleName: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    email: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    phone: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    uiLanguage: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime, "req", "datetime">;
    updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime, "req", "datetime">;
}, "native">, import("@iota-uz/sdk/runtime").ObjectSchema<{
    userId: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
    email: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    firstName: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    lastName: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    userType: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
}>>;

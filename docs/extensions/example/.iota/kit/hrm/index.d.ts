export const Employee: import("@iota-uz/sdk/runtime").Entity<"hrm.employee", {
    firstName: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    lastName: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    middleName: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    email: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    phone: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">;
export const Created: import("@iota-uz/sdk/runtime").Event<"hrm.employee.created.v1", import("@iota-uz/sdk/runtime").Entity<"hrm.employee", {
    firstName: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    lastName: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    middleName: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    email: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    phone: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">, import("@iota-uz/sdk/runtime").ObjectSchema<{
    employeeId: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
    firstName: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    lastName: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    email: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    phone: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
    hireDate: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").ISODate, "req", "date">;
}>>;
export const Updated: import("@iota-uz/sdk/runtime").Event<"hrm.employee.updated.v1", import("@iota-uz/sdk/runtime").Entity<"hrm.employee", {
    firstName: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    lastName: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    middleName: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    email: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    phone: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">, import("@iota-uz/sdk/runtime").ObjectSchema<{
    employeeId: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
    changed: import("@iota-uz/sdk/runtime").Schema<string[], "req", "array">;
    resignationDate: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").ISODate, "opt", "date">;
}>>;

export const Group: import("@iota-uz/sdk/runtime").Entity<"core.group", {
    name: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    description: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    type: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">;
export const Add_user: import("@iota-uz/sdk/runtime").Command<"core.group.add_user", import("@iota-uz/sdk/runtime").ObjectSchema<{
    group_id: import("@iota-uz/sdk/runtime").Schema<string, "req", "uuid">;
    user_id: import("@iota-uz/sdk/runtime").Schema<number, "req", "int">;
}>, import("@iota-uz/sdk/runtime").ObjectSchema<{
    outcome: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    output: import("@iota-uz/sdk/runtime").Schema<{
        group_id: string;
        members: number;
    }, "opt", "object">;
}>, false>;
export const GroupCreated: import("@iota-uz/sdk/runtime").Event<"core.group.created.v1", import("@iota-uz/sdk/runtime").Entity<"core.group", {
    name: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    description: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    type: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">, import("@iota-uz/sdk/runtime").ObjectSchema<{
    groupId: import("@iota-uz/sdk/runtime").Schema<string, "req", "uuid">;
    name: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
}>>;
export const Member_added: import("@iota-uz/sdk/runtime").Event<"core.group.member_added.v1", import("@iota-uz/sdk/runtime").Entity<"core.group", {
    name: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    description: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    type: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">, import("@iota-uz/sdk/runtime").ObjectSchema<{
    groupId: import("@iota-uz/sdk/runtime").Schema<string, "req", "uuid">;
    userId: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
    groupName: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
}>>;
export const Member_removed: import("@iota-uz/sdk/runtime").Event<"core.group.member_removed.v1", import("@iota-uz/sdk/runtime").Entity<"core.group", {
    name: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    description: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    type: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">, import("@iota-uz/sdk/runtime").ObjectSchema<{
    groupId: import("@iota-uz/sdk/runtime").Schema<string, "req", "uuid">;
    userId: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
    groupName: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
}>>;
export const Remove_user: import("@iota-uz/sdk/runtime").Command<"core.group.remove_user", import("@iota-uz/sdk/runtime").ObjectSchema<{
    group_id: import("@iota-uz/sdk/runtime").Schema<string, "req", "uuid">;
    user_id: import("@iota-uz/sdk/runtime").Schema<number, "req", "int">;
}>, import("@iota-uz/sdk/runtime").ObjectSchema<{
    outcome: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    output: import("@iota-uz/sdk/runtime").Schema<{
        group_id: string;
        members: number;
    }, "opt", "object">;
}>, false>;
export const Role: import("@iota-uz/sdk/runtime").Entity<"core.role", {
    name: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    description: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    type: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">;
export const RoleCreated: import("@iota-uz/sdk/runtime").Event<"core.role.created.v1", import("@iota-uz/sdk/runtime").Entity<"core.role", {
    name: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    description: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    type: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">, import("@iota-uz/sdk/runtime").ObjectSchema<{
    roleId: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
    name: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
}>>;
export const Permission_granted: import("@iota-uz/sdk/runtime").Event<"core.role.permission_granted.v1", import("@iota-uz/sdk/runtime").Entity<"core.role", {
    name: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    description: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    type: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">, import("@iota-uz/sdk/runtime").ObjectSchema<{
    roleId: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
    permission: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
}>>;
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
export const Block: import("@iota-uz/sdk/runtime").Command<"core.user.block", import("@iota-uz/sdk/runtime").ObjectSchema<{
    user_id: import("@iota-uz/sdk/runtime").Schema<number, "req", "int">;
    reason: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
}>, import("@iota-uz/sdk/runtime").ObjectSchema<{
    outcome: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    output: import("@iota-uz/sdk/runtime").Schema<{
        user_id: number;
        blocked: boolean;
    }, "opt", "object">;
}>, false>;
export const Blocked: import("@iota-uz/sdk/runtime").Event<"core.user.blocked.v1", import("@iota-uz/sdk/runtime").Entity<"core.user", {
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
    blockReason: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
}>>;
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
    phone: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
}>>;
export const Deleted: import("@iota-uz/sdk/runtime").Event<"core.user.deleted.v1", import("@iota-uz/sdk/runtime").Entity<"core.user", {
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
    firstName: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    lastName: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
}>>;
export const Role_assigned: import("@iota-uz/sdk/runtime").Event<"core.user.role_assigned.v1", import("@iota-uz/sdk/runtime").Entity<"core.user", {
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
    roleId: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
}>>;
export const Unblock: import("@iota-uz/sdk/runtime").Command<"core.user.unblock", import("@iota-uz/sdk/runtime").ObjectSchema<{
    user_id: import("@iota-uz/sdk/runtime").Schema<number, "req", "int">;
}>, import("@iota-uz/sdk/runtime").ObjectSchema<{
    outcome: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    output: import("@iota-uz/sdk/runtime").Schema<{
        user_id: number;
        blocked: boolean;
    }, "opt", "object">;
}>, false>;
export const Unblocked: import("@iota-uz/sdk/runtime").Event<"core.user.unblocked.v1", import("@iota-uz/sdk/runtime").Entity<"core.user", {
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
}>>;
export const Updated: import("@iota-uz/sdk/runtime").Event<"core.user.updated.v1", import("@iota-uz/sdk/runtime").Entity<"core.user", {
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
    firstName: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    lastName: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    changed: import("@iota-uz/sdk/runtime").Schema<string[], "req", "array">;
}>>;

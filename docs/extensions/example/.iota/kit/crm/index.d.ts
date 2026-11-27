/** @type {import("@iota-uz/sdk/runtime").ListRef<"crm.lists.clients">} */
export const ListsClients: import("@iota-uz/sdk/runtime").ListRef<"crm.lists.clients">;
export const Chat: import("@iota-uz/sdk/runtime").Entity<"crm.chat", {
    clientId: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
    lastMessageAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime, "req", "datetime">;
}, "native">;
export const Message_added: import("@iota-uz/sdk/runtime").Event<"crm.chat.message_added.v1", import("@iota-uz/sdk/runtime").Entity<"crm.chat", {
    clientId: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
    lastMessageAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime, "req", "datetime">;
}, "native">, import("@iota-uz/sdk/runtime").ObjectSchema<{
    chatId: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
    clientId: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
    messageId: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
    transport: import("@iota-uz/sdk/runtime").Schema<"telegram" | "whatsapp" | "instagram" | "sms" | "email" | "phone" | "website" | "other", "req", "enum">;
    direction: import("@iota-uz/sdk/runtime").Schema<"inbound" | "outbound", "req", "enum">;
    body: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    sentAt: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").ISODateTime, "req", "datetime">;
}>>;
export const Send_message: import("@iota-uz/sdk/runtime").Command<"crm.chat.send_message", import("@iota-uz/sdk/runtime").ObjectSchema<{
    chat_id: import("@iota-uz/sdk/runtime").Schema<number, "req", "int">;
    message: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    transport: import("@iota-uz/sdk/runtime").Schema<"telegram" | "whatsapp" | "instagram" | "sms" | "email" | "phone" | "website" | "other", "opt", "enum">;
}>, import("@iota-uz/sdk/runtime").ObjectSchema<{
    outcome: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    output: import("@iota-uz/sdk/runtime").Schema<{
        chat_id: number;
        message_id: number;
    }, "opt", "object">;
}>, false>;
export const Client: import("@iota-uz/sdk/runtime").Entity<"crm.client", {
    firstName: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    lastName: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    middleName: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    phone: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    address: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    email: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    dateOfBirth: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODate | null, "opt", "date">;
    gender: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    comments: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">;
export const Created: import("@iota-uz/sdk/runtime").Event<"crm.client.created.v1", import("@iota-uz/sdk/runtime").Entity<"crm.client", {
    firstName: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    lastName: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    middleName: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    phone: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    address: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    email: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    dateOfBirth: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODate | null, "opt", "date">;
    gender: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    comments: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">, import("@iota-uz/sdk/runtime").ObjectSchema<{
    clientId: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
    firstName: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    lastName: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
    middleName: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
    phone: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
    email: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
}>>;
export const Deleted: import("@iota-uz/sdk/runtime").Event<"crm.client.deleted.v1", import("@iota-uz/sdk/runtime").Entity<"crm.client", {
    firstName: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    lastName: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    middleName: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    phone: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    address: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    email: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    dateOfBirth: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODate | null, "opt", "date">;
    gender: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    comments: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">, import("@iota-uz/sdk/runtime").ObjectSchema<{
    clientId: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
    firstName: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
}>>;
export const Updated: import("@iota-uz/sdk/runtime").Event<"crm.client.updated.v1", import("@iota-uz/sdk/runtime").Entity<"crm.client", {
    firstName: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
    lastName: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    middleName: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    phone: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    address: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    email: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    dateOfBirth: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODate | null, "opt", "date">;
    gender: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    comments: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
    createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
}, "native">, import("@iota-uz/sdk/runtime").ObjectSchema<{
    clientId: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Int64, "req", "int64">;
    firstName: import("@iota-uz/sdk/runtime").Schema<string, "req", "string">;
    lastName: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
    middleName: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
    phone: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
    email: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
    changed: import("@iota-uz/sdk/runtime").Schema<string[], "req", "array">;
}>>;
export const Clients: import("@iota-uz/sdk/runtime").Route<"crm.clients", import("@iota-uz/sdk/runtime").ObjectSchema<{}>, "/crm/clients">;
export const SaveClient: import("@iota-uz/sdk/runtime").Command<"crm.commands.saveClient", import("@iota-uz/sdk/runtime").ObjectSchema<{
    client: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Ref<import("@iota-uz/sdk/runtime").Entity<"crm.client", {
        firstName: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
        lastName: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
        middleName: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
        phone: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
        address: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
        email: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
        dateOfBirth: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODate | null, "opt", "date">;
        gender: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
        comments: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
        createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
        updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    }, "native">>, "opt", "ref">;
    firstName: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
    lastName: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
    middleName: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
    phone: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
    email: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
    address: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
    passportSeries: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
    passportNumber: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
    pin: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
    countryCode: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
    comments: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
}>, import("@iota-uz/sdk/runtime").ObjectSchema<{
    client: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Ref<import("@iota-uz/sdk/runtime").Entity<"crm.client", {
        firstName: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
        lastName: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
        middleName: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
        phone: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
        address: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
        email: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
        dateOfBirth: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODate | null, "opt", "date">;
        gender: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
        comments: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
        createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
        updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    }, "native">>, "req", "ref">;
    created: import("@iota-uz/sdk/runtime").Schema<boolean, "req", "bool">;
}>, false>;
export const UpsertClient: import("@iota-uz/sdk/runtime").Operation<"crm.operations.upsertClient", import("@iota-uz/sdk/runtime").ObjectSchema<{
    client: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Ref<import("@iota-uz/sdk/runtime").Entity<"crm.client", {
        firstName: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
        lastName: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
        middleName: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
        phone: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
        address: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
        email: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
        dateOfBirth: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODate | null, "opt", "date">;
        gender: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
        comments: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
        createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
        updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    }, "native">>, "opt", "ref">;
    firstName: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
    lastName: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
    middleName: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
    phone: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
    email: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
    address: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
    passportSeries: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
    passportNumber: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
    pin: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
    countryCode: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
    comments: import("@iota-uz/sdk/runtime").Schema<string, "opt", "string">;
}>, import("@iota-uz/sdk/runtime").ObjectSchema<{
    client: import("@iota-uz/sdk/runtime").Schema<import("@iota-uz/sdk/runtime").Ref<import("@iota-uz/sdk/runtime").Entity<"crm.client", {
        firstName: import("@iota-uz/sdk/runtime").Field<string, "req", "string">;
        lastName: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
        middleName: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
        phone: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
        address: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
        email: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
        dateOfBirth: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODate | null, "opt", "date">;
        gender: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
        comments: import("@iota-uz/sdk/runtime").Field<string | null, "opt", "string">;
        createdAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
        updatedAt: import("@iota-uz/sdk/runtime").Field<import("@iota-uz/sdk/runtime").ISODateTime | null, "opt", "datetime">;
    }, "native">>, "req", "ref">;
    created: import("@iota-uz/sdk/runtime").Schema<boolean, "req", "bool">;
}>>;
export const ClientView: import("@iota-uz/sdk/runtime").Route<"crm.routes.clientView", import("@iota-uz/sdk/runtime").ObjectSchema<{
    id: import("@iota-uz/sdk/runtime").Schema<number, "req", "int">;
}>, "/crm/clients/:id">;
export const ClientSidebar: import("@iota-uz/sdk/runtime").SlotRef<{
    clientId: number;
}>;

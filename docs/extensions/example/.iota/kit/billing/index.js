// Generated developer kit module: pure references, no handlers.
import { event, field, t } from "@iota-uz/sdk/runtime";
import { foreign, nativeEntity } from "@iota-uz/sdk/runtime/kit";

export const Transaction = nativeEntity("billing.transaction", { "status": field.text({ maxLength: 50 }), "currency": field.text({ maxLength: 3 }), "gateway": field.text({ maxLength: 50 }), "createdAt": field.datetime(), "updatedAt": field.datetime() }, { key: "uuid" });
export const Status_changed = foreign(event("billing.transaction.status_changed.v1", { subject: Transaction, payload: t.object({ "transactionId": t.uuid(), "gateway": t.string(), "from": t.enum(["created","pending","completed","failed","canceled","refunded","partially-refunded","expired"]), "to": t.enum(["created","pending","completed","failed","canceled","refunded","partially-refunded","expired"]), "amount": t.object({ "amount": t.int64(), "currency": t.string() }) }), description: "The status of a billing transaction changed. Published in the transaction that saves the new status, only when the status differs from the previous one." }));

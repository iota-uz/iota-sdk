// Generated developer kit module: pure references, no handlers.
import { event, field, t } from "@iota-uz/sdk/runtime";
import { foreign, nativeEntity } from "@iota-uz/sdk/runtime/kit";

export const Order = nativeEntity("warehouse.order", { "type": field.text({ maxLength: 255 }), "status": field.text({ maxLength: 255 }), "createdAt": field.datetime().optional() }, { key: "int64" });
export const Completed = foreign(event("warehouse.order.completed.v1", { subject: Order, payload: t.object({ "orderId": t.int64(), "type": t.enum(["in","out"]) }), description: "A warehouse order was completed. Published in the transaction that completes the order." }));
export const Created = foreign(event("warehouse.order.created.v1", { subject: Order, payload: t.object({ "orderId": t.int64(), "type": t.enum(["in","out"]), "status": t.enum(["pending","complete"]) }), description: "A warehouse order was created. Published in the transaction that creates the order." }));

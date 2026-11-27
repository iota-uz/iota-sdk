// Generated developer kit module: pure references, no handlers.
import { event, field, t } from "@iota-uz/sdk/runtime";
import { foreign, nativeEntity } from "@iota-uz/sdk/runtime/kit";

export const Acceptance = nativeEntity("projects.acceptance", { "projectId": field.uuid(), "kind": field.text({ maxLength: 32 }), "number": field.text({ maxLength: 64 }), "date": field.date(), "signedOn": field.date().optional(), "amount": field.int64(), "currency": field.text({ maxLength: 3 }), "status": field.text({ maxLength: 16 }), "createdAt": field.datetime().optional(), "updatedAt": field.datetime().optional() }, { key: "uuid" });
export const Signed = foreign(event("projects.acceptance.signed.v1", { subject: Acceptance, payload: t.object({ "acceptanceId": t.uuid(), "projectId": t.uuid(), "kind": t.string(), "number": t.string(), "amount": t.object({ "amount": t.int64(), "currency": t.string() }), "signedOn": t.date() }), description: "An acceptance document of a project was signed. Published in the transaction that signs the document." }));

// Generated developer kit module: pure references, no handlers.
import { event, field, t } from "@iota-uz/sdk/runtime";
import { foreign, nativeEntity } from "@iota-uz/sdk/runtime/kit";

export const User = nativeEntity("core.user", { "firstName": field.text({ maxLength: 255 }), "lastName": field.text({ maxLength: 255 }), "middleName": field.text({ maxLength: 255 }).optional(), "email": field.text({ maxLength: 255 }), "phone": field.text({ maxLength: 255 }).optional(), "uiLanguage": field.text({ maxLength: 16 }), "createdAt": field.datetime(), "updatedAt": field.datetime() }, { key: "int64" });
export const Created = foreign(event("core.user.created.v1", { subject: User, payload: t.object({ "userId": t.int64(), "email": t.string(), "firstName": t.string(), "lastName": t.string(), "userType": t.string() }), description: "A user account was created. Published in the transaction that creates the user." }));

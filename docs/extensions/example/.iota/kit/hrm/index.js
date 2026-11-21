// Generated developer kit module: pure references, no handlers.
import { event, field, t } from "@iota-uz/sdk/runtime";
import { foreign, nativeEntity } from "@iota-uz/sdk/runtime/kit";

export const Employee = nativeEntity("hrm.employee", { "firstName": field.text({ maxLength: 255 }), "lastName": field.text({ maxLength: 255 }), "middleName": field.text({ maxLength: 255 }).optional(), "email": field.text({ maxLength: 255 }), "phone": field.text({ maxLength: 255 }).optional(), "createdAt": field.datetime().optional(), "updatedAt": field.datetime().optional() }, { key: "int64" });
export const Created = foreign(event("hrm.employee.created.v1", { subject: Employee, payload: t.object({ "employeeId": t.int64(), "firstName": t.string(), "lastName": t.string(), "email": t.string(), "phone": t.string().optional(), "hireDate": t.date() }), description: "An employee was created. Published in the transaction that creates the employee." }));
export const Updated = foreign(event("hrm.employee.updated.v1", { subject: Employee, payload: t.object({ "employeeId": t.int64(), "changed": t.array(t.string()), "resignationDate": t.date().optional() }), description: "An employee was updated. Published in the transaction of the update; changed lists the field IDs it changed, without their values." }));

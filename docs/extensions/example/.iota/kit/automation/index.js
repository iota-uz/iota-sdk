// Generated developer kit module: pure references, no handlers.
import { domainError, event, field, t } from "@iota-uz/sdk/runtime";
import { foreign, nativeEntity } from "@iota-uz/sdk/runtime/kit";

export const Action_failed = foreign(domainError("automation.action_failed", t.object({  })));
export const Forbidden = foreign(domainError("automation.forbidden", t.object({  })));
export const Invalid_output = foreign(domainError("automation.invalid_output", t.object({  })));
export const Task = nativeEntity("automation.task", { "status": field.enum(["open","decided","expired","cancelled"]), "dueAt": field.datetime().optional(), "createdAt": field.datetime(), "decidedAt": field.datetime().optional() }, { key: "uuid" });
export const Assigned = foreign(event("automation.task.assigned.v1", { subject: Task, payload: t.object({ "task_id": t.uuid(), "run_id": t.uuid(), "automation_id": t.uuid(), "node_id": t.string(), "title": t.string(), "reason": t.enum(["created","delegated","escalated"]), "user_ids": t.array(t.int()), "delegated_by": t.int().optional(), "due_at": t.datetime().optional() }), description: "A human step of an automation was assigned to people: on creation, delegation or escalation." }));
export const Transport_unavailable = foreign(domainError("automation.transport_unavailable", t.object({  })));
export const User_not_found = foreign(domainError("automation.user_not_found", t.object({  })));

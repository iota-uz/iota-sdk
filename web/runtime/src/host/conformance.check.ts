// Compile-time conformance: the runtime objects built by context.ts expose exactly
// the members of the public context interfaces of ../server.ts. Type-only; not built.
import type { CommandContext, EndpointRequest, GuardContext, QueryContext, Tx, WorkflowContext } from "../server.ts";
import type {
  createCommandContext,
  createEndpointRequest,
  createGuardContext,
  createQuery,
  createTx,
  createWorkflowContext,
} from "./context.ts";

type SameKeys<A, B> = [keyof A] extends [keyof B] ? ([keyof B] extends [keyof A] ? true : false) : false;
type Assert<T extends true> = T;

export type Conformance = [
  Assert<SameKeys<ReturnType<typeof createTx>, Tx>>,
  Assert<SameKeys<ReturnType<typeof createQuery>, QueryContext>>,
  Assert<SameKeys<ReturnType<typeof createCommandContext>, CommandContext>>,
  Assert<SameKeys<ReturnType<typeof createWorkflowContext>, WorkflowContext>>,
  Assert<SameKeys<ReturnType<typeof createGuardContext>, GuardContext>>,
  Assert<SameKeys<ReturnType<typeof createEndpointRequest>, EndpointRequest>>,
];

// Wire types of the iota.runtime/1 protocol (docs/extension-runtime.md §3).
// Type-only: erased by the host build.

export const PROTOCOL = "iota.runtime/1";

export type ErrorKind = "domain" | "validation" | "denied" | "internal" | "suspend";

export interface WireIssue {
  code: string;
  phase?: "input" | "write";
  source?: string;
  path?: Array<string | number>;
  params?: unknown;
}

export interface WireError {
  kind: ErrorKind;
  code?: string;
  message?: string;
  params?: unknown;
  issues?: WireIssue[];
  suspend?: unknown;
  stack?: string;
}

export type InvokeKind =
  | "command"
  | "atomic"
  | "operation"
  | "query"
  | "subscription"
  | "workflow"
  | "guard"
  | "template"
  | "endpoint"
  | "schedule";

export interface InvokeContext {
  tenant: string;
  /** id is the int64 user id as a decimal string. */
  user: { id: string; kind: "initiator" | "owner"; locale?: string; permissions: string[] };
  invocation: string;
  trigger: "ui" | "mcp" | "endpoint" | "schedule" | "subscription" | "workflow";
  parent?: string;
  instance?: string;
  occurrence?: unknown;
  deadline?: string;
  tx?: string;
}

export interface InvokeFrame {
  t: "invoke";
  id: string;
  artifact: { hash: string; dir: string };
  kind: InvokeKind;
  export: string;
  resource: string;
  input: unknown;
  ctx: InvokeContext;
}

export interface ReplyFrame {
  t: "reply";
  inv: string;
  id: number;
  ok: boolean;
  value?: unknown;
  error?: WireError;
}

export type InboundFrame =
  | { t: "hello"; protocol: string }
  | InvokeFrame
  | ReplyFrame
  | { t: "cancel"; id: string };

export type CallFn = (method: string, args: Record<string, unknown>) => Promise<unknown>;

// Runtime context objects handed to handlers. Each one implements the matching
// interface of ../server.ts; conformance.check.ts asserts the member sets match.
import { compileFilter } from "../entity.ts";
import type { Int64, Json } from "../model.ts";
import type { Principal } from "../server.ts";
import { CODE, internal, toWire } from "./errors.ts";
import type { CallFn, InvokeContext } from "./protocol.ts";

type Resource = string | { readonly id: string };
type Reference = string | { readonly key: string };
type Level = "debug" | "info" | "warn" | "error";
type Fields = Readonly<Record<string, Json>> | undefined;

export interface Deps {
  invocation: string;
  ctx: InvokeContext;
  call: CallFn;
  log: (level: Level, message: string) => void;
}

const idOf = (resource: Resource): string => (typeof resource === "string" ? resource : resource.id);
const keyOf = (ref: Reference): string => (typeof ref === "string" ? ref : ref.key);

function formatLog(message: string, fields: Fields): string {
  return fields === undefined ? message : `${message} ${JSON.stringify(fields)}`;
}

function logger(deps: Deps) {
  const at = (level: Level) => (message: string, fields?: Fields) => deps.log(level, formatLog(message, fields));
  return { debug: at("debug"), info: at("info"), warn: at("warn"), error: at("error") };
}

function principal(ctx: InvokeContext): Principal {
  const granted = new Set(ctx.user.permissions);
  return {
    tenantId: ctx.tenant,
    userId: ctx.user.id as Int64,
    kind: ctx.user.kind,
    ...(ctx.user.locale === undefined ? {} : { locale: ctx.user.locale }),
    has: (permission) => {
      const id = typeof permission === "string" ? permission : permission.id;
      return id === "" || granted.has(id);
    },
  };
}

function invocationInfo(deps: Deps) {
  const { ctx } = deps;
  return {
    id: deps.invocation,
    trigger: ctx.trigger,
    ...(ctx.parent === undefined ? {} : { parentId: ctx.parent }),
  };
}

interface FindArgs {
  where?: object;
  orderBy?: readonly { field: string; desc?: boolean }[];
  limit?: number;
  offset?: number;
}

function findQuery(query: FindArgs | undefined): Record<string, unknown> {
  if (query === undefined) return {};
  const { where, ...rest } = query;
  return where === undefined ? rest : { ...rest, where: compileFilter(where) };
}

/** Read access shared by Tx (`tx.*`, handle-bound) and QueryContext (`q.*`, snapshot). */
function reader(call: (op: "get" | "find" | "count", args: Record<string, unknown>) => Promise<unknown>) {
  return {
    get: (entity: Resource, ref: Reference) => call("get", { entity: idOf(entity), key: keyOf(ref) }),
    find: (entity: Resource, query?: FindArgs) => call("find", { entity: idOf(entity), query: findQuery(query) }),
    count: (entity: Resource, where?: object) =>
      call("count", { entity: idOf(entity), where: where === undefined ? null : compileFilter(where) }),
  };
}

/** Tx proxy bound to one Go-owned transaction handle. */
export function createTx(deps: Deps, handle: string, scope: { open: boolean }) {
  const call = (method: string, args: Record<string, unknown>) => {
    if (!scope.open) {
      return Promise.reject(internal(CODE.txClosed, "transaction is closed"));
    }
    return deps.call(method, { tx: handle, ...args });
  };
  return {
    user: principal(deps.ctx),
    invocation: invocationInfo(deps),
    ...reader((op, args) => call(`tx.${op}`, args)),
    insert: (entity: Resource, values: unknown) => call("tx.insert", { entity: idOf(entity), values }),
    update: (entity: Resource, ref: Reference, patch: object, options?: { rowVersion?: number }) =>
      call("tx.update", {
        entity: idOf(entity),
        key: keyOf(ref),
        patch: options?.rowVersion === undefined ? patch : { ...patch, rowVersion: options.rowVersion },
      }),
    delete: async (entity: Resource, ref: Reference): Promise<void> => {
      await call("tx.delete", { entity: idOf(entity), key: keyOf(ref) });
    },
    put: (overlay: Resource, base: Reference, values: unknown) =>
      call("tx.putOverlay", { overlay: idOf(overlay), baseKey: keyOf(base), values }),
    call: (callable: Resource, input: unknown) => call("tx.call", { contract: idOf(callable), input }),
    emit: async (event: Resource, args: object): Promise<void> => {
      await call("tx.emit", { event: idOf(event), ...args });
    },
    notify: async (notice: Resource, args: object): Promise<void> => {
      await call("tx.notify", { notice: idOf(notice), ...args });
    },
    start: async (workflow: Resource, input: unknown, options?: { key?: string }) => {
      const started = (await call("tx.start", {
        workflow: idOf(workflow),
        input,
        ...(options?.key === undefined ? {} : { key: options.key }),
      })) as { instance: string };
      return { workflow: idOf(workflow), instance: started.instance };
    },
  };
}

/** Read-only data access; Go serves it from a per-invocation read snapshot. */
export function createQuery(deps: Deps) {
  return {
    user: principal(deps.ctx),
    invocation: invocationInfo(deps),
    log: logger(deps),
    ...reader((op, args) => deps.call(`q.${op}`, args)),
  };
}

/** Runs cb inside a Go-owned transaction opened by `open` and closed by `close`. */
async function inTransaction<T>(
  deps: Deps,
  open: () => Promise<string>,
  close: (handle: string, outcome: "commit" | "rollback", value?: unknown) => Promise<unknown>,
  cb: (tx: ReturnType<typeof createTx>) => Promise<T>,
): Promise<T> {
  const handle = await open();
  const scope = { open: true };
  let result: T;
  try {
    result = await cb(createTx(deps, handle, scope));
  } catch (err) {
    scope.open = false;
    await close(handle, "rollback").catch(() => undefined);
    throw err;
  }
  scope.open = false;
  await close(handle, "commit", result);
  return result;
}

/** CommandContext of an ordinary command. */
export function createCommandContext(deps: Deps) {
  let inTx = false;
  return {
    user: principal(deps.ctx),
    invocation: invocationInfo(deps),
    log: logger(deps),
    query: createQuery(deps),
    secrets: { get: (name: string) => deps.call("secrets.get", { name }) as Promise<string> },
    call: (command: Resource, input: unknown) => deps.call("invoke.command", { command: idOf(command), input }),
    async transaction<T>(cb: (tx: ReturnType<typeof createTx>) => Promise<T>): Promise<T> {
      if (inTx) {
        throw internal(CODE.nestedTransaction, "ctx.transaction cannot be nested");
      }
      inTx = true;
      try {
        return await inTransaction(
          deps,
          async () => ((await deps.call("tx.begin", {})) as { tx: string }).tx,
          (tx, outcome) => deps.call("tx.end", { tx, outcome }),
          cb,
        );
      } finally {
        inTx = false;
      }
    },
  };
}

type Replayed = { replay: true; value: unknown } | { replay: false; [key: string]: unknown };

interface SignalRefLike {
  readonly workflow: string;
  readonly name: string;
}

type StepEffect = (step: { idempotencyKey: string }) => Promise<unknown>;

/** WorkflowContext: every durable primitive is a wf.* host call; Go owns the journal. */
export function createWorkflowContext(deps: Deps) {
  const assignee = (value: unknown) =>
    typeof value === "object" && value !== null && "userId" in value
      ? { entity: "core.user", key: String((value as { userId: unknown }).userId) }
      : value;
  return {
    user: principal(deps.ctx),
    instance: { id: deps.ctx.instance ?? "" },
    log: logger(deps),
    async transaction(
      name: string,
      body: ((tx: ReturnType<typeof createTx>) => Promise<unknown>) | Resource,
      input?: unknown,
    ): Promise<unknown> {
      if (typeof body !== "function") {
        return deps.call("wf.transaction.call", { name, callable: idOf(body), input });
      }
      const begun = (await deps.call("wf.transaction.begin", { name })) as Replayed;
      if (begun.replay) return begun.value;
      return inTransaction(
        deps,
        async () => begun.tx as string,
        (tx, outcome, value) => deps.call("wf.transaction.end", { name, tx, outcome, value: value ?? null }),
        body,
      );
    },
    async step(name: string, first: unknown, second?: StepEffect): Promise<unknown> {
      const effect = (typeof first === "function" ? first : second) as StepEffect;
      const begun = (await deps.call("wf.step.begin", { name })) as Replayed;
      if (begun.replay) return begun.value;
      let value: unknown;
      try {
        value = await effect({ idempotencyKey: begun.idempotencyKey as string });
      } catch (err) {
        await deps.call("wf.step.end", { name, ok: false, error: toWire(err) }).catch(() => undefined);
        throw err;
      }
      await deps.call("wf.step.end", { name, ok: true, value: value ?? null });
      return value;
    },
    sleep: async (name: string, duration: string): Promise<void> => {
      await deps.call("wf.sleep", { name, duration });
    },
    async waitFor(name: string, signal: SignalRefLike, options?: { timeout: string }): Promise<unknown> {
      const result = (await deps.call("wf.waitFor", {
        name,
        workflow: signal.workflow,
        signal: signal.name,
        ...(options === undefined ? {} : { timeout: options.timeout }),
      })) as { outcome: string; payload?: unknown };
      return options === undefined ? result.payload : result;
    },
    task: (name: string, spec: { subject: unknown; assignee: unknown; decision: { type: unknown }; timeout?: string }) =>
      deps.call("wf.task", {
        name,
        subject: spec.subject,
        assignee: assignee(spec.assignee),
        decision: spec.decision.type,
        ...(spec.timeout === undefined ? {} : { timeout: spec.timeout }),
      }),
    call: (name: string, command: Resource, input: unknown) =>
      deps.call("wf.call", { name, command: idOf(command), input }),
    start: async (name: string, workflow: Resource, input: unknown) => {
      const started = (await deps.call("wf.start", { name, workflow: idOf(workflow), input })) as { instance: string };
      return { workflow: idOf(workflow), instance: started.instance };
    },
    now: (name: string) => deps.call("wf.now", { name }) as Promise<string>,
    uuid: (name: string) => deps.call("wf.uuid", { name }) as Promise<string>,
  };
}

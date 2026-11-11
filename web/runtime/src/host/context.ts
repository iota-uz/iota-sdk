import { CODE, internal, toWire } from "./errors.ts";
import type { CallFn, InvokeContext } from "./protocol.ts";

type Resource = string | { id: string };
type Reference = string | { key: string };
type Level = "debug" | "info" | "warn" | "error";

export interface Deps {
  invocation: string;
  ctx: InvokeContext;
  call: CallFn;
  signal: AbortSignal;
  log: (level: Level, message: string) => void;
}

const idOf = (resource: Resource): string => (typeof resource === "string" ? resource : resource.id);
const keyOf = (ref: Reference): string => (typeof ref === "string" ? ref : ref.key);

function formatLog(message: string, fields?: unknown): string {
  return fields === undefined ? message : `${message} ${JSON.stringify(fields)}`;
}

function logger(deps: Deps) {
  const at = (level: Level) => (message: string, fields?: unknown) => deps.log(level, formatLog(message, fields));
  return { debug: at("debug"), info: at("info"), warn: at("warn"), error: at("error") };
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
    get: (entity: Resource, ref: Reference) => call("tx.get", { entity: idOf(entity), key: keyOf(ref) }),
    find: (entity: Resource, query?: unknown) => call("tx.find", { entity: idOf(entity), query: query ?? {} }),
    count: (entity: Resource, where?: unknown) => call("tx.count", { entity: idOf(entity), where: where ?? null }),
    insert: (entity: Resource, values: unknown) => call("tx.insert", { entity: idOf(entity), values }),
    update: (entity: Resource, ref: Reference, patch: unknown) =>
      call("tx.update", { entity: idOf(entity), key: keyOf(ref), patch }),
    delete: (entity: Resource, ref: Reference) => call("tx.delete", { entity: idOf(entity), key: keyOf(ref) }),
    put: (overlay: Resource, base: Reference, values: unknown) =>
      call("tx.putOverlay", { overlay: idOf(overlay), baseKey: keyOf(base), values }),
    call: (contract: Resource, input: unknown) => call("tx.call", { contract: idOf(contract), input }),
    emit: (event: Resource, payload: unknown) => call("tx.emit", { event: idOf(event), ...(payload as object) }),
    notify: (notice: Resource, request: unknown) => call("tx.notify", { notice: idOf(notice), ...(request as object) }),
    start: (workflow: Resource, input: unknown, options?: { key?: string }) =>
      call("tx.start", { workflow: idOf(workflow), input, ...(options?.key === undefined ? {} : { key: options.key }) }),
  };
}

export type TxProxy = ReturnType<typeof createTx>;

/** Read-only data access; Go serves it from a per-invocation read snapshot. */
export function createQuery(deps: Deps) {
  return {
    user: deps.ctx.user,
    invocation: deps.invocation,
    signal: deps.signal,
    log: logger(deps),
    get: (entity: Resource, ref: Reference) => deps.call("q.get", { entity: idOf(entity), key: keyOf(ref) }),
    find: (entity: Resource, query?: unknown) => deps.call("q.find", { entity: idOf(entity), query: query ?? {} }),
    count: (entity: Resource, where?: unknown) => deps.call("q.count", { entity: idOf(entity), where: where ?? null }),
  };
}

export type QueryProxy = ReturnType<typeof createQuery>;

/** Runs cb inside a Go-owned transaction opened by `open` and closed by `close`. */
async function inTransaction<T>(
  deps: Deps,
  open: () => Promise<string>,
  close: (handle: string, outcome: "commit" | "rollback", value?: unknown) => Promise<unknown>,
  cb: (tx: TxProxy) => Promise<T>,
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
export function createCommandContext(deps: Deps, secretsAllowed: boolean) {
  let inTx = false;
  return {
    user: deps.ctx.user,
    tenant: deps.ctx.tenant,
    invocation: deps.invocation,
    signal: deps.signal,
    log: logger(deps),
    query: createQuery(deps),
    secrets: {
      get: (name: string) => {
        if (!secretsAllowed) return Promise.reject(internal(CODE.hostCall, "secrets are not available"));
        return deps.call("secrets.get", { name }) as Promise<string>;
      },
    },
    call: (command: Resource, input: unknown) => deps.call("invoke.command", { command: idOf(command), input }),
    async transaction<T>(cb: (tx: TxProxy) => Promise<T>): Promise<T> {
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

/** WorkflowContext: every durable primitive is a wf.* host call; Go owns the journal. */
export function createWorkflowContext(deps: Deps) {
  return {
    user: deps.ctx.user,
    tenant: deps.ctx.tenant,
    invocation: deps.invocation,
    occurrence: deps.ctx.occurrence,
    signal: deps.signal,
    log: logger(deps),
    async transaction<T>(name: string, cb: (tx: TxProxy) => Promise<T>): Promise<T> {
      const begun = (await deps.call("wf.transaction.begin", { name })) as Replayed;
      if (begun.replay) return begun.value as T;
      return inTransaction(
        deps,
        async () => begun.tx as string,
        (tx, outcome, value) => deps.call("wf.transaction.end", { name, tx, outcome, value: value ?? null }),
        cb,
      );
    },
    async step<T>(name: string, fn: (info: { idempotencyKey: string }) => Promise<T>): Promise<T> {
      const begun = (await deps.call("wf.step.begin", { name })) as Replayed;
      if (begun.replay) return begun.value as T;
      let value: T;
      try {
        value = await fn({ idempotencyKey: begun.idempotencyKey as string });
      } catch (err) {
        await deps.call("wf.step.end", { name, ok: false, error: toWire(err) }).catch(() => undefined);
        throw err;
      }
      await deps.call("wf.step.end", { name, ok: true, value: value ?? null });
      return value;
    },
    sleep: (name: string, duration: string) => deps.call("wf.sleep", { name, duration }),
    waitFor: (name: string, signal: string, options?: { timeout?: string }) =>
      deps.call("wf.waitFor", { name, signal, ...options }),
    task: (name: string, options: unknown) => deps.call("wf.task", { name, ...(options as object) }),
    call: (name: string, command: Resource, input: unknown) =>
      deps.call("wf.call", { name, command: idOf(command), input }),
    start: (name: string, workflow: Resource, input: unknown) =>
      deps.call("wf.start", { name, workflow: idOf(workflow), input }),
  };
}

/** Context handed to kinds that carry no data access (template, endpoint). */
export function createPlainContext(deps: Deps) {
  return { user: deps.ctx.user, tenant: deps.ctx.tenant, invocation: deps.invocation, signal: deps.signal, log: logger(deps) };
}

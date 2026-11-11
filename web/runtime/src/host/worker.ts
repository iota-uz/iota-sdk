// Host worker of the shared Node pool (docs/extension-runtime.md §3).
// Speaks iota.runtime/1 over stdio: 4-byte big-endian length + UTF-8 JSON.
import { AsyncLocalStorage } from "node:async_hooks";
import { stat } from "node:fs/promises";
import { join } from "node:path";
import { pathToFileURL } from "node:url";

import {
  createCommandContext,
  createEndpointRequest,
  createGuardContext,
  createQuery,
  createTx,
  createWorkflowContext,
  type Deps,
} from "./context.ts";
import { CODE, RuntimeError, fromWire, internal, toWire } from "./errors.ts";
import { PROTOCOL } from "./protocol.ts";
import type { InboundFrame, InvokeFrame, ReplyFrame, WireError } from "./protocol.ts";

type Level = "debug" | "info" | "warn" | "error";
type Handler = (...args: unknown[]) => unknown;

interface Invocation {
  id: string;
  controller: AbortController;
  suspended: RuntimeError | null;
}

const als = new AsyncLocalStorage<Invocation>();
const stdout = process.stdout;
const writeRaw = stdout.write.bind(stdout) as (chunk: Uint8Array) => boolean;

const invocations = new Map<string, Invocation>();
const pending = new Map<number, { inv: string; resolve: (v: unknown) => void; reject: (e: unknown) => void }>();
const modules = new Map<string, Promise<Record<string, unknown>>>();
let nextCall = 1;

function send(frame: object): void {
  const payload = Buffer.from(JSON.stringify(frame), "utf8");
  const head = Buffer.alloc(4);
  head.writeUInt32BE(payload.length, 0);
  writeRaw(Buffer.concat([head, payload]));
}

function sendLog(inv: string, level: Level, msg: string): void {
  send({ t: "log", inv, level, msg });
}

// Anything a package prints must never corrupt the protocol stream.
function routeConsole(): void {
  const forward = (level: Level) => (...args: unknown[]) =>
    sendLog(als.getStore()?.id ?? "", level, args.map((a) => (typeof a === "string" ? a : JSON.stringify(a))).join(" "));
  console.log = forward("info");
  console.info = forward("info");
  console.debug = forward("debug");
  console.warn = forward("warn");
  console.error = forward("error");
  stdout.write = ((chunk: string | Uint8Array, ...rest: unknown[]) => {
    sendLog(als.getStore()?.id ?? "", "info", typeof chunk === "string" ? chunk : Buffer.from(chunk).toString("utf8"));
    const cb = rest.find((r) => typeof r === "function") as (() => void) | undefined;
    cb?.();
    return true;
  }) as typeof stdout.write;
}

function hostCall(inv: Invocation, method: string, args: Record<string, unknown>): Promise<unknown> {
  if (inv.suspended) {
    return Promise.reject(internal(CODE.suspended, "invocation is suspended; host calls are rejected"));
  }
  if (inv.controller.signal.aborted) {
    return Promise.reject(internal(CODE.canceled, "invocation was canceled"));
  }
  const id = nextCall++;
  return new Promise((resolve, reject) => {
    pending.set(id, { inv: inv.id, resolve, reject });
    send({ t: "call", inv: inv.id, id, method, args });
  }).catch((err: unknown) => {
    if (err instanceof RuntimeError && err.kind === "suspend") {
      inv.suspended = err;
    }
    throw err;
  });
}

function onReply(frame: ReplyFrame): void {
  const call = pending.get(frame.id);
  if (!call) return;
  pending.delete(frame.id);
  if (frame.ok) {
    call.resolve(frame.value);
  } else {
    call.reject(fromWire(frame.error ?? { kind: "internal", code: CODE.hostCall, message: "host call failed" }));
  }
}

function onCancel(id: string): void {
  const inv = invocations.get(id);
  if (!inv) return;
  inv.controller.abort();
  for (const [callId, call] of pending) {
    if (call.inv === id) {
      pending.delete(callId);
      call.reject(internal(CODE.canceled, "invocation was canceled"));
    }
  }
}

function loadArtifact(artifact: { hash: string; dir: string }): Promise<Record<string, unknown>> {
  let loaded = modules.get(artifact.hash);
  if (!loaded) {
    loaded = (async () => {
      const entry = join(artifact.dir, "server.mjs");
      try {
        await stat(entry);
      } catch {
        throw internal(CODE.artifactMissing, `artifact ${artifact.hash}: ${entry} does not exist`);
      }
      try {
        return (await import(pathToFileURL(entry).href)) as Record<string, unknown>;
      } catch (err) {
        throw internal(CODE.artifactMissing, `artifact ${artifact.hash}: import failed: ${(err as Error).message}`);
      }
    })();
    loaded.catch(() => modules.delete(artifact.hash));
    modules.set(artifact.hash, loaded);
  }
  return loaded;
}

function isObject(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object";
}

/** Server values are functions or branded objects carrying `handler` (or `renderers` for templates). */
function resolveHandler(mod: Record<string, unknown>, name: string): Handler {
  const value = mod[name];
  if (typeof value === "function") return value as Handler;
  if (isObject(value) && typeof value.handler === "function") {
    return (value.handler as Handler).bind(value);
  }
  throw internal(CODE.exportMissing, `export "${name}" is not an implementation`);
}

function resolveRenderer(mod: Record<string, unknown>, name: string, channel: string): Handler {
  const value = mod[name];
  const renderer = isObject(value) && isObject(value.renderers) ? value.renderers[channel] : undefined;
  if (typeof renderer !== "function") {
    throw internal(CODE.exportMissing, `export "${name}" has no ${channel} template`);
  }
  return renderer as Handler;
}

function requireTx(frame: InvokeFrame): string {
  if (!frame.ctx.tx) {
    throw internal(CODE.badInvocation, `${frame.kind} invocation requires a transaction handle`);
  }
  return frame.ctx.tx;
}

async function execute(frame: InvokeFrame, inv: Invocation): Promise<unknown> {
  const mod = await loadArtifact(frame.artifact);
  if (!(frame.export in mod)) {
    throw internal(CODE.exportMissing, `artifact ${frame.artifact.hash} has no export "${frame.export}"`);
  }

  const deps: Deps = {
    invocation: frame.id,
    ctx: frame.ctx,
    call: (method, args) => hostCall(inv, method, args),
    log: (level, message) => sendLog(frame.id, level, message),
  };

  const exported = mod[frame.export];
  const scope = { open: true };
  try {
    if (frame.kind === "template") {
      const args = frame.input as { channel: string; payload: unknown; subject: unknown; locale: string };
      return await resolveRenderer(mod, frame.export, args.channel)({ payload: args.payload, subject: args.subject, locale: args.locale });
    }
    const handler = resolveHandler(mod, frame.export);
    switch (frame.kind) {
      case "command":
        return await handler(createCommandContext(deps), frame.input);
      case "endpoint":
        return await handler({ ...createCommandContext(deps), request: createEndpointRequest(frame.input) });
      case "schedule":
        return await handler({ ...createCommandContext(deps), slot: (frame.input as { slot: string }).slot });
      case "atomic":
      case "operation":
      case "subscription":
        return await handler(createTx(deps, requireTx(frame), scope), frame.input);
      case "guard": {
        const phase = isObject(exported) ? exported.phase : undefined;
        const wantsTx = phase === undefined ? frame.ctx.tx !== undefined : phase !== "input";
        return await handler(wantsTx ? createGuardContext(deps, requireTx(frame), scope) : createQuery(deps), frame.input);
      }
      case "query":
        return await handler(createQuery(deps), frame.input);
      case "workflow":
        return await handler(createWorkflowContext(deps), frame.input);
      default:
        throw internal(CODE.badInvocation, `unsupported invocation kind "${String(frame.kind)}"`);
    }
  } finally {
    scope.open = false;
  }
}

async function onInvoke(frame: InvokeFrame): Promise<void> {
  const inv: Invocation = { id: frame.id, controller: new AbortController(), suspended: null };
  invocations.set(frame.id, inv);
  let done: { ok: true; value: unknown } | { ok: false; error: WireError };
  try {
    const value = await als.run(inv, () => execute(frame, inv));
    done = inv.suspended
      ? { ok: false, error: toWire(inv.suspended) }
      : { ok: true, value: value === undefined ? null : value };
  } catch (err) {
    done = { ok: false, error: toWire(inv.suspended ?? err) };
  } finally {
    invocations.delete(frame.id);
    for (const [callId, call] of pending) {
      if (call.inv === frame.id) pending.delete(callId);
    }
  }
  try {
    send({ t: "done", id: frame.id, ...done });
  } catch (err) {
    send({ t: "done", id: frame.id, ok: false, error: toWire(internal(CODE.hostCall, `result is not serializable: ${(err as Error).message}`)) });
  }
}

function onFrame(frame: InboundFrame, state: { helloed: boolean }): void {
  if (!state.helloed) {
    if (frame.t !== "hello") {
      process.stderr.write("iota-runtime-worker: first frame must be hello\n");
      process.exit(3);
    }
    state.helloed = true;
    send({ t: "ready", protocol: PROTOCOL, pid: process.pid });
    if (frame.protocol !== PROTOCOL) {
      process.stderr.write(`iota-runtime-worker: host speaks ${frame.protocol}, worker speaks ${PROTOCOL}\n`);
      process.exit(3);
    }
    return;
  }
  switch (frame.t) {
    case "invoke":
      void onInvoke(frame);
      break;
    case "reply":
      onReply(frame);
      break;
    case "cancel":
      onCancel(frame.id);
      break;
    default:
      process.stderr.write(`iota-runtime-worker: unexpected frame ${(frame as { t: string }).t}\n`);
  }
}

function main(): void {
  routeConsole();
  process.on("unhandledRejection", (reason) => {
    sendLog(als.getStore()?.id ?? "", "error", `unhandled rejection: ${reason instanceof Error ? (reason.stack ?? reason.message) : String(reason)}`);
  });
  process.on("uncaughtException", (err) => {
    process.stderr.write(`iota-runtime-worker: uncaught exception: ${err.stack ?? err.message}\n`);
    process.exit(70);
  });

  const state = { helloed: false };
  let buffered: Buffer = Buffer.alloc(0);
  process.stdin.on("data", (chunk: Buffer) => {
    buffered = buffered.length === 0 ? chunk : Buffer.concat([buffered, chunk]);
    while (buffered.length >= 4) {
      const size = buffered.readUInt32BE(0);
      if (buffered.length < 4 + size) break;
      const payload = buffered.subarray(4, 4 + size);
      buffered = buffered.subarray(4 + size);
      onFrame(JSON.parse(payload.toString("utf8")) as InboundFrame, state);
    }
  });
  process.stdin.on("end", () => process.exit(0));
}

main();

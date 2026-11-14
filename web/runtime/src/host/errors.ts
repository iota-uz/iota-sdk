import type { ErrorKind, WireError, WireIssue } from "./protocol.ts";

const KINDS: ReadonlySet<string> = new Set(["domain", "validation", "denied", "internal", "suspend"]);

export const CODE = {
  artifactMissing: "runtime.artifactMissing",
  exportMissing: "runtime.exportMissing",
  protocol: "runtime.protocol",
  hostCall: "runtime.hostCall",
  suspended: "runtime.suspended",
  canceled: "runtime.canceled",
  nestedTransaction: "runtime.nestedTransaction",
  txClosed: "runtime.txClosed",
  badInvocation: "runtime.badInvocation",
  intentInTransaction: "runtime.intentInTransaction",
  resultNotJson: "runtime.resultNotJson",
} as const;

/** Structured error that crosses the Go/Node boundary without losing its shape. */
export class RuntimeError extends Error {
  readonly kind: ErrorKind;
  readonly code: string | undefined;
  readonly params: unknown;
  readonly issues: WireIssue[] | undefined;
  readonly suspend: unknown;

  constructor(wire: WireError) {
    super(wire.message ?? wire.code ?? wire.kind);
    this.name = "RuntimeError";
    this.kind = wire.kind;
    this.code = wire.code;
    this.params = wire.params;
    this.issues = wire.issues;
    this.suspend = wire.suspend;
  }
}

export function internal(code: string, message: string): RuntimeError {
  return new RuntimeError({ kind: "internal", code, message });
}

export function fromWire(wire: WireError): RuntimeError {
  return new RuntimeError(KINDS.has(wire.kind) ? wire : { ...wire, kind: "internal" });
}

/**
 * Serializes anything thrown by a handler. Declared domain errors, validation
 * errors and denials (any object carrying a known `kind`, plus `code`,
 * `params`, `issues`) keep their structure; everything else is `internal`.
 */
export function toWire(err: unknown): WireError {
  if (err !== null && typeof err === "object") {
    const e = err as Record<string, unknown>;
    const wireOf = typeof e.toWire === "function" ? (e.toWire as () => unknown).call(err) : undefined;
    const source = (wireOf !== null && typeof wireOf === "object" ? wireOf : e) as Record<string, unknown>;
    if (typeof source.kind === "string" && KINDS.has(source.kind)) {
      const wire: WireError = { kind: source.kind as ErrorKind };
      if (typeof source.code === "string") wire.code = source.code;
      const message = typeof source.message === "string" ? source.message : undefined;
      if (message !== undefined) wire.message = message;
      if (source.params !== undefined) wire.params = source.params;
      if (Array.isArray(source.issues)) wire.issues = source.issues as WireIssue[];
      if (source.suspend !== undefined) wire.suspend = source.suspend;
      return wire;
    }
    if (err instanceof Error) {
      return { kind: "internal", code: "runtime.unhandled", message: err.message, stack: err.stack };
    }
  }
  return { kind: "internal", code: "runtime.unhandled", message: String(err) };
}

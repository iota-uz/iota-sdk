// Build worker protocol. One request per process: a single JSON document on stdin, a single JSON
// document on stdout (everything else a dependency prints goes to stderr). The tree to build is
// already materialized in `dir` (with node_modules when the package has npm dependencies).
import type { ManifestJSON, ResourceJSON } from "../model.ts";

export type Severity = "error" | "warning";

/** Points at the author's source, never at generated output. */
export interface Diagnostic {
  file: string;
  line?: number;
  column?: number;
  severity: Severity;
  code: string;
  message: string;
}

/** `@iota-kit/<ns>` modules: module name -> relative path -> text. */
export type KitModules = Readonly<Record<string, Readonly<Record<string, string>>>>;

export interface KitPayload {
  hash: string;
  modules: KitModules;
}

export interface PackageRequest {
  op: "check" | "build" | "test";
  /** Materialized source tree. */
  dir: string;
  /** Scratch/output directory; artifacts are written below it. */
  outDir: string;
  kit: KitPayload;
}

export interface KitSourcePayload {
  namespace: string;
  resources: readonly ResourceJSON[];
  /** Pure contracts module of an active package, shipped as the module JS. */
  contractsModule?: string;
  /** Native list IDs the namespace owns; packages extend them with `column(<ns>.<List>, ...)`. */
  lists?: readonly string[];
}

export interface KitRequest {
  op: "kit";
  sources: readonly KitSourcePayload[];
}

export type Request = PackageRequest | KitRequest;

export interface BuildResponse {
  diagnostics: Diagnostic[];
  manifest?: ManifestJSON;
  /** Artifact paths relative to `<outDir>/artifacts`. */
  artifacts: string[];
}

export interface TestCaseResult {
  name: string;
  file: string;
  passed: boolean;
  message?: string;
  durationMs: number;
}

export interface TestResponse {
  diagnostics: Diagnostic[];
  cases: TestCaseResult[];
}

export interface KitResponse {
  diagnostics: Diagnostic[];
  hash: string;
  modules: KitModules;
}

export type Response = BuildResponse | TestResponse | KitResponse;

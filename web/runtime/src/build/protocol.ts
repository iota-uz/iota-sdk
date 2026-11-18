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
  /** `test` only: typecheck the package first and run nothing when it does not compile (default true). */
  typecheck?: boolean;
  /** `test` only: run just the test files whose path contains one of these strings. */
  filter?: readonly string[];
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

export interface SourceLocation {
  file: string;
  line: number;
  column: number;
}

export interface TestCaseResult {
  name: string;
  /** The test file (relative to the package). */
  file: string;
  passed: boolean;
  message?: string;
  durationMs: number;
  /** Failure only: where in the author's source the failure surfaced (source-mapped, relative to the package). */
  location?: SourceLocation;
  /** Failure only: the source-mapped frames of the author's code and its dependencies, innermost first. */
  frames?: string[];
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

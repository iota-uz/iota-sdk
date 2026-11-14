// Child process entry for `test`: imports bundled test files, runs the cases they register through
// `@iota-uz/sdk/runtime/testing`, and blocks outbound network so an unknown effect fails the test
// instead of reaching a real service. The only way out is a fake http route (createTestHost({ fakes })).
import { readFileSync } from "node:fs";
import dns from "node:dns";
import { SourceMap } from "node:module";
import net from "node:net";
import { relative, resolve } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

interface Config {
  loader: string;
  files: { file: string; path: string; map: string; base: string }[];
  caseTimeoutMs: number;
  /** Directories of the platform's own modules (host, build worker): their frames are noise in a failure report. */
  internal: string[];
}

interface FakeHttpRequest {
  method: string;
  url: string;
  headers: Record<string, string>;
  body?: unknown;
}
interface FakeHttpResponse {
  status: number;
  headers?: Record<string, string>;
  body?: unknown;
}

/** Shared with testing.ts through Symbol.for("iota.testing.state"). */
interface TestingState {
  cases: { name: string; fn: () => unknown; timeoutMs?: number }[];
  http: { request(request: FakeHttpRequest): Promise<FakeHttpResponse> } | null;
}

interface CaseResult {
  name: string;
  file: string;
  passed: boolean;
  message?: string;
  durationMs: number;
  location?: { file: string; line: number; column: number };
  frames?: string[];
}

const state: TestingState = ((globalThis as Record<symbol, unknown>)[Symbol.for("iota.testing.state")] ??= { cases: [], http: null }) as TestingState;

function blocked(what: string): Error {
  return new Error(`outbound ${what} is blocked in synthetic tests: register a fake route (createTestHost({ fakes }))`);
}

function blockNetwork(): void {
  net.Socket.prototype.connect = function connect(): never {
    throw blocked("network connection");
  } as unknown as typeof net.Socket.prototype.connect;
  (dns as { lookup: unknown }).lookup = (): never => {
    throw blocked("dns lookup");
  };
  globalThis.fetch = async (input: string | URL | Request, init?: RequestInit): Promise<Response> => {
    const request = new Request(input, init);
    const text = request.method === "GET" || request.method === "HEAD" ? "" : await request.text();
    let body: unknown;
    if (text !== "") {
      try {
        body = JSON.parse(text);
      } catch {
        body = text;
      }
    }
    const fake = state.http;
    if (fake === null) throw blocked(`http ${request.method} ${request.url}`);
    const res = await fake.request({ method: request.method, url: request.url, headers: Object.fromEntries(request.headers), ...(body === undefined ? {} : { body }) });
    const payload = res.body === undefined ? null : typeof res.body === "string" ? res.body : JSON.stringify(res.body);
    const headers = { ...(res.body !== undefined && typeof res.body !== "string" ? { "content-type": "application/json" } : {}), ...(res.headers ?? {}) };
    return new Response(res.status === 204 || res.status === 304 ? null : payload, { status: res.status, headers });
  };
}

// ---- source-mapped failures: stacks point at the author's .ts, never at the (deleted) temp bundle

interface Mapped {
  map: SourceMap;
  base: string;
}
const bundles = new Map<string, Mapped>();
const cwd = process.cwd();

function register(file: { path: string; map: string; base: string }): void {
  try {
    const payload = JSON.parse(readFileSync(file.map, "utf8")) as ConstructorParameters<typeof SourceMap>[0];
    bundles.set(file.path, { map: new SourceMap(payload), base: file.base });
  } catch {
    // A bundle without a map keeps its generated positions.
  }
}

const FRAME = /^\s*at (?:(.*?) \()?(.+?):(\d+):(\d+)\)?$/;
let internalDirs: string[] = [];
const isInternal = (file: string): boolean => file.startsWith("node:") || internalDirs.some((d) => file.startsWith(d));

interface Frame {
  text: string;
  location?: { file: string; line: number; column: number };
  author: boolean;
}

function mapFrame(line: string): Frame | undefined {
  const m = FRAME.exec(line);
  if (m === null) return undefined;
  const [, fn, rawFile = "", rawLine = "0", rawColumn = "0"] = m;
  const file = rawFile.startsWith("file://") ? fileURLToPath(rawFile) : rawFile;
  const bundle = bundles.get(file);
  if (bundle === undefined) {
    if (isInternal(file)) return undefined;
    const rel = relative(cwd, file);
    const inside = !rel.startsWith("..") && !rel.includes("node_modules");
    return { text: `${fn === undefined ? "" : `${fn} `}${inside ? rel : file}:${rawLine}:${rawColumn}`, author: false };
  }
  const entry = bundle.map.findEntry(Number(rawLine) - 1, Number(rawColumn) - 1);
  const source = (entry as { originalSource?: string }).originalSource;
  if (source === undefined) return undefined;
  const abs = source.includes(":") ? source : resolve(bundle.base, source);
  const rel = source.includes(":") ? source : relative(cwd, abs);
  if (isInternal(abs)) return undefined;
  const authored = !source.includes(":") && !rel.startsWith("..") && !rel.split(/[\\/]/).includes("node_modules");
  const at = { file: rel, line: (entry as { originalLine: number }).originalLine + 1, column: (entry as { originalColumn: number }).originalColumn + 1 };
  return { text: `${fn === undefined ? "" : `${fn} `}${at.file}:${at.line}:${at.column}`, ...(authored ? { location: at } : {}), author: authored };
}

function failure(err: unknown): Pick<CaseResult, "message" | "location" | "frames"> {
  if (!(err instanceof Error)) return { message: String(err) };
  const frames = (err.stack ?? "").split("\n").map(mapFrame).filter((f): f is Frame => f !== undefined);
  const first = frames.find((f) => f.location !== undefined)?.location;
  const generic = err.name === "Error";
  const code = (err as { code?: unknown }).code;
  const head = generic ? err.message : `${err.name}${typeof code === "string" ? ` [${code}]` : ""}: ${err.message}`;
  return {
    message: head,
    ...(first === undefined ? {} : { location: first }),
    ...(frames.length === 0 ? {} : { frames: frames.slice(0, 8).map((f) => f.text) }),
  };
}

const describe = (err: unknown): string => (err instanceof Error ? (err.stack ?? err.message).split("\n").slice(0, 10).join("\n") : String(err));

async function runCase(file: string, c: TestingState["cases"][number], timeout: number): Promise<CaseResult> {
  const started = performance.now();
  state.http = null;
  let timer: NodeJS.Timeout | undefined;
  try {
    await Promise.race([
      Promise.resolve().then(() => c.fn()),
      new Promise<never>((_, reject) => {
        timer = setTimeout(() => reject(new Error(`test timed out after ${c.timeoutMs ?? timeout}ms`)), c.timeoutMs ?? timeout);
      }),
    ]);
    return { name: c.name, file, passed: true, durationMs: Math.round(performance.now() - started) };
  } catch (err) {
    return { name: c.name, file, passed: false, ...failure(err), durationMs: Math.round(performance.now() - started) };
  } finally {
    clearTimeout(timer);
    state.http = null;
  }
}

async function main(): Promise<CaseResult[]> {
  const config = JSON.parse(process.argv[2] ?? "{}") as Config;
  await import(config.loader);
  internalDirs = config.internal;
  blockNetwork();
  const results: CaseResult[] = [];
  for (const { file, path, map, base } of config.files) {
    state.cases.length = 0;
    register({ path, map, base });
    try {
      await import(pathToFileURL(path).href);
    } catch (err) {
      results.push({ name: file, file, passed: false, ...failure(err), message: `cannot load test file: ${failure(err).message ?? describe(err)}`, durationMs: 0 });
      continue;
    }
    for (const c of state.cases.splice(0)) results.push(await runCase(file, c, config.caseTimeoutMs));
  }
  return results;
}

const log = (...args: unknown[]): void => void process.stderr.write(args.map(String).join(" ") + "\n");
console.log = console.info = console.debug = console.warn = console.error = log;
const write = process.stdout.write.bind(process.stdout);
main()
  .then((cases) => write(JSON.stringify({ cases }) + "\n", () => process.exit(0)))
  .catch((err: unknown) => {
    write(JSON.stringify({ cases: [{ name: "runner", file: "", passed: false, message: describe(err), durationMs: 0 }] }) + "\n", () => process.exit(0));
  });

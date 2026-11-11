// Child process entry for `test`: imports bundled test files, runs the cases they register through
// `@iota-uz/sdk/runtime/testing`, and blocks outbound network so an unknown effect fails the test
// instead of reaching a real service. The only way out is a fake http route (createTestHost({ fakes })).
import dns from "node:dns";
import net from "node:net";
import { pathToFileURL } from "node:url";

interface Config {
  loader: string;
  files: { file: string; path: string }[];
  caseTimeoutMs: number;
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

const describe = (err: unknown): string => {
  if (err instanceof Error) return (err.stack ?? err.message).split("\n").slice(0, 10).join("\n");
  return String(err);
};

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
    return { name: c.name, file, passed: false, message: describe(err), durationMs: Math.round(performance.now() - started) };
  } finally {
    clearTimeout(timer);
    state.http = null;
  }
}

async function main(): Promise<CaseResult[]> {
  const config = JSON.parse(process.argv[2] ?? "{}") as Config;
  await import(config.loader);
  blockNetwork();
  const results: CaseResult[] = [];
  for (const { file, path } of config.files) {
    state.cases.length = 0;
    try {
      await import(pathToFileURL(path).href);
    } catch (err) {
      results.push({ name: file, file, passed: false, message: `cannot load test file: ${describe(err)}`, durationMs: 0 });
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

// The driver of a Code node run, evaluated in every fresh sandbox before the code. It builds `env`, calls
// `__mod.default(ctx, env)`, waits for it and reports the result to the host exactly once.
//
// It captures kafig's `host` bridge and removes it from the global object before the code runs, so the
// code reaches the host only through `env`. That lets the driver bound what the code can queue for the host
// (fetch count, request bytes in flight, log lines and their length) before anything crosses into Go
// memory. The host still validates every call (see host.go): the driver is not the boundary of trust, only
// the bound on what a flood of calls can make the host hold.
(() => {
  "use strict";
  const G = globalThis;
  const H = G.host;
  delete G.host;
  const { freeze, keys, defineProperty } = Object;
  const parse = JSON.parse;
  const stringify = JSON.stringify;
  const isArray = Array.isArray;
  const Str = String;
  const ErrorCtor = Error;
  const TypeErrorCtor = TypeError;
  const rpc = H.rpc;
  const rpcSync = H.rpcSync;

  // Request bytes that may wait in the host's queue at once, across all fetches of the run.
  const MAX_QUEUED_REQUEST_BYTES = 16 << 20;
  const MAX_REQUEST_BODY = 10 << 20;

  const deepFreeze = (v) => {
    if (v !== null && typeof v === "object") {
      for (const k of keys(v)) deepFreeze(v[k]);
      freeze(v);
    }
    return v;
  };

  const format = (a) => {
    if (typeof a === "string") return a;
    try {
      if (a instanceof ErrorCtor) {
        const head = Str(a.name) + ": " + Str(a.message);
        return a.stack ? head + "\n" + Str(a.stack) : head;
      }
    } catch (_) {}
    try {
      const s = stringify(a, (_, v) => (typeof v === "bigint" ? Str(v) : v));
      if (s !== undefined) return s;
    } catch (_) {}
    try {
      return Str(a);
    } catch (_) {
      return "[unprintable]";
    }
  };

  const errorInfo = (e) => {
    const info = { name: "Error", message: "", stack: "" };
    try {
      if (e !== null && (typeof e === "object" || typeof e === "function")) {
        if (e.name !== undefined) info.name = Str(e.name);
        if (e.message !== undefined) info.message = Str(e.message);
        if (e.stack !== undefined) info.stack = Str(e.stack);
        if (info.message === "" && !(e instanceof ErrorCtor)) info.message = format(e);
      } else {
        info.message = format(e);
      }
    } catch (_) {
      info.message = "[unprintable exception]";
    }
    return info;
  };

  class NetworkBlocked extends ErrorCtor {
    constructor(message) {
      super(message);
      this.name = "NetworkBlocked";
    }
  }

  // The stack of a fetch error is the stack of the call: after the await only the driver is on it.
  const fail = (err, site) => {
    try {
      defineProperty(err, "stack", { value: site.stack, writable: true, configurable: true });
    } catch (_) {}
    return err;
  };

  const response = (r) => {
    // The host keeps the body until it is read, so unread bodies never wait in the host's queue.
    let body;
    const text = () => {
      if (body === undefined) body = r.bodyId ? Str(rpcSync("body", r.bodyId)) : "";
      return body;
    };
    return freeze({
      status: r.status,
      statusText: Str(r.statusText),
      ok: r.status >= 200 && r.status < 300,
      headers: freeze(r.headers || {}),
      url: Str(r.url),
      redirected: r.redirected === true,
      text: async () => text(),
      json: async () => parse(text()),
    });
  };

  const makeEnv = (secrets, limits) => {
    let fetches = 0;
    let queuedBytes = 0;
    let logLines = 0;

    const fetch = async (url, init) => {
      const site = new ErrorCtor();
      if (++fetches > limits.maxFetches) {
        throw fail(new TypeErrorCtor("fetch failed: at most " + limits.maxFetches + " fetches per run"), site);
      }
      const req = { url: Str(url) };
      if (init !== undefined && init !== null) {
        if (init.method !== undefined) req.method = Str(init.method);
        if (init.headers !== undefined && init.headers !== null) {
          const h = {};
          for (const k of keys(init.headers)) h[Str(k)] = Str(init.headers[k]);
          req.headers = h;
        }
        if (init.body !== undefined && init.body !== null) req.body = Str(init.body);
        if (init.redirect !== undefined) req.redirect = Str(init.redirect);
      }
      if (req.body && req.body.length > MAX_REQUEST_BODY) {
        throw fail(new TypeErrorCtor("fetch failed: request body is too long"), site);
      }
      const size = req.url.length + (req.body ? req.body.length : 0);
      if (queuedBytes + size > MAX_QUEUED_REQUEST_BYTES) {
        throw fail(new TypeErrorCtor("fetch failed: too much request data in flight"), site);
      }
      queuedBytes += size;
      let r;
      try {
        r = await rpc("fetch", req);
      } finally {
        queuedBytes -= size;
      }
      if (r.blocked) throw fail(new NetworkBlocked(Str(r.blocked)), site);
      if (r.error) throw fail(new TypeErrorCtor("fetch failed: " + Str(r.error)), site);
      return response(r);
    };

    return freeze({
      fetch,
      secrets: freeze(secrets),
      log: (...args) => {
        if (logLines > limits.maxLogLines) return;
        logLines++;
        let line = args.map(format).join(" ");
        if (line.length > limits.maxLogChars) line = line.slice(0, limits.maxLogChars);
        rpcSync("log", [line]);
      },
      crypto: freeze({
        randomUUID: () => rpcSync("uuid", null),
        digest: async (alg, data) => rpcSync("digest", { alg: Str(alg), data: Str(data) }),
      }),
    });
  };

  const report = (r) => {
    rpcSync("__result", r);
  };

  const run = async (mod, ctxJSON, secretsJSON, limitsJSON) => {
    const limits = parse(limitsJSON);
    let res;
    try {
      const fn = mod !== null && typeof mod === "object" ? mod.default : undefined;
      if (typeof fn !== "function") {
        throw new TypeErrorCtor("the code must export default async function run(ctx, env)");
      }
      res = await fn(deepFreeze(parse(ctxJSON)), makeEnv(parse(secretsJSON), limits));
    } catch (e) {
      report({ error: errorInfo(e) });
      return;
    }
    try {
      if (res === null || typeof res !== "object" || isArray(res)) {
        report({ invalid: "run must return { outcome, output? }, got " + format(res).slice(0, 200) });
        return;
      }
      const outcome = res.outcome;
      const output = res.output;
      let out = null;
      if (output !== undefined) {
        try {
          out = stringify(output);
        } catch (e) {
          report({ invalid: "output is not JSON: " + errorInfo(e).message });
          return;
        }
        if (out === undefined) {
          report({ invalid: "output is not JSON: " + typeof output });
          return;
        }
        if (out.length > limits.maxOutputBytes) {
          report({ invalid: "output is at least " + out.length + " bytes, at most " + limits.maxOutputBytes });
          return;
        }
      }
      report({ outcome: typeof outcome === "string" ? outcome : null, output: out });
    } catch (e) {
      report({ error: errorInfo(e) });
    }
  };

  defineProperty(G, "__codeRun", { value: freeze(run), writable: false, configurable: false, enumerable: false });
})();

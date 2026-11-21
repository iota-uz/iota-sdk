// The driver of a Code node run, evaluated in every fresh sandbox before the code. It builds `env`, calls
// `__mod.default(ctx, env)`, waits for it and reports the result to the host exactly once. The intrinsics it
// uses are captured here, before the code can replace them. Nothing here is a security boundary: the host
// validates every call (see host.go).
(() => {
  "use strict";
  const H = host;
  const G = globalThis;
  const { freeze, keys, defineProperty } = Object;
  const parse = JSON.parse;
  const stringify = JSON.stringify;
  const isArray = Array.isArray;
  const Str = String;
  const ErrorCtor = Error;
  const TypeErrorCtor = TypeError;
  const rpc = H.rpc;
  const rpcSync = H.rpcSync;

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

  const response = (r) => {
    const body = Str(r.body);
    return freeze({
      status: r.status,
      statusText: r.statusText,
      ok: r.status >= 200 && r.status < 300,
      headers: freeze(r.headers),
      url: r.url,
      redirected: r.redirected,
      text: async () => body,
      json: async () => parse(body),
    });
  };

  const fetch = async (url, init) => {
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
    const r = await rpc("fetch", req);
    if (r.blocked) throw new NetworkBlocked(Str(r.blocked));
    if (r.error) throw new TypeErrorCtor("fetch failed: " + Str(r.error));
    return response(r);
  };

  const makeEnv = (secrets) =>
    freeze({
      fetch,
      secrets: freeze(secrets),
      log: (...args) => {
        rpcSync("log", args.map(format));
      },
      crypto: freeze({
        randomUUID: () => rpcSync("uuid", null),
        digest: async (alg, data) => rpcSync("digest", { alg: Str(alg), data: Str(data) }),
      }),
    });

  const report = (r) => {
    rpcSync("__result", r);
  };

  const run = async (mod, ctxJSON, secretsJSON) => {
    let res;
    try {
      const fn = mod !== null && typeof mod === "object" ? mod.default : undefined;
      if (typeof fn !== "function") {
        throw new TypeErrorCtor("the code must export default async function run(ctx, env)");
      }
      res = await fn(deepFreeze(parse(ctxJSON)), makeEnv(parse(secretsJSON)));
    } catch (e) {
      report({ error: errorInfo(e) });
      return;
    }
    try {
      if (res === null || typeof res !== "object" || isArray(res)) {
        report({ invalid: "run must return { outcome, output? }, got " + format(res) });
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
      }
      report({ outcome: typeof outcome === "string" ? outcome : null, output: out });
    } catch (e) {
      report({ error: errorInfo(e) });
    }
  };

  defineProperty(G, "__codeRun", { value: freeze(run), writable: false, configurable: false, enumerable: false });
})();

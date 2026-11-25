// The driver of a Code node run, evaluated in every fresh sandbox before the code. It builds `env`, calls
// `__mod.default(ctx, env)`, waits for it and reports the result to the host exactly once.
//
// It captures kafig's `host` bridge and removes it from the global object before the code runs, so the
// code reaches the host only through `env`. kafig's bridge serializes call parameters with the global JSON
// and queues async calls in Go before any check, so the driver also freezes JSON, and an async call carries
// nothing the code sizes: a fetch request is first staged with a synchronous call, where Go admits its exact
// bytes against the run's budget, and the async fetch carries only the staging ticket. The host validates
// every call (see host.go); the driver only keeps the code's calls to small, counted ones.
(() => {
  "use strict";
  const G = globalThis;
  const H = G.host;
  delete G.host;
  const { freeze, keys, defineProperty, create } = Object;
  const JSONObj = G.JSON;
  freeze(JSONObj);
  defineProperty(G, "JSON", { value: JSONObj, writable: false, configurable: false, enumerable: false });
  const parse = JSONObj.parse;
  const stringify = JSONObj.stringify;
  const isArray = Array.isArray;
  const Str = String;
  const ErrorCtor = Error;
  const TypeErrorCtor = TypeError;
  const rpc = H.rpc;
  const rpcSync = H.rpcSync;


  // What the bridge serializes is a primitive or a null-prototype object of primitives: no toJSON or other
  // prototype property the code changed takes part in it.
  const bare = (o) => {
    const out = create(null);
    const names = keys(o);
    for (let i = 0; i < names.length; i++) {
      const v = o[names[i]];
      out[names[i]] = v !== null && typeof v === "object" ? bare(v) : v;
    }
    return out;
  };

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

  class ConnectionError extends ErrorCtor {
    constructor(message, code, retryable) {
      super(message);
      this.name = "ConnectionError";
      this.code = code;
      this.retryable = retryable === true;
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

  const makeEnv = (secrets, limits, conns) => {
    let fetches = 0;
    let logLines = 0;

    // env.connections.<name>.<method>(path, body?): the host adds the connection's authorization; the
    // request is staged as a fetch is, and counts as one.
    const connectionCall = (name, method) => async (path, body) => {
      const site = new ErrorCtor();
      if (++fetches > limits.maxFetches) {
        throw fail(new TypeErrorCtor("connection call failed: at most " + limits.maxFetches + " fetches per run"), site);
      }
      const req = create(null);
      req.connection = name;
      req.method = method;
      req.path = Str(path);
      if (body !== undefined && body !== null) {
        if (typeof body === "string") {
          req.body = body;
        } else {
          let text;
          try {
            text = stringify(body);
          } catch (e) {
            throw fail(new TypeErrorCtor("connection call failed: the body is not JSON"), site);
          }
          if (text !== undefined) req.body = Str(text);
        }
      }
      let ticket;
      try {
        ticket = rpcSync("fetchStage", stringify(req));
      } catch (e) {
        throw fail(new TypeErrorCtor("connection call failed: " + Str(e && e.message)), site);
      }
      const r = await rpc("connection", ticket);
      if (r.refused) throw fail(new ConnectionError(Str(r.refused), Str(r.code), r.retryable), site);
      if (r.blocked) throw fail(new NetworkBlocked(Str(r.blocked)), site);
      if (r.error) throw fail(new TypeErrorCtor("connection call failed: " + Str(r.error)), site);
      return response(r);
    };

    // env.connections.<name>.fetch(pathOrURL, init?): fetch on the connection's API; the host adds the
    // authorization over any header of the same name.
    const connectionFetch = (name) => async (path, init) => {
      const site = new ErrorCtor();
      if (++fetches > limits.maxFetches) {
        throw fail(new TypeErrorCtor("connection fetch failed: at most " + limits.maxFetches + " fetches per run"), site);
      }
      const req = create(null);
      req.connection = name;
      req.fetch = true;
      req.path = Str(path);
      if (init !== undefined && init !== null) {
        if (init.method !== undefined) req.httpMethod = Str(init.method);
        if (init.headers !== undefined && init.headers !== null) {
          const h = create(null);
          const names = keys(init.headers);
          for (let i = 0; i < names.length; i++) h[Str(names[i])] = Str(init.headers[names[i]]);
          req.headers = h;
        }
        if (init.body !== undefined && init.body !== null) req.body = Str(init.body);
      }
      let ticket;
      try {
        ticket = rpcSync("fetchStage", stringify(req));
      } catch (e) {
        throw fail(new TypeErrorCtor("connection fetch failed: " + Str(e && e.message)), site);
      }
      const r = await rpc("connection", ticket);
      if (r.refused) throw fail(new ConnectionError(Str(r.refused), Str(r.code), r.retryable), site);
      if (r.blocked) throw fail(new NetworkBlocked(Str(r.blocked)), site);
      if (r.error) throw fail(new TypeErrorCtor("connection fetch failed: " + Str(r.error)), site);
      return response(r);
    };

    // env.connections.<name>.<operation>(args?): a Go operation; the argument and the result are JSON.
    const connectionOperation = (name, operation) => async (args) => {
      const site = new ErrorCtor();
      if (++fetches > limits.maxFetches) {
        throw fail(new TypeErrorCtor("connection call failed: at most " + limits.maxFetches + " fetches per run"), site);
      }
      const req = create(null);
      req.connection = name;
      req.operation = operation;
      if (args !== undefined && args !== null) {
        let text;
        try {
          text = stringify(args);
        } catch (e) {
          throw fail(new TypeErrorCtor("connection call failed: the arguments are not JSON"), site);
        }
        if (text !== undefined) req.args = Str(text);
      }
      let ticket;
      try {
        ticket = rpcSync("fetchStage", stringify(req));
      } catch (e) {
        throw fail(new TypeErrorCtor("connection call failed: " + Str(e && e.message)), site);
      }
      const r = await rpc("operation", ticket);
      if (r.refused) throw fail(new ConnectionError(Str(r.refused), Str(r.code), r.retryable), site);
      if (r.blocked) throw fail(new NetworkBlocked(Str(r.blocked)), site);
      if (r.error) throw fail(new TypeErrorCtor("connection call failed: " + Str(r.error)), site);
      return r.value === undefined ? null : r.value;
    };

    const connections = create(null);
    const connNames = keys(conns);
    for (let i = 0; i < connNames.length; i++) {
      const name = connNames[i];
      const spec = conns[name];
      const c = create(null);
      if (spec.baseUrl) c.baseUrl = Str(spec.baseUrl);
      for (let j = 0; j < spec.methods.length; j++) c[spec.methods[j]] = connectionCall(name, spec.methods[j]);
      for (let j = 0; j < spec.operations.length; j++) c[spec.operations[j]] = connectionOperation(name, spec.operations[j]);
      if (spec.fetch) c.fetch = connectionFetch(name);
      connections[name] = freeze(c);
    }

    const fetch = async (url, init) => {
      const site = new ErrorCtor();
      if (++fetches > limits.maxFetches) {
        throw fail(new TypeErrorCtor("fetch failed: at most " + limits.maxFetches + " fetches per run"), site);
      }
      // The request is built from null-prototype objects and primitive strings and serialized once here,
      // so nothing the code changed in the prototypes or built-ins takes part in what reaches the host.
      const req = create(null);
      req.url = Str(url);
      if (init !== undefined && init !== null) {
        if (init.method !== undefined) req.method = Str(init.method);
        if (init.headers !== undefined && init.headers !== null) {
          const h = create(null);
          const names = keys(init.headers);
          for (let i = 0; i < names.length; i++) h[Str(names[i])] = Str(init.headers[names[i]]);
          req.headers = h;
        }
        if (init.body !== undefined && init.body !== null) req.body = Str(init.body);
        if (init.redirect !== undefined) req.redirect = Str(init.redirect);
      }
      let ticket;
      try {
        ticket = rpcSync("fetchStage", stringify(req));
      } catch (e) {
        throw fail(new TypeErrorCtor("fetch failed: " + Str(e && e.message)), site);
      }
      const r = await rpc("fetch", ticket);
      if (r.blocked) throw fail(new NetworkBlocked(Str(r.blocked)), site);
      if (r.error) throw fail(new TypeErrorCtor("fetch failed: " + Str(r.error)), site);
      return response(r);
    };

    return freeze({
      fetch,
      connections: freeze(connections),
      secrets: freeze(secrets),
      log: (...args) => {
        if (logLines > limits.maxLogLines) return;
        logLines++;
        let line = "";
        for (let i = 0; i < args.length; i++) line += (i > 0 ? " " : "") + format(args[i]);
        if (line.length > limits.maxLogChars) line = Str(line.slice(0, limits.maxLogChars));
        rpcSync("log", Str(line));
      },
      crypto: freeze({
        randomUUID: () => rpcSync("uuid", null),
        digest: async (alg, data) => rpcSync("digest", bare({ alg: Str(alg), data: Str(data) })),
      }),
    });
  };

  const report = (r) => {
    rpcSync("__result", bare(r));
  };

  const run = async (mod, ctxJSON, secretsJSON, limitsJSON, connsJSON) => {
    const limits = parse(limitsJSON);
    let res;
    try {
      const fn = mod !== null && typeof mod === "object" ? mod.default : undefined;
      if (typeof fn !== "function") {
        throw new TypeErrorCtor("the code must export default async function run(ctx, env)");
      }
      res = await fn(deepFreeze(parse(ctxJSON)), makeEnv(parse(secretsJSON), limits, parse(connsJSON)));
    } catch (e) {
      report({ error: errorInfo(e) });
      return;
    }
    try {
      if (res === null || typeof res !== "object" || isArray(res)) {
        report({ invalid: "run must return { outcome, output? }, got " + Str(format(res)).slice(0, 200) });
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

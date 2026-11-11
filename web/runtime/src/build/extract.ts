// Child process entry: imports the built contracts/server bundles and the Node-safe client bundle
// and extracts the manifest. Runs isolated from the build worker so package code (top-level effects,
// failing imports) can neither crash nor influence it.
import { pathToFileURL } from "node:url";

import { collectManifest, ManifestError } from "../manifest.ts";
import type { CapabilitiesJSON, ManifestJSON } from "../model.ts";

interface Config {
  loader: string;
  namespace: string;
  contracts?: string;
  server?: string;
  client?: string;
  capabilities?: CapabilitiesJSON;
  dependencies?: Record<string, string>;
}

type Outcome =
  | { ok: true; manifest: ManifestJSON }
  | { ok: false; kind: "manifest"; problems: readonly string[] }
  | { ok: false; kind: "import"; graph: string; message: string };

type Namespace = Readonly<Record<string, unknown>>;

async function main(): Promise<Outcome> {
  const config = JSON.parse(process.argv[2] ?? "{}") as Config;
  await import(config.loader);
  const load = async (graph: string, file: string | undefined): Promise<Namespace | Outcome> => {
    if (file === undefined) return {};
    try {
      return (await import(pathToFileURL(file).href)) as Namespace;
    } catch (err) {
      const e = err as Error;
      return { ok: false, kind: "import", graph, message: e.stack ?? e.message ?? String(err) };
    }
  };
  const contracts = await load("contracts", config.contracts);
  const server = await load("server", config.server);
  const client = await load("client", config.client);
  for (const graph of [contracts, server, client]) {
    if ("ok" in graph && graph.ok === false) return graph as Outcome;
  }
  try {
    return {
      ok: true,
      manifest: collectManifest({
        namespace: config.namespace,
        contracts: contracts as Namespace,
        server: server as Namespace,
        client: client as Namespace,
        ...(config.capabilities === undefined ? {} : { capabilities: config.capabilities }),
        ...(config.dependencies === undefined ? {} : { dependencies: config.dependencies }),
      }),
    };
  } catch (err) {
    if (err instanceof ManifestError) return { ok: false, kind: "manifest", problems: err.problems };
    throw err;
  }
}

// Package code may print; only the final line is the protocol.
const log = (...args: unknown[]): void => void process.stderr.write(args.map(String).join(" ") + "\n");
console.log = console.info = console.debug = console.warn = console.error = log;
const write = process.stdout.write.bind(process.stdout);
main()
  .then((outcome) => write(JSON.stringify(outcome) + "\n", () => process.exit(0)))
  .catch((err: unknown) => {
    write(JSON.stringify({ ok: false, kind: "import", graph: "extract", message: err instanceof Error ? (err.stack ?? err.message) : String(err) }) + "\n", () => process.exit(0));
  });

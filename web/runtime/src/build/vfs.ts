// A read-only virtual file system layered over the package directory: in-memory files (kit modules,
// the synthetic SDK manifest) and mounted real directories (SDK typings, vendored libs) appear at
// virtual paths below <dir>/node_modules, so module resolution behaves as if they were installed.
import { existsSync, readFileSync, readdirSync, statSync } from "node:fs";
import { join, posix } from "node:path";

import type { Layout } from "./layout.ts";
import type { KitPayload } from "./protocol.ts";

interface Mount {
  /** Virtual absolute directory (posix, no trailing slash). */
  virtual: string;
  real: string;
}

const SDK_PACKAGE = {
  name: "@iota-uz/sdk",
  version: "0.0.0",
  type: "module",
  exports: {
    "./runtime": { types: "./runtime/index.d.ts" },
    "./runtime/server": { types: "./runtime/server.d.ts" },
    "./runtime/client": { types: "./runtime/client.d.ts" },
    "./runtime/testing": { types: "./runtime/testing.d.ts" },
  },
};

export class VirtualFS {
  readonly dir: string;
  private readonly files = new Map<string, string>();
  private readonly mounts: Mount[] = [];
  private readonly virtualDirs = new Set<string>();

  constructor(dir: string, layout: Layout, kit: KitPayload) {
    this.dir = dir;
    const modules = posix.join(dir, "node_modules");
    this.addFile(posix.join(modules, "@iota-uz/sdk/package.json"), JSON.stringify(SDK_PACKAGE));
    this.mounts.push({ virtual: posix.join(modules, "@iota-uz/sdk/runtime"), real: layout.sdkTypes });
    // Vendored typings fill in only what the package did not install itself.
    this.mountUnlessInstalled(posix.join(modules, "solid-js"), layout.vendor.solid);
    this.mountUnlessInstalled(posix.join(modules, "@types/node"), layout.vendor.nodeTypes);
    for (const [name, files] of Object.entries(kit.modules)) {
      for (const [path, text] of Object.entries(files)) this.addFile(posix.join(modules, name, path), text);
    }
  }

  private mountUnlessInstalled(virtual: string, real: string): void {
    if (!existsSync(virtual)) this.mounts.push({ virtual, real });
  }

  addFile(path: string, text: string): void {
    this.files.set(path, text);
    for (let d = posix.dirname(path); d !== "/" && !this.virtualDirs.has(d); d = posix.dirname(d)) this.virtualDirs.add(d);
  }

  /** Real path backing `path`, when it lies in a mount. */
  private mapped(path: string): string | undefined {
    for (const m of this.mounts) {
      if (path === m.virtual) return m.real;
      if (path.startsWith(m.virtual + "/")) return join(m.real, path.slice(m.virtual.length + 1));
    }
    return undefined;
  }

  private isMountAncestor(path: string): boolean {
    return this.mounts.some((m) => m.virtual.startsWith(path + "/"));
  }

  fileExists(path: string): boolean {
    if (this.files.has(path)) return true;
    const real = this.mapped(path) ?? path;
    try {
      return statSync(real).isFile();
    } catch {
      return false;
    }
  }

  readFile(path: string): string | undefined {
    const inMemory = this.files.get(path);
    if (inMemory !== undefined) return inMemory;
    try {
      return readFileSync(this.mapped(path) ?? path, "utf8");
    } catch {
      return undefined;
    }
  }

  directoryExists(path: string): boolean {
    if (this.virtualDirs.has(path) || this.isMountAncestor(path)) return true;
    try {
      return statSync(this.mapped(path) ?? path).isDirectory();
    } catch {
      return false;
    }
  }

  getDirectories(path: string): string[] {
    const names = new Set<string>();
    try {
      for (const e of readdirSync(this.mapped(path) ?? path, { withFileTypes: true })) if (e.isDirectory()) names.add(e.name);
    } catch {
      // not a real directory
    }
    const prefix = path.endsWith("/") ? path : path + "/";
    for (const d of this.virtualDirs) if (d.startsWith(prefix)) names.add(d.slice(prefix.length).split("/")[0] ?? "");
    for (const m of this.mounts) if (m.virtual.startsWith(prefix)) names.add(m.virtual.slice(prefix.length).split("/")[0] ?? "");
    names.delete("");
    return [...names].sort();
  }

  /** Virtual file contents by absolute path, for kit declaration emit. */
  inMemoryFiles(): ReadonlyMap<string, string> {
    return this.files;
  }
}

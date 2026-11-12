// Where the shipped CLI finds the rest of the distribution, and how it talks to the build worker.
import { spawn } from 'node:child_process'
import { existsSync, readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

import type { KitModules, Request, Response } from '../build/protocol.ts'

/** A failure that is not about the author's package: missing files, no network, a crashed worker. Exit code 3. */
export class EnvironmentError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'EnvironmentError'
  }
}

export interface Distribution {
  /** `<sdk>/dist/runtime`: the directory holding `cli/`, `build/`, `host/` and the SDK modules. */
  readonly root: string
  readonly worker: string
  /** The catalog snapshot of the SDK's own natives (core, crm). */
  readonly sdkCatalog: string
  /** The `@iota-uz/sdk` version this CLI ships with. */
  readonly sdkVersion: string
}

export function locateDistribution(moduleURL: string): Distribution {
  const root = dirname(dirname(fileURLToPath(moduleURL)))
  let version = '0.0.0'
  for (let dir = root, i = 0; i < 4; dir = dirname(dir), i++) {
    const file = join(dir, 'package.json')
    if (!existsSync(file)) continue
    const pkg = JSON.parse(readFileSync(file, 'utf8')) as { name?: string; version?: string }
    if (pkg.name === '@iota-uz/sdk' && pkg.version !== undefined) {
      version = pkg.version
      break
    }
  }
  return { root, worker: join(root, 'build', 'worker.mjs'), sdkCatalog: join(root, 'catalog', 'sdk-natives.json'), sdkVersion: version }
}

/** One worker process per operation: a JSON request on stdin, a JSON response on stdout. */
export function runWorker<R extends Response>(dist: Distribution, request: Request, env: NodeJS.ProcessEnv): Promise<R> {
  if (!existsSync(dist.worker)) throw new EnvironmentError(`build worker not found at ${dist.worker}; reinstall @iota-uz/sdk`)
  return new Promise((resolve, reject) => {
    const child = spawn(process.execPath, ['--no-warnings', dist.worker], { env, stdio: ['pipe', 'pipe', 'pipe'] })
    let stdout = ''
    let stderr = ''
    child.stdout.on('data', (b: Buffer) => (stdout += b.toString('utf8')))
    child.stderr.on('data', (b: Buffer) => (stderr += b.toString('utf8')))
    child.on('error', (err) => reject(new EnvironmentError(`cannot start node: ${err.message}`)))
    child.on('close', (code) => {
      const line = stdout.trim().split('\n').pop() ?? ''
      try {
        resolve(JSON.parse(line) as R)
      } catch {
        const text = (stderr || stdout).trim()
        const hint = /Cannot find (?:module|package) 'esbuild'/.test(text) ? '\nesbuild is a peer dependency of @iota-uz/sdk: add it to your devDependencies (npm install --save-dev esbuild@^0.25.0).' : ''
        reject(new EnvironmentError(`the build worker exited with ${String(code)}:\n${text.slice(0, 2000)}${hint}`))
      }
    })
    child.stdin.end(JSON.stringify(request))
  })
}

export type { KitModules }

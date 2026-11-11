#!/usr/bin/env node
// Bundles the runtime host entry (web/runtime/src/client-host/entry.ts) with the canonical client host into one
// browser module: the file an application serves as `/assets/runtime-host.js`. This is what a consumer's frontend
// build does with `@iota-uz/sdk/runtime/client-host/entry`; the SDK identity is stamped with the commit the Go
// side of the browser end-to-end suite announces in its host manifest.
//
//   node build-host.mjs <out-file> [source-commit]
import { readFile } from 'node:fs/promises'
import { createRequire } from 'node:module'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const repo = resolve(here, '../../../..')
const require = createRequire(join(repo, 'web/runtime/package.json'))
const esbuild = require('esbuild')

const out = process.argv[2]
const commit = process.argv[3] ?? '0123456789abcdef0123456789abcdef01234567'
if (!out) throw new Error('usage: build-host.mjs <out-file> [source-commit]')

const clientHost = join(repo, 'web/client-host/src')
const placeholder = '__IOTA_SDK_SOURCE_COMMIT__'

await esbuild.build({
  entryPoints: [join(repo, 'web/runtime/src/client-host/entry.ts')],
  outfile: resolve(out),
  bundle: true,
  format: 'esm',
  platform: 'browser',
  target: 'es2022',
  conditions: ['browser', 'import'],
  sourcemap: 'inline',
  logLevel: 'warning',
  // The tsconfig `paths` of src/client-host point `solid-js` at a directory, which skips the package's export
  // conditions and bundles Solid's server build. The browser build must resolve through the conditions.
  tsconfigRaw: '{}',
  plugins: [{
    name: 'iota-host',
    setup(build) {
      // The package alias the SDK's dist resolves through its exports map.
      build.onResolve({ filter: /^@iota-uz\/sdk\/client-host$/ }, () => ({ path: join(clientHost, 'solid-index.ts') }))
      // The identity a published package carries is stamped at release time.
      build.onLoad({ filter: /client-host[\\/]src[\\/]identity\.ts$/ }, async (args) => ({
        contents: (await readFile(args.path, 'utf8')).replace(placeholder, commit),
        loader: 'ts',
      }))
    },
  }],
})
console.log(`built the runtime host bundle into ${resolve(out)}`)

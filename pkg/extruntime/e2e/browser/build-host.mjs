#!/usr/bin/env node
// Bundles the runtime host entry (web/runtime/src/client-host/entry.ts) with the canonical client host into one
// browser module plus lazy chunks: the files an application serves as `/assets/runtime-host.js` and
// `/assets/runtime-host-chunk-*.js`. This is what a consumer's frontend build does with
// `@iota-uz/sdk/runtime/client-host/entry`; the SDK identity is stamped with the commit the Go side of the browser
// end-to-end suite announces in its host manifest.
//
// The UI kit (`@iota-uz/sdk/solid-ui`) is a lazy chunk of this bundle: code splitting is on, so a page that no package
// styles with the kit never downloads it. The kit is the package a consumer's build resolves through the SDK exports; here
// its library build (web/solid-ui, vite) is made first and the string module of its stylesheet is generated from the same
// source the SDK package builds it from (web/solid-ui/src/embedded.css).
//
//   node build-host.mjs <out-file> [source-commit]
import { spawnSync } from 'node:child_process'
import { readFile } from 'node:fs/promises'
import { createRequire } from 'node:module'
import { basename, dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const repo = resolve(here, '../../../..')
const require = createRequire(join(repo, 'web/runtime/package.json'))
const esbuild = require('esbuild')

const out = process.argv[2]
const commit = process.argv[3] ?? '0123456789abcdef0123456789abcdef01234567'
if (!out) throw new Error('usage: build-host.mjs <out-file> [source-commit]')

const clientHost = join(repo, 'web/client-host/src')
const solidUI = join(repo, 'web/solid-ui')
const placeholder = '__IOTA_SDK_SOURCE_COMMIT__'

function run(command, args, cwd) {
  const result = spawnSync(command, args, { cwd, stdio: 'inherit', env: process.env })
  if (result.status !== 0) throw new Error(`${command} ${args.join(' ')} failed (${result.status ?? result.signal})`)
}

// The kit's library build and its embedded utilities sheet (what `pnpm --dir web/solid-ui build:package` produces for the SDK package).
run(join(solidUI, 'node_modules/.bin/vite'), ['build', '--config', 'vite.package.config.ts', '--logLevel', 'warn'], solidUI)
const embeddedCss = join(dirname(resolve(out)), 'kit-embedded.css')
run(join(solidUI, 'node_modules/.bin/tailwindcss'), ['--input', 'src/embedded.css', '--output', embeddedCss, '--minify'], solidUI)

const outFile = resolve(out)
await esbuild.build({
  entryPoints: { [basename(outFile).replace(/\.js$/, '')]: join(repo, 'web/runtime/src/client-host/entry.ts') },
  outdir: dirname(outFile),
  bundle: true,
  splitting: true,
  chunkNames: 'runtime-host-chunk-[hash]',
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
      // The kit and its stylesheet, as the SDK package's exports map names them.
      build.onResolve({ filter: /^@iota-uz\/sdk\/solid-ui$/ }, () => ({ path: join(solidUI, 'package-dist/index.js') }))
      build.onResolve({ filter: /^@iota-uz\/sdk\/solid-ui\/embedded-css$/ }, () => ({ path: 'embedded-css', namespace: 'iota-embedded-css' }))
      build.onLoad({ filter: /.*/, namespace: 'iota-embedded-css' }, async () => ({
        contents: `export default ${JSON.stringify((await readFile(embeddedCss, 'utf8')).trim())}`,
        loader: 'js',
      }))
      // The identity a published package carries is stamped at release time.
      build.onLoad({ filter: /client-host[\\/]src[\\/]identity\.ts$/ }, async (args) => ({
        contents: (await readFile(args.path, 'utf8')).replace(placeholder, commit),
        loader: 'ts',
      }))
    },
  }],
})
console.log(`built the runtime host bundle into ${outFile}`)

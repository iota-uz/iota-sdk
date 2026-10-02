import assert from 'node:assert/strict'
import { mkdtemp, mkdir, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import path from 'node:path'
import test from 'node:test'
import { checkAppletDependencies } from './check-applet-dependencies.mjs'

test('production applet dependencies remain registry-only beside SDK source workspaces', async () => {
  // Falsely green if the wrapper checks no applet files: the forbidden applet
  // manifest and its independently forbidden lock must each fail the real CLI.
  const root = await mkdtemp(path.join(tmpdir(), 'sdk-applet-policy-test-'))
  const web = path.join(root, 'modules', 'example', 'presentation', 'web')
  try {
    await mkdir(path.join(root, '.applets'), { recursive: true })
    await writeFile(path.join(root, '.applets', 'config.toml'), 'version = 2\n')
    await mkdir(web, { recursive: true })
    await writeFile(path.join(root, 'pnpm-lock.yaml'), "importers:\n  web/client-host:\n    devDependencies:\n      '@iota-uz/sdk':\n        specifier: workspace:*\n        version: link:../sdk\n")
    const manifest = spec => JSON.stringify({ name: 'example-web', dependencies: { '@iota-uz/sdk': spec } })
    await writeFile(path.join(web, 'package.json'), manifest('0.4.39'))
    await checkAppletDependencies(root)
    await writeFile(path.join(web, 'package.json'), manifest('workspace:*'))
    await assert.rejects(checkAppletDependencies(root))
    await writeFile(path.join(web, 'package.json'), manifest('0.4.39'))
    await writeFile(path.join(web, 'pnpm-lock.yaml'), "importers:\n  .:\n    dependencies:\n      '@iota-uz/sdk':\n        specifier: link:../../sdk\n        version: link:../../sdk\n")
    await assert.rejects(checkAppletDependencies(root))
  } finally {
    await rm(root, { recursive: true, force: true })
  }
})

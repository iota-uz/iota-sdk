import { cp, mkdtemp, mkdir, readdir, rm } from 'node:fs/promises'
import { execFileSync } from 'node:child_process'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

export async function checkAppletDependencies(root, command = 'applet') {
  const staged = await mkdtemp(path.join(tmpdir(), 'sdk-applet-dependencies-'))
  try {
    await mkdir(path.join(staged, '.applets'))
    await cp(path.join(root, '.applets', 'config.toml'), path.join(staged, '.applets', 'config.toml'))
    for (const module of await readdir(path.join(root, 'modules'), { withFileTypes: true })) {
      if (!module.isDirectory()) continue
      const relative = path.join('modules', module.name, 'presentation', 'web')
      let entries
      try { entries = await readdir(path.join(root, relative)) } catch (error) {
        if (error.code === 'ENOENT') continue
        throw error
      }
      if (!entries.includes('package.json')) continue
      await mkdir(path.join(staged, relative), { recursive: true })
      for (const name of ['package.json', 'pnpm-lock.yaml', 'pnpm-workspace.yaml', '.npmrc']) {
        if (entries.includes(name)) await cp(path.join(root, relative, name), path.join(staged, relative, name))
      }
    }
    // SDK source workspaces consume the package they build; production applets
    // must still pass the external CLI's exact registry dependency policy.
    execFileSync(command, ['deps', 'check'], { cwd: staged, stdio: 'inherit' })
  } finally {
    await rm(staged, { recursive: true, force: true })
  }
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  await checkAppletDependencies(process.cwd())
}

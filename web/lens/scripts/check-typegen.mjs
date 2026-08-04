import { spawnSync } from 'node:child_process'
import { cpSync, mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../..')
const contract = path.join(root, 'web', 'lens', 'src', 'contract')
const temporary = mkdtempSync(path.join(tmpdir(), 'lens-typegen-'))
const snapshot = path.join(temporary, 'contract')
let status = 0

try {
  cpSync(contract, snapshot, { recursive: true })
  const generated = spawnSync('go', ['run', './cmd/lens-typegen'], { cwd: root, env: process.env, stdio: 'inherit' })
  status = generated.status ?? 1
  if (status === 0) {
    const drift = spawnSync('diff', ['-ru', snapshot, contract], { cwd: root, encoding: 'utf8' })
    status = drift.status ?? 1
    if (status !== 0) {
      process.stderr.write(drift.stdout)
      process.stderr.write('Lens contract output drifted; review and keep the regenerated files.\n')
    }
  }
} finally {
  rmSync(temporary, { recursive: true, force: true })
}

process.exitCode = status

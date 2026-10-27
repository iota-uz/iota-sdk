import { spawnSync } from 'node:child_process'
import { cpSync, mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../..')
const contracts = ['contract', 'reports/contract'].map((name) => path.join(root, 'web', 'lens', 'src', name))
const temporary = mkdtempSync(path.join(tmpdir(), 'lens-typegen-'))
const snapshots = contracts.map((_, index) => path.join(temporary, `contract-${index}`))
let status = 0

try {
  contracts.forEach((contract, index) => cpSync(contract, snapshots[index], { recursive: true }))
  const generated = spawnSync('go', ['run', './cmd/lens-typegen'], { cwd: root, env: process.env, stdio: 'inherit' })
  status = generated.status ?? 1
  if (status === 0) {
    for (const [index, contract] of contracts.entries()) {
      const drift = spawnSync('diff', ['-ru', snapshots[index], contract], { cwd: root, encoding: 'utf8' })
      status = status || (drift.status ?? 1)
      if (drift.status !== 0) {
        process.stderr.write(drift.stdout)
        process.stderr.write('Lens contract output drifted; review and keep the regenerated files.\n')
      }
    }
  }
} finally {
  rmSync(temporary, { recursive: true, force: true })
}

process.exitCode = status

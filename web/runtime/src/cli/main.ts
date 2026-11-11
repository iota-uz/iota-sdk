#!/usr/bin/env node
import { run } from './commands.ts'

const color = process.stdout.isTTY === true && process.env.NO_COLOR === undefined
const code = await run(
  process.argv.slice(2),
  {
    cwd: process.cwd(),
    env: process.env,
    color,
    out: (text) => void process.stdout.write(`${text}\n`),
    err: (text) => void process.stderr.write(`${text}\n`),
  },
  import.meta.url,
).catch((err: unknown) => {
  process.stderr.write(`iota-ext: unexpected failure: ${err instanceof Error ? (err.stack ?? err.message) : String(err)}\n`)
  return 3
})
process.exitCode = code

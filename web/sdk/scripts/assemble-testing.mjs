import { cp, mkdir, rm } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
const target = fileURLToPath(new URL('../dist/testing', import.meta.url))
await rm(target, { recursive: true, force: true })
await mkdir(target, { recursive: true })
await cp(fileURLToPath(new URL('../../testing/dist', import.meta.url)), target, { recursive: true })

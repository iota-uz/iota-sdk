import { readdirSync, statSync } from 'node:fs'
import { join } from 'node:path'

/**
 * Every runtime source file under `directory`, tests excluded.
 *
 * Shared by the tests that enforce a cross-cutting convention by reading the
 * source rather than rendering it — the defects those guard against are a new
 * file somewhere doing its own thing, which no rendered test of any one
 * component can see.
 */
export function sourceFiles(directory: string): Array<string> {
  return readdirSync(directory).flatMap((entry) => {
    const path = join(directory, entry)
    if (statSync(path).isDirectory()) return sourceFiles(path)
    return /\.tsx?$/.test(path) && !/\.test\.tsx?$/.test(path) ? [path] : []
  })
}

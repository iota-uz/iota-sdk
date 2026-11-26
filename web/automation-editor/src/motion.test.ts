// Motion lint (#1352): the automation screens take every duration and curve from the motion tokens
// (--motion-*, --ease-*, Tailwind duration-instant|fast|base|slow and ease-out|in|standard). A literal
// `transition: 0.3s ease`, `duration-200` or `cubic-bezier(…)` on a screen fails here. A line may keep its
// own timing only with a `motion-allow: <reason>` note (an indefinite spinner, a gesture's own time).
import { readdirSync, readFileSync, statSync } from 'node:fs'
import path from 'node:path'
import { describe, expect, it } from 'vitest'
import { motionMs } from './motion'

const repo = path.resolve(__dirname, '../../..')
const roots = [
  'web/automation-editor/src',
  'modules/automations/presentation/graphs/web',
  'modules/automations/presentation/integrations/web',
  'modules/automations/presentation/inbox',
  'modules/core/presentation/templates/pages/notification_settings',
]
const skipDirs = new Set(['node_modules', 'dist', 'fixtures', 'browser-tests', 'results'])

function files(dir: string): string[] {
  let out: string[] = []
  for (const name of readdirSync(dir)) {
    const full = path.join(dir, name)
    if (statSync(full).isDirectory()) {
      if (!skipDirs.has(name)) out = out.concat(files(full))
    } else if (/\.(css|tsx|ts|templ)$/.test(name) && !/\.(test|vitest|spec)\.tsx?$|\.generated\.(ts|css)$|_templ\.go$/.test(name)) {
      out.push(full)
    }
  }
  return out
}

/** The offending fragments of one line, or none. */
export function motionViolations(line: string): string[] {
  if (line.includes('motion-allow:')) return []
  const found: string[] = []
  // A CSS transition or animation declaration: no literal time and no curve but a token.
  for (const m of line.matchAll(/\b(?:transition|animation)(?:-duration|-delay|-timing-function)?\s*:\s*([^;}'"`]*)/g)) {
    const value = m[1]!.replace(/var\(--[\w-]+\)/g, '')
    if (/(^|[\s,(])\d*\.?\d+m?s\b/.test(value)) found.push(m[0])
    else if (/\b(?:ease|ease-in|ease-out|ease-in-out|linear|step-start|step-end)\b|cubic-bezier\(/.test(value)) found.push(m[0])
  }
  // Tailwind: a numeric duration or delay, a curve that is not a token, an arbitrary time.
  for (const m of line.matchAll(/(?<![\w-])(?:[\w-]+:)*(?:duration|delay)-(?:\d+|\[[^\]]*\])(?![\w-])/g)) found.push(m[0])
  for (const m of line.matchAll(/(?<![\w-])(?:[\w-]+:)*ease-(?:linear|in-out|\[[^\]]*\])(?![\w-])/g)) found.push(m[0])
  // A JS animation option with a literal time (viewport zoom, element.animate).
  for (const m of line.matchAll(/\bduration:\s*\d+/g)) found.push(m[0])
  return found
}

describe('motion tokens (#1352)', () => {
  it('flags literal timings and lets tokens through', () => {
    expect(motionViolations('.a { transition: opacity 0.3s ease; }')).toHaveLength(1)
    expect(motionViolations('.a { transition: transform 200ms ease-out; }')).toHaveLength(1)
    expect(motionViolations('.a { animation: x var(--motion-base) cubic-bezier(0, 0, 1, 1); }')).toHaveLength(1)
    expect(motionViolations('<div class="transition-colors duration-200">')).toEqual(['duration-200'])
    expect(motionViolations('<div class="motion-safe:duration-[250ms] ease-in-out">')).toHaveLength(2)
    expect(motionViolations('instance.scaleBy(factor, { duration: 150 })')).toHaveLength(1)
    expect(motionViolations('.a { transition: opacity var(--motion-fast) var(--ease-in); }')).toEqual([])
    expect(motionViolations('<div class="transition-opacity duration-base ease-out">')).toEqual([])
    expect(motionViolations('.s { animation: spin 1s linear infinite; } /* motion-allow: spinner */')).toEqual([])
    expect(motionViolations('const durationAmount = 5')).toEqual([])
  })

  it('no screen of the automations uses a duration or curve outside the tokens', () => {
    const problems: string[] = []
    for (const root of roots) {
      for (const file of files(path.join(repo, root))) {
        if (file.endsWith('motion.test.ts')) continue
        readFileSync(file, 'utf8').split('\n').forEach((line, i) => {
          for (const v of motionViolations(line)) problems.push(`${path.relative(repo, file)}:${i + 1}  ${v}`)
        })
      }
    }
    expect(problems).toEqual([])
  })

  it('reads the tokens, which are zero under reduced motion', () => {
    document.documentElement.style.setProperty('--motion-fast', '140ms')
    expect(motionMs('fast')).toBe(140)
    document.documentElement.style.setProperty('--motion-fast', '0ms')
    expect(motionMs('fast')).toBe(0)
    document.documentElement.style.removeProperty('--motion-fast')
  })
})

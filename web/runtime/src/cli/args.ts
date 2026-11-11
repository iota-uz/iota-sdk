// Minimal argument parser: `--flag`, `--key value`, `--key=value`, positionals. Unknown flags are errors.
export class UsageError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'UsageError'
  }
}

export interface FlagSpec {
  /** Flags that take a value. */
  readonly values?: readonly string[]
  /** Flags that are on or off. */
  readonly booleans?: readonly string[]
  /** Flags that may repeat (their values are collected). */
  readonly lists?: readonly string[]
}

export interface Parsed {
  readonly positionals: readonly string[]
  value(name: string): string | undefined
  list(name: string): readonly string[]
  flag(name: string): boolean
}

export function parseArgs(argv: readonly string[], spec: FlagSpec): Parsed {
  const values = new Map<string, string>()
  const lists = new Map<string, string[]>()
  const flags = new Set<string>()
  const positionals: string[] = []
  const takesValue = new Set([...(spec.values ?? []), ...(spec.lists ?? [])])
  const isBoolean = new Set(spec.booleans ?? [])
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i] ?? ''
    if (arg === '--') {
      positionals.push(...argv.slice(i + 1))
      break
    }
    if (!arg.startsWith('--')) {
      positionals.push(arg)
      continue
    }
    const eq = arg.indexOf('=')
    const name = arg.slice(2, eq === -1 ? undefined : eq)
    if (isBoolean.has(name)) {
      if (eq !== -1) throw new UsageError(`--${name} does not take a value`)
      flags.add(name)
      continue
    }
    if (!takesValue.has(name)) throw new UsageError(`unknown option --${name}`)
    let value: string | undefined = eq === -1 ? undefined : arg.slice(eq + 1)
    if (value === undefined) {
      value = argv[++i]
      if (value === undefined || value.startsWith('--')) throw new UsageError(`--${name} needs a value`)
    }
    if (spec.lists?.includes(name) === true) lists.set(name, [...(lists.get(name) ?? []), value])
    else values.set(name, value)
  }
  return {
    positionals,
    value: (name) => values.get(name),
    list: (name) => lists.get(name) ?? [],
    flag: (name) => flags.has(name),
  }
}

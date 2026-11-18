import type { TypeFiles } from './types'

/**
 * Type information for Monaco's TypeScript worker: the developer kit
 * (`@iota-kit/*`, from the immutable catalog snapshot), `@iota-uz/sdk/runtime*`
 * and solid-js, all registered as virtual node_modules files, and the draft's
 * own files as models (so `./contracts` resolves to the draft's file before
 * the package was ever activated).
 */

/** The part of Monaco's TypeScript defaults this module drives. */
export interface TsDefaults {
  setCompilerOptions(options: Record<string, unknown>): void
  setDiagnosticsOptions(options: Record<string, unknown>): void
  setEagerModelSync(value: boolean): void
  addExtraLib(content: string, filePath?: string): { dispose(): void }
}

export interface TsEnums {
  ScriptTarget: { ES2022: number }
  ModuleKind: { ESNext: number }
  ModuleResolutionKind: { NodeJs: number }
  JsxEmit: { Preserve: number }
}

/** modelUri is the URI of a draft file's model; drafts live at the virtual root. */
export function modelUri(path: string): string { return `file:///${path.replace(/^\/+/, '')}` }

/** typeUri is the URI of a typings file below the virtual node_modules. */
export function typeUri(relative: string): string { return `file:///node_modules/${relative.replace(/^\/+/, '')}` }

/** pathOf is the draft path of a model URI, or undefined for non-draft models. */
export function pathOf(uri: string): string | undefined {
  if (!uri.startsWith('file:///') || uri.startsWith('file:///node_modules/')) return undefined
  return uri.slice('file:///'.length)
}

const sdkPaths = ['dist/*/index.d.ts', 'dist/*.d.ts', '*/index.d.ts', '*.d.ts']

export function compilerOptions(ts: TsEnums): Record<string, unknown> {
  return {
    target: ts.ScriptTarget.ES2022,
    module: ts.ModuleKind.ESNext,
    moduleResolution: ts.ModuleResolutionKind.NodeJs,
    jsx: ts.JsxEmit.Preserve,
    jsxImportSource: 'solid-js',
    allowNonTsExtensions: true,
    esModuleInterop: true,
    allowSyntheticDefaultImports: true,
    strict: true,
    skipLibCheck: true,
    noEmit: true,
    lib: ['esnext', 'dom', 'dom.iterable'],
    baseUrl: 'file:///',
    paths: {
      '@iota-uz/sdk/*': sdkPaths.map((p) => `node_modules/@iota-uz/sdk/${p}`),
      '@iota-kit/*': ['node_modules/@iota-kit/*/index.d.ts'],
    },
  }
}

/** configureTypeScript sets the compiler options once per Monaco instance. */
export function configureTypeScript(defaults: readonly TsDefaults[], ts: TsEnums): void {
  for (const d of defaults) {
    d.setCompilerOptions(compilerOptions(ts))
    // Syntax and semantic checks run in the worker; the build worker's check is
    // the authority and its diagnostics are drawn as separate markers.
    d.setDiagnosticsOptions({ noSemanticValidation: false, noSyntaxValidation: false })
    d.setEagerModelSync(true)
  }
}

/**
 * TypeLibraries owns the extra libs registered for one type set (kit or SDK).
 * Loading the same hash again is a no-op; a new hash replaces the files.
 */
export class TypeLibraries {
  private libs: Array<{ dispose(): void }> = []
  private current: string | undefined
  private disposed = false

  constructor(private readonly defaults: readonly TsDefaults[]) {}

  get hash(): string | undefined { return this.current }

  /** set registers the files; returns false when the hash is already loaded. */
  set(files: TypeFiles): boolean {
    if (this.disposed || (this.current !== undefined && this.current === files.hash && files.hash !== '')) return false
    this.clear()
    for (const [rel, content] of Object.entries(files.files)) {
      for (const d of this.defaults) this.libs.push(d.addExtraLib(content, typeUri(rel)))
    }
    this.current = files.hash
    return true
  }

  clear(): void {
    for (const l of this.libs.splice(0)) l.dispose()
    this.current = undefined
  }

  dispose(): void {
    this.clear()
    this.disposed = true
  }
}

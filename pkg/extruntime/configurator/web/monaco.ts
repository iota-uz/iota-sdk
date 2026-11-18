import type * as Monaco from 'monaco-editor/esm/vs/editor/editor.api'
import { configureTypeScript, type TsDefaults, type TsEnums } from './monacoTypes'

export type MonacoApi = typeof Monaco

let loading: Promise<MonacoApi> | undefined

/**
 * loadMonaco fetches the editor on first use. The editor, its language
 * services and the three workers are separate lazy chunks: nothing of Monaco
 * is in the screen's own bundle, and a user who only reads versions never
 * downloads it. A failed load is not cached, so Retry loads again.
 */
export function loadMonaco(): Promise<MonacoApi> {
  loading ??= doLoad().catch((err: unknown) => {
    loading = undefined
    throw err
  })
  return loading
}

async function doLoad(): Promise<MonacoApi> {
  const [api, , , , , editorWorker, tsWorker, jsonWorker] = await Promise.all([
    import('monaco-editor/esm/vs/editor/editor.api'),
    import('monaco-editor/esm/vs/editor/editor.all'),
    import('monaco-editor/esm/vs/language/typescript/monaco.contribution'),
    import('monaco-editor/esm/vs/language/json/monaco.contribution'),
    import('monaco-editor/esm/vs/basic-languages/typescript/typescript.contribution'),
    import('monaco-editor/esm/vs/editor/editor.worker?worker'),
    import('monaco-editor/esm/vs/language/typescript/ts.worker?worker'),
    import('monaco-editor/esm/vs/language/json/json.worker?worker'),
  ])
  const workers: Record<string, new () => Worker> = {
    typescript: tsWorker.default, javascript: tsWorker.default, json: jsonWorker.default,
  }
  ;(self as unknown as { MonacoEnvironment: Monaco.Environment }).MonacoEnvironment = {
    getWorker: (_id: string, label: string) => new (workers[label] ?? editorWorker.default)(),
  }
  const ts = api.languages.typescript
  configureTypeScript(
    [ts.typescriptDefaults as unknown as TsDefaults, ts.javascriptDefaults as unknown as TsDefaults],
    ts as unknown as TsEnums,
  )
  return api
}

export function themeName(theme: 'light' | 'dark'): string { return theme === 'dark' ? 'vs-dark' : 'vs' }

export function languageOf(path: string): string {
  if (/\.(tsx?|mts|cts)$/.test(path)) return 'typescript'
  if (/\.(jsx?|mjs|cjs)$/.test(path)) return 'javascript'
  if (/\.json$/.test(path)) return 'json'
  if (/\.css$/.test(path)) return 'css'
  if (/\.md$/.test(path)) return 'markdown'
  return 'plaintext'
}

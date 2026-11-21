import base from './vite.package.config'
export default {
  ...base,
  build: { ...base.build, outDir: '/tmp/claude-1000/ae-analyze', minify: 'esbuild' },
  plugins: [...(base.plugins ?? []), {
    name: 'sizes',
    generateBundle(_o: unknown, bundle: Record<string, { type: string; modules?: Record<string, { renderedLength: number }> }>) {
      for (const [name, chunk] of Object.entries(bundle)) {
        if (chunk.type !== 'chunk' || !chunk.modules) continue
        const rows = Object.entries(chunk.modules).map(([id, m]) => [id.replace(/.*node_modules\/\.pnpm\/[^/]+\/node_modules\//, '').replace(/.*automation-editor\//, ''), m.renderedLength] as const).sort((a, b) => b[1] - a[1])
        console.log('==', name, rows.reduce((s, r) => s + r[1], 0))
        for (const [id, len] of rows.slice(0, 40)) console.log(String(len).padStart(8), id)
      }
    },
  }],
}

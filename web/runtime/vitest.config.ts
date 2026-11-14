import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vitest/config'

const local = (p: string) => fileURLToPath(new URL(p, import.meta.url))

export default defineConfig({
  // The conformance package (tests/conformance/package) imports the SDK and the kit by their package names.
  resolve: {
    alias: [
      { find: /^@iota-uz\/sdk\/runtime$/, replacement: local('./src/index.ts') },
      { find: /^@iota-uz\/sdk\/runtime\/server$/, replacement: local('./src/server.ts') },
      { find: /^@iota-kit\/core$/, replacement: local('./tests/conformance/kit-core.ts') },
    ],
  },
  test: { include: ['tests/**/*.test.ts'], environment: 'node' },
})

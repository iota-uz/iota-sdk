import { readFileSync, readdirSync } from 'node:fs'
import { gzipSync } from 'node:zlib'
import { transformWithEsbuild } from 'vite'
const dir = process.argv[2]
for (const f of readdirSync(dir).filter(n => n.endsWith('.js'))) {
  const code = readFileSync(dir + '/' + f, 'utf8')
  const out = await transformWithEsbuild(code, f, { minify: true, format: 'esm', loader: 'js' })
  console.log(f, (code.length/1024).toFixed(1), 'min', (out.code.length/1024).toFixed(1), 'gz', (gzipSync(out.code).length/1024).toFixed(1))
}

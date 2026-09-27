/** Static preview of the built story bundle: `/` -> stories.html, plus dist files. */
import { createServer } from 'node:http'
import { existsSync, readFileSync, statSync } from 'node:fs'
import { dirname, extname, join, normalize } from 'node:path'
import { fileURLToPath } from 'node:url'

const dist = join(dirname(fileURLToPath(import.meta.url)), '..', 'dist')
const port = Number(process.env.LENS_VR_PORT ?? '61000')

const types = {
  '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.json': 'application/json',
  '.svg': 'image/svg+xml', '.png': 'image/png', '.woff': 'font/woff', '.woff2': 'font/woff2',
}

createServer((request, response) => {
  const url = (request.url ?? '/').split('?')[0]
  const relative = url === '/' ? 'stories.html' : normalize(url).replace(/^([/\\])+/, '')
  const file = join(dist, relative)
  const candidate = file.endsWith('.html') || existsSync(file) && statSync(file).isFile()
    ? file
    : join(dist, `${relative}.html`)
  if (!existsSync(candidate)) {
    response.writeHead(404).end('not found')
    return
  }
  response.writeHead(200, { 'Content-Type': types[extname(candidate)] ?? 'application/octet-stream' })
  response.end(readFileSync(candidate))
}).listen(port, '127.0.0.1', () => {
  console.log(`story preview on http://127.0.0.1:${port}`)
})

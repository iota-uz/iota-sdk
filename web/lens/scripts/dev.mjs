import { createServer } from 'vite'

const fixtureMode = process.argv.slice(2).includes('--fixture')
const server = await createServer({
  mode: fixtureMode ? 'fixture' : 'development',
  define: {
    'import.meta.env.LENS_FIXTURE_MODE': JSON.stringify(fixtureMode ? 'true' : 'false'),
  },
})

await server.listen()
server.printUrls()
server.bindCLIShortcuts({ print: true })

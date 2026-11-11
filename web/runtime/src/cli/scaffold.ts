// `iota-ext init`: the files of a new package. The sources compile against the public API and its test passes.
export interface ScaffoldOptions {
  namespace: string
  /** npm name, e.g. "@acme/loyalty". */
  name: string
  sdkVersion: string
  client: boolean
}

const NAMESPACE = /^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*$/

export function validateNamespace(namespace: string): string | undefined {
  if (!NAMESPACE.test(namespace)) return `"${namespace}" is not a namespace: use dotted lowercase words such as "acme.loyalty"`
  if (!namespace.includes('.')) return `"${namespace}" has one segment: use "<owner>.<module>" such as "acme.loyalty" (the first segment must not collide with a platform namespace such as core or crm)`
  return undefined
}

export function defaultPackageName(namespace: string): string {
  const [owner = 'ext', ...rest] = namespace.split('.')
  return `@${owner}/${rest.join('-')}`
}

const q = JSON.stringify

export function scaffold(o: ScaffoldOptions): Record<string, string> {
  const files: Record<string, string> = {}
  const pkg = {
    name: o.name,
    version: '0.1.0',
    private: true,
    type: 'module',
    scripts: { check: 'iota-ext check', build: 'iota-ext build', test: 'iota-ext test' },
    iota: {
      namespace: o.namespace,
      contracts: './src/contracts.ts',
      server: './src/server.ts',
      ...(o.client ? { client: './src/client.tsx' } : {}),
    },
    // Exact, not a range: the extension runtime ships from the version of the CLI that wrote this file, and earlier public
    // builds of the package have no dist/runtime (a caret range could resolve to one of them).
    dependencies: { '@iota-uz/sdk': o.sdkVersion === '0.0.0' ? '*' : o.sdkVersion },
    devDependencies: { '@types/node': '^22.0.0', ...(o.client ? { 'solid-js': '^1.9.0' } : {}), typescript: '~5.7.2' },
  }
  files['package.json'] = JSON.stringify(pkg, null, 2) + '\n'
  files['tsconfig.json'] =
    JSON.stringify(
      {
        compilerOptions: {
          target: 'ES2022',
          module: 'ESNext',
          moduleResolution: 'Bundler',
          strict: true,
          noEmit: true,
          skipLibCheck: true,
          lib: ['ES2022', 'DOM', 'DOM.Iterable'],
          types: ['node'],
          // The generated kit lives in .iota/kit, not in node_modules (npm install deletes what it did not install).
          paths: { '@iota-kit/*': ['./.iota/kit/*'] },
          ...(o.client ? { jsx: 'preserve', jsxImportSource: 'solid-js' } : {}),
        },
        include: ['src'],
      },
      null,
      2,
    ) + '\n'
  // .iota/kit is committed: it pins the snapshot the package was written against and needs no server to restore.
  files['.gitignore'] = 'node_modules\n.iota/build\n'
  files['src/contracts.ts'] = `// Pure contracts: every resource this package declares. No handlers, no Node, no Solid.
import { command, entity, field, query, t } from "@iota-uz/sdk/runtime"

export const Note = entity(${q(`${o.namespace}.note`)}, {
  title: field.text({ maxLength: 120 }),
  body: field.text().optional(),
})

export const AddNote = command(${q(`${o.namespace}.addNote`)}, {
  atomic: true,
  input: t.object({ title: t.string(), body: t.string().optional() }),
  output: t.object({ note: t.ref(Note) }),
})

export const ListNotes = query(${q(`${o.namespace}.listNotes`)}, {
  input: t.object({}),
  output: t.array(t.object({ title: t.string(), body: t.string().nullable() })),
})
`
  files['src/server.ts'] = `// Implementations: each export binds a handler to a contract by typed reference.
import { implement } from "@iota-uz/sdk/runtime/server"
import * as c from "./contracts"

export const addNote = implement(c.AddNote, async (tx, input) => {
  const row = await tx.insert(c.Note, { title: input.title, body: input.body })
  return { note: row.ref }
})

export const listNotes = implement(c.ListNotes, async (q) => {
  const rows = await q.find(c.Note, { orderBy: [{ field: "createdAt", desc: true }], limit: 50 })
  return rows.map((row) => ({ title: row.title, body: row.body }))
})
`
  files['src/server.test.ts'] = `import { assert, createTestHost, test } from "@iota-uz/sdk/runtime/testing"
import * as s from "./server"

test("a stored note is listed back", async () => {
  const host = createTestHost({ implementations: [s.addNote, s.listNotes] })
  const added = await host.transaction((tx) => s.addNote.handler(tx, { title: "First" }))
  assert.equal(added.note.entity, ${q(`${o.namespace}.note`)})
  const notes = await s.listNotes.handler(host.query(), {})
  assert.deepEqual(notes, [{ title: "First", body: null }])
})
`
  if (o.client) {
    files['src/client.tsx'] = `// Browser graph: pages, navigation, slots. Solid components; import contracts, never the server entry.
import { lazy } from "solid-js"
import { navigation, page } from "@iota-uz/sdk/runtime/client"

export const notes = page(${q(`${o.namespace}.notes`)}, {
  path: ${q(`/${o.namespace.replace(/\./g, '-')}/notes`)},
  nav: { label: { en: "Notes" }, group: ${q(o.namespace)}, icon: "note" },
  component: lazy(() => import("./NotesScreen")),
})

export const nav = navigation({
  groups: [{ id: ${q(o.namespace)}, label: { en: "Notes" } }],
})
`
    files['src/NotesScreen.tsx'] = `import { For, Show } from "solid-js"
import { useQuery, type PageProps } from "@iota-uz/sdk/runtime/client"
import * as c from "./contracts"

export default function NotesScreen(_props: PageProps) {
  const notes = useQuery(c.ListNotes, {})
  return (
    <section>
      <h1>Notes</h1>
      <Show when={notes.data()}>
        {(rows) => (
          <ul>
            <For each={rows()}>{(note) => <li>{note.title}</li>}</For>
          </ul>
        )}
      </Show>
    </section>
  )
}
`
  }
  return files
}

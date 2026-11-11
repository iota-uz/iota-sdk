// Reads the npm-style .tgz the management API serves for a kit module (ustar + pax headers, regular files only).
import { gunzipSync } from 'node:zlib'

export interface TarEntry {
  /** Path inside the archive without the leading `package/` directory. */
  readonly path: string
  readonly data: Uint8Array
}

const decoder = new TextDecoder()

function field(block: Uint8Array, start: number, length: number): string {
  const slice = block.subarray(start, start + length)
  const end = slice.indexOf(0)
  return decoder.decode(end === -1 ? slice : slice.subarray(0, end))
}

function paxPath(data: Uint8Array): string | undefined {
  // "<len> path=<value>\n" records.
  const text = decoder.decode(data)
  let rest = text
  while (rest.length > 0) {
    const space = rest.indexOf(' ')
    const length = Number(rest.slice(0, space))
    if (!Number.isInteger(length) || length <= 0) return undefined
    const record = rest.slice(space + 1, length - 1)
    if (record.startsWith('path=')) return record.slice('path='.length)
    rest = rest.slice(length)
  }
  return undefined
}

/** Files of a gzip-compressed tar archive. Entries that escape the archive root are rejected. */
export function untar(gz: Uint8Array): TarEntry[] {
  const tar = gunzipSync(gz)
  const out: TarEntry[] = []
  let offset = 0
  let pending: string | undefined
  while (offset + 512 <= tar.length) {
    const header = tar.subarray(offset, offset + 512)
    if (header.every((b) => b === 0)) break
    const size = parseInt(field(header, 124, 12).trim() || '0', 8)
    const type = field(header, 156, 1)
    const prefix = field(header, 345, 155)
    let name = prefix === '' ? field(header, 0, 100) : `${prefix}/${field(header, 0, 100)}`
    const body = tar.subarray(offset + 512, offset + 512 + size)
    offset += 512 + Math.ceil(size / 512) * 512
    if (type === 'x') {
      pending = paxPath(body)
      continue
    }
    if (type === 'g') continue
    if (pending !== undefined) {
      name = pending
      pending = undefined
    }
    if (type !== '0' && type !== '') continue
    const parts = name.split('/').filter((p) => p !== '' && p !== '.')
    if (parts.includes('..') || name.startsWith('/')) throw new Error(`archive entry "${name}" escapes the package root`)
    const path = parts.slice(1).join('/')
    if (path !== '') out.push({ path, data: Uint8Array.from(body) })
  }
  return out
}

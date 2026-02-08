/**
 * File Utilities
 * Validation, conversion, and formatting for file attachments
 */

const MAX_FILE_SIZE_BYTES = 20 * 1024 * 1024 // 20MB

const ALLOWED_MIME_TYPES = new Set<string>([
  'image/jpeg',
  'image/jpg',
  'image/png',
  'image/gif',
  'image/webp',
  'application/pdf',
  'application/msword',
  'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
  'application/vnd.ms-excel',
  'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  'text/csv',
  'text/tab-separated-values',
  'text/plain',
  'text/markdown',
  'application/json',
  'application/xml',
  'text/xml',
  'application/yaml',
  'text/yaml',
  'application/x-yaml',
  'text/x-yaml',
  'text/log',
])

const ALLOWED_EXTENSIONS = new Set<string>([
  '.png',
  '.jpg',
  '.jpeg',
  '.gif',
  '.webp',
  '.pdf',
  '.doc',
  '.docx',
  '.xls',
  '.xlsx',
  '.csv',
  '.tsv',
  '.txt',
  '.md',
  '.json',
  '.xml',
  '.yaml',
  '.yml',
  '.log',
])

export const ATTACHMENT_ACCEPT_ATTRIBUTE = Array.from(ALLOWED_EXTENSIONS).join(',')

export function isImageMimeType(mimeType: string): boolean {
  return mimeType.toLowerCase().startsWith('image/')
}

/**
 * Validates a file against size and type constraints
 * @throws Error if validation fails
 */
export function validateAttachmentFile(file: File, maxSizeBytes: number = MAX_FILE_SIZE_BYTES): void {
  const mimeType = file.type.toLowerCase()
  const fileName = file.name || 'file'
  const extensionMatch = /\.(\w+)$/.exec(fileName.toLowerCase())
  const extension = extensionMatch ? `.${extensionMatch[1]}` : ''

  const mimeAllowed = mimeType !== '' && ALLOWED_MIME_TYPES.has(mimeType)
  const extensionAllowed = extension !== '' && ALLOWED_EXTENSIONS.has(extension)

  if (!mimeAllowed && !extensionAllowed) {
    throw new Error(`Invalid file type: ${mimeType || extension || 'unknown'}`)
  }

  if (file.size > maxSizeBytes) {
    const sizeMB = (file.size / 1024 / 1024).toFixed(1)
    const maxSizeMB = (maxSizeBytes / 1024 / 1024).toFixed(0)
    throw new Error(`File too large: ${sizeMB}MB exceeds ${maxSizeMB}MB limit`)
  }
}

/**
 * Backward-compatible image validator used by older components/stories.
 */
export function validateImageFile(file: File, maxSizeBytes: number = MAX_FILE_SIZE_BYTES): void {
  validateAttachmentFile(file, maxSizeBytes)
  if (!isImageMimeType(file.type)) {
    throw new Error(`Invalid file type: ${file.type}. Only image files are allowed.`)
  }
}

/**
 * Encode an ArrayBuffer to a base64 string.
 */
function encodeArrayBuffer(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer)
  let binary = ''
  for (let i = 0; i < bytes.length; i++) {
    binary += String.fromCharCode(bytes[i])
  }
  return btoa(binary)
}

function readWithFileReader(file: File): Promise<ArrayBuffer> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => {
      if (reader.result instanceof ArrayBuffer) {
        resolve(reader.result)
        return
      }
      reject(new Error('Unexpected reader result'))
    }
    reader.onerror = () => reject(reader.error ?? new Error('FileReader failed'))
    reader.onabort = () => reject(new Error('File read was aborted'))
    reader.readAsArrayBuffer(file)
  })
}

async function readFileBuffer(file: File): Promise<ArrayBuffer> {
  try {
    return await new Response(file).arrayBuffer()
  } catch {
    // Fall through to native/file-reader strategies for drag/drop edge cases.
  }

  try {
    return await file.arrayBuffer()
  } catch {
    return readWithFileReader(file)
  }
}

/**
 * Converts a file to base64 string (without data URL prefix).
 * Prefers Response/Streams reading, then falls back to native Blob and
 * FileReader strategies for drag/drop edge cases across browsers.
 */
export async function convertToBase64(file: File): Promise<string> {
  try {
    const buffer = await readFileBuffer(file)
    return encodeArrayBuffer(buffer)
  } catch {
    throw new Error(`Failed to read file: ${file.name}`)
  }
}

/**
 * Creates a data URL from base64 string and MIME type
 */
export function createDataUrl(base64: string, mimeType: string): string {
  return `data:${mimeType};base64,${base64}`
}

/**
 * Formats file size in human-readable format
 */
export function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

/**
 * Validates multiple files don't exceed count limit
 * @throws Error if count exceeds limit
 */
export function validateFileCount(currentCount: number, newCount: number, maxCount: number = 10): void {
  const total = currentCount + newCount
  if (total > maxCount) {
    throw new Error(`Cannot attach more than ${maxCount} files (attempting to add ${total})`)
  }
}

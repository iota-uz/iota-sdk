/**
 * File Utilities
 * Validation, conversion, and formatting for file attachments
 */

const MAX_FILE_SIZE_BYTES = 20 * 1024 * 1024 // 20MB
const ALLOWED_IMAGE_TYPES = ['image/png', 'image/jpeg', 'image/webp', 'image/gif']

/**
 * Validates an image file against size and type constraints
 * @throws Error if validation fails
 */
export function validateImageFile(file: File, maxSizeBytes: number = MAX_FILE_SIZE_BYTES): void {
  if (!ALLOWED_IMAGE_TYPES.includes(file.type)) {
    throw new Error(`Invalid file type: ${file.type}. Only PNG, JPEG, WebP, and GIF are allowed.`)
  }
  if (file.size > maxSizeBytes) {
    const sizeMB = (file.size / 1024 / 1024).toFixed(1)
    const maxSizeMB = (maxSizeBytes / 1024 / 1024).toFixed(0)
    throw new Error(`File too large: ${sizeMB}MB exceeds ${maxSizeMB}MB limit`)
  }
}

/**
 * Converts a file to base64 string (without data URL prefix)
 */
export async function convertToBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => {
      const result = reader.result as string
      // Strip data URL prefix (e.g., "data:image/png;base64,")
      const base64 = result.split(',')[1]
      resolve(base64)
    }
    reader.onerror = () => reject(new Error('Failed to read file'))
    reader.readAsDataURL(file)
  })
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

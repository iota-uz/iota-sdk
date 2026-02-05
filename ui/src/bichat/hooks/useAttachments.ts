/**
 * useAttachments Hook
 * Manages file upload state, validation, and preview
 */

import { useState, useCallback, useMemo } from 'react'
import type { Attachment } from '../types'

export interface FileValidationError {
  file: File
  reason: 'size' | 'type' | 'count' | 'custom'
  message: string
}

export interface UseAttachmentsOptions {
  /** Maximum number of files (default: 10) */
  maxFiles?: number
  /** Maximum file size in bytes (default: 10MB) */
  maxFileSize?: number
  /** Allowed MIME types (default: images only) */
  allowedTypes?: string[]
  /** Custom validation function */
  validate?: (file: File) => string | null
  /** Callback when files are added */
  onAdd?: (files: Attachment[]) => void
  /** Callback when a file is removed */
  onRemove?: (file: Attachment) => void
  /** Callback when validation fails */
  onError?: (errors: FileValidationError[]) => void
}

export interface UseAttachmentsReturn {
  /** Current attachments */
  files: Attachment[]
  /** Validation errors from last operation */
  errors: FileValidationError[]
  /** Whether files are being processed */
  isProcessing: boolean
  /** Whether max file limit is reached */
  isMaxReached: boolean
  /** Number of remaining slots */
  remainingSlots: number
  /** Add files (validates and processes) */
  add: (files: FileList | File[]) => Promise<void>
  /** Remove a specific file */
  remove: (fileOrId: Attachment | string) => void
  /** Clear all files */
  clear: () => void
  /** Clear errors */
  clearErrors: () => void
  /** Set files directly (for controlled mode) */
  setFiles: (files: Attachment[]) => void
}

const DEFAULT_MAX_FILES = 10
const DEFAULT_MAX_FILE_SIZE = 10 * 1024 * 1024 // 10MB
const DEFAULT_ALLOWED_TYPES = ['image/jpeg', 'image/png', 'image/gif', 'image/webp']

/**
 * Generate a unique ID for an attachment
 */
function generateId(): string {
  return `attachment-${Date.now()}-${Math.random().toString(36).substring(7)}`
}

/**
 * Check if a file is an image
 */
function isImageFile(file: File): boolean {
  return file.type.startsWith('image/')
}

/**
 * Create a data URL preview for an image file
 */
async function createImagePreview(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as string)
    reader.onerror = () => reject(new Error('Failed to read file'))
    reader.readAsDataURL(file)
  })
}

/**
 * Hook for managing file attachments
 *
 * @example
 * ```tsx
 * const attachments = useAttachments({
 *   maxFiles: 5,
 *   maxFileSize: 5 * 1024 * 1024, // 5MB
 *   onError: (errors) => errors.forEach(e => toast.error(e.message)),
 * })
 *
 * <input
 *   type="file"
 *   multiple
 *   accept="image/*"
 *   onChange={(e) => attachments.add(e.target.files)}
 * />
 *
 * {attachments.files.map(file => (
 *   <AttachmentPreview
 *     key={file.id}
 *     attachment={file}
 *     onRemove={() => attachments.remove(file)}
 *   />
 * ))}
 *
 * {attachments.errors.length > 0 && (
 *   <ErrorList errors={attachments.errors} />
 * )}
 * ```
 */
export function useAttachments(options: UseAttachmentsOptions = {}): UseAttachmentsReturn {
  const {
    maxFiles = DEFAULT_MAX_FILES,
    maxFileSize = DEFAULT_MAX_FILE_SIZE,
    allowedTypes = DEFAULT_ALLOWED_TYPES,
    validate,
    onAdd,
    onRemove,
    onError,
  } = options

  const [files, setFiles] = useState<Attachment[]>([])
  const [errors, setErrors] = useState<FileValidationError[]>([])
  const [isProcessing, setIsProcessing] = useState(false)

  const isMaxReached = useMemo(() => files.length >= maxFiles, [files.length, maxFiles])
  const remainingSlots = useMemo(() => Math.max(0, maxFiles - files.length), [files.length, maxFiles])

  const validateFile = useCallback(
    (file: File, currentCount: number): FileValidationError | null => {
      // Check count
      if (currentCount >= maxFiles) {
        return {
          file,
          reason: 'count',
          message: `Maximum ${maxFiles} files allowed`,
        }
      }

      // Check file size
      if (file.size > maxFileSize) {
        const maxSizeMB = (maxFileSize / (1024 * 1024)).toFixed(1)
        return {
          file,
          reason: 'size',
          message: `File "${file.name}" exceeds ${maxSizeMB}MB limit`,
        }
      }

      // Check file type
      if (allowedTypes.length > 0 && !allowedTypes.includes(file.type)) {
        return {
          file,
          reason: 'type',
          message: `File type "${file.type || 'unknown'}" not allowed`,
        }
      }

      // Custom validation
      if (validate) {
        const customError = validate(file)
        if (customError) {
          return {
            file,
            reason: 'custom',
            message: customError,
          }
        }
      }

      return null
    },
    [maxFiles, maxFileSize, allowedTypes, validate]
  )

  const add = useCallback(
    async (newFiles: FileList | File[]) => {
      if (isProcessing) return

      setIsProcessing(true)
      const fileArray = Array.from(newFiles)
      const validationErrors: FileValidationError[] = []
      const validFiles: Attachment[] = []
      let currentCount = files.length

      for (const file of fileArray) {
        const error = validateFile(file, currentCount)
        if (error) {
          validationErrors.push(error)
          continue
        }

        // Create attachment object
        const attachment: Attachment = {
          id: generateId(),
          filename: file.name,
          mimeType: file.type,
          sizeBytes: file.size,
        }

        // Add preview URL for images
        if (isImageFile(file)) {
          try {
            const preview = await createImagePreview(file)
            attachment.base64Data = preview
          } catch {
            // Continue without preview if it fails
          }
        }

        validFiles.push(attachment)
        currentCount++
      }

      if (validationErrors.length > 0) {
        setErrors(validationErrors)
        onError?.(validationErrors)
      }

      if (validFiles.length > 0) {
        setFiles((prev) => [...prev, ...validFiles])
        onAdd?.(validFiles)
      }

      setIsProcessing(false)
    },
    [files.length, validateFile, onAdd, onError, isProcessing]
  )

  const remove = useCallback(
    (fileOrId: Attachment | string) => {
      const id = typeof fileOrId === 'string' ? fileOrId : fileOrId.id
      setFiles((prev) => {
        const fileToRemove = prev.find((f) => f.id === id)
        if (fileToRemove) {
          onRemove?.(fileToRemove)
        }
        return prev.filter((f) => f.id !== id)
      })
    },
    [onRemove]
  )

  const clear = useCallback(() => {
    setFiles([])
    setErrors([])
  }, [])

  const clearErrors = useCallback(() => {
    setErrors([])
  }, [])

  const setFilesHandler = useCallback((newFiles: Attachment[]) => {
    setFiles(newFiles)
  }, [])

  return {
    files,
    errors,
    isProcessing,
    isMaxReached,
    remainingSlots,
    add,
    remove,
    clear,
    clearErrors,
    setFiles: setFilesHandler,
  }
}

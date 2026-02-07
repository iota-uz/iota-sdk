/**
 * AttachmentUpload Component
 * Handles file selection and base64 encoding for chat attachments
 * Provides loading states, validation, and error handling
 */

import { memo, useRef, useCallback, useState } from 'react'
import { Paperclip, CircleNotch } from '@phosphor-icons/react'
import { Attachment } from '../types'
import {
  ATTACHMENT_ACCEPT_ATTRIBUTE,
  convertToBase64,
  createDataUrl,
  formatFileSize,
  isImageMimeType,
  validateAttachmentFile,
} from '../utils/fileUtils'
import { useToast } from '../hooks/useToast'
import { useTranslation } from '../hooks/useTranslation'

interface AttachmentError {
  filename: string
  error: string
}

interface AttachmentUploadProps {
  /** Callback fired when files are successfully converted and validated */
  onAttachmentsSelected: (attachments: Attachment[]) => void
  /** Maximum number of attachments allowed (default: 10) */
  maxAttachments?: number
  /** Maximum file size in bytes (default: 20 MB) */
  maxSizeBytes?: number
  /** Whether the component is disabled */
  disabled?: boolean
}

const AttachmentUpload = memo<AttachmentUploadProps>(
  ({ onAttachmentsSelected, maxAttachments = 10, maxSizeBytes = 20 * 1024 * 1024, disabled = false }) => {
    const fileInputRef = useRef<HTMLInputElement>(null)
    const [isLoading, setIsLoading] = useState(false)
    const toast = useToast()
    const { t } = useTranslation()

    /**
     * Handles file selection from the input element
     * Validates, converts to base64, and calls onAttachmentsSelected
     */
    const handleFileSelect = useCallback(
      async (e: React.ChangeEvent<HTMLInputElement>) => {
        const files = Array.from(e.target.files || [])

        // Reset input so same file selection can be processed again
        if (fileInputRef.current) {
          fileInputRef.current.value = ''
        }

        if (files.length === 0) {
          return
        }

        setIsLoading(true)

        try {
          // Validate file count
          if (files.length > maxAttachments) {
            toast.error(t('error.maxFiles', { max: maxAttachments, selected: files.length }))
            setIsLoading(false)
            return
          }

          const attachments: Attachment[] = []
          const errors: AttachmentError[] = []

          // Process each file
          for (const file of files) {
            // Validate file
            try {
              validateAttachmentFile(file, maxSizeBytes)
            } catch (validationErr) {
              errors.push({ filename: file.name, error: validationErr instanceof Error ? validationErr.message : String(validationErr) })
              continue
            }

            try {
              // Convert to base64
              const base64Data = await convertToBase64(file)
              const attachment: Attachment = {
                id: '',
                filename: file.name,
                mimeType: file.type,
                sizeBytes: file.size,
                base64Data,
              }
              if (isImageMimeType(file.type)) {
                attachment.preview = createDataUrl(base64Data, file.type)
              }

              attachments.push(attachment)
            } catch (err) {
              errors.push({
                filename: file.name,
                error: err instanceof Error ? err.message : String(err),
              })
            }
          }

          // Show error toasts for failed files
          if (errors.length > 0) {
            errors.forEach((err) => {
              toast.error(`${err.filename}: ${err.error}`)
            })
          }

          // Call parent callback with successful attachments
          if (attachments.length > 0) {
            onAttachmentsSelected(attachments)
            const message =
              attachments.length === 1
                ? t('attachment.fileAdded', { size: formatFileSize(attachments[0].sizeBytes) })
                : t('attachment.fileAdded', { size: `${attachments.length} files` })
            toast.success(message)
          } else if (errors.length > 0) {
            toast.error(t('attachment.invalidFile'))
          }
        } finally {
          setIsLoading(false)
        }
      },
      [maxAttachments, maxSizeBytes, onAttachmentsSelected, toast, t]
    )

    /**
     * Triggers the hidden file input
     */
    const handleClick = useCallback(() => {
      fileInputRef.current?.click()
    }, [])

    const isDisabled = disabled || isLoading

    return (
      <div className="relative">
        {/* Hidden File Input */}
        <input
          ref={fileInputRef}
          type="file"
          multiple
          accept={ATTACHMENT_ACCEPT_ATTRIBUTE}
          onChange={handleFileSelect}
          disabled={isDisabled}
          className="sr-only"
          aria-label={t('attachment.selectFiles')}
        />

        {/* Trigger Button */}
        <button
          type="button"
          onClick={handleClick}
          disabled={isDisabled}
          className="
            flex items-center justify-center
            w-8 h-8
            text-gray-600 dark:text-gray-400
            hover:text-gray-800 dark:hover:text-gray-200
            hover:bg-gray-100 dark:hover:bg-gray-700
            disabled:text-gray-400 dark:disabled:text-gray-600
            disabled:opacity-50 disabled:cursor-not-allowed
            rounded-lg
            transition-all
            duration-200
            active:scale-95
            focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50
          "
          aria-label={t('attachment.selectFiles')}
          aria-busy={isLoading}
        >
          {isLoading ? (
            <CircleNotch size={20} className="w-5 h-5 animate-spin" weight="fill" />
          ) : (
            <Paperclip size={20} className="w-5 h-5" weight="fill" />
          )}
        </button>
      </div>
    )
  }
)

AttachmentUpload.displayName = 'AttachmentUpload'

export default AttachmentUpload

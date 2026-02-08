import { useEffect, useState } from 'react'
import { Dialog, DialogBackdrop, DialogPanel } from '@headlessui/react'
import { FloppyDisk, PencilSimple, Trash, X } from '@phosphor-icons/react'
import type { SessionArtifact } from '../types'
import { useTranslation } from '../hooks/useTranslation'
import { SessionArtifactPreview } from './SessionArtifactPreview'

interface SessionArtifactPreviewModalProps {
  isOpen: boolean
  artifact: SessionArtifact | null
  canRename?: boolean
  canDelete?: boolean
  onClose: () => void
  onRename?: (artifact: SessionArtifact, name: string) => Promise<void>
  onDelete?: (artifact: SessionArtifact) => Promise<void>
}

export function SessionArtifactPreviewModal({
  isOpen,
  artifact,
  canRename = false,
  canDelete = false,
  onClose,
  onRename,
  onDelete,
}: SessionArtifactPreviewModalProps) {
  const { t } = useTranslation()
  const [isEditingName, setIsEditingName] = useState(false)
  const [nameDraft, setNameDraft] = useState('')
  const [submittingRename, setSubmittingRename] = useState(false)
  const [submittingDelete, setSubmittingDelete] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    setIsEditingName(false)
    setSubmittingRename(false)
    setSubmittingDelete(false)
    setError(null)
    setNameDraft(artifact?.name || '')
  }, [artifact])

  const handleClose = () => {
    if (submittingRename || submittingDelete) {
      return
    }
    onClose()
  }

  const handleRename = async () => {
    if (!artifact || !onRename) {
      return
    }

    const nextName = nameDraft.trim()
    if (!nextName || nextName === artifact.name) {
      setIsEditingName(false)
      setNameDraft(artifact.name)
      return
    }

    setSubmittingRename(true)
    setError(null)
    try {
      await onRename(artifact, nextName)
      setIsEditingName(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('artifacts.renameFailed'))
    } finally {
      setSubmittingRename(false)
    }
  }

  const handleDelete = async () => {
    if (!artifact || !onDelete) {
      return
    }
    if (!window.confirm(t('artifacts.deleteConfirm'))) {
      return
    }

    setSubmittingDelete(true)
    setError(null)
    try {
      await onDelete(artifact)
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : t('artifacts.deleteFailed'))
    } finally {
      setSubmittingDelete(false)
    }
  }

  if (!artifact) {
    return null
  }

  return (
    <Dialog open={isOpen} onClose={handleClose} className="relative z-50">
      <DialogBackdrop className="fixed inset-0 bg-black/50 backdrop-blur-sm" />

      <div className="fixed inset-0 overflow-y-auto p-4 lg:p-6">
        <div className="mx-auto flex min-h-full w-full max-w-6xl items-center justify-center">
          <DialogPanel className="flex max-h-[92vh] w-full flex-col overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-xl dark:border-gray-700 dark:bg-gray-900">
            <div className="flex items-start justify-between gap-3 border-b border-gray-200 px-4 py-3 dark:border-gray-700">
              <div className="min-w-0 flex-1">
                {isEditingName ? (
                  <div className="flex items-center gap-2">
                    <input
                      value={nameDraft}
                      onChange={(e) => setNameDraft(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') {
                          e.preventDefault()
                          void handleRename()
                        }
                        if (e.key === 'Escape') {
                          e.preventDefault()
                          setIsEditingName(false)
                          setNameDraft(artifact.name)
                        }
                      }}
                      className="w-full rounded-lg border border-gray-300 px-3 py-1.5 text-sm text-gray-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                      aria-label={t('artifacts.rename')}
                      autoFocus
                    />
                    <button
                      type="button"
                      onClick={() => {
                        void handleRename()
                      }}
                      disabled={submittingRename}
                      className="cursor-pointer inline-flex items-center gap-1 rounded-lg bg-primary-600 px-2.5 py-1.5 text-xs font-medium text-white transition-colors hover:bg-primary-700 disabled:cursor-not-allowed disabled:opacity-60"
                    >
                      <FloppyDisk className="h-3.5 w-3.5" weight="bold" />
                      {t('message.save')}
                    </button>
                    <button
                      type="button"
                      onClick={() => {
                        setIsEditingName(false)
                        setNameDraft(artifact.name)
                      }}
                      disabled={submittingRename}
                      className="cursor-pointer rounded-lg border border-gray-200 px-2.5 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-60 dark:border-gray-700 dark:text-gray-200 dark:hover:bg-gray-800"
                    >
                      {t('message.cancel')}
                    </button>
                  </div>
                ) : (
                  <h2 className="truncate text-base font-semibold text-gray-900 dark:text-gray-100">
                    {artifact.name}
                  </h2>
                )}
                {artifact.description && (
                  <p className="mt-1 truncate text-xs text-gray-500 dark:text-gray-400">
                    {artifact.description}
                  </p>
                )}
              </div>

              <div className="flex items-center gap-1.5">
                {canRename && onRename && !isEditingName && (
                  <button
                    type="button"
                    onClick={() => {
                      setError(null)
                      setIsEditingName(true)
                    }}
                    className="cursor-pointer rounded-lg border border-gray-200 p-2 text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-900 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800 dark:hover:text-gray-100"
                    aria-label={t('artifacts.rename')}
                    title={t('artifacts.rename')}
                  >
                    <PencilSimple className="h-4 w-4" weight="regular" />
                  </button>
                )}
                {canDelete && onDelete && (
                  <button
                    type="button"
                    onClick={() => {
                      void handleDelete()
                    }}
                    disabled={submittingDelete}
                    className="cursor-pointer rounded-lg border border-red-200 p-2 text-red-600 transition-colors hover:bg-red-50 hover:text-red-700 disabled:cursor-not-allowed disabled:opacity-60 dark:border-red-900/60 dark:text-red-400 dark:hover:bg-red-950/30 dark:hover:text-red-300"
                    aria-label={t('artifacts.delete')}
                    title={t('artifacts.delete')}
                  >
                    <Trash className="h-4 w-4" weight="regular" />
                  </button>
                )}
                <button
                  type="button"
                  onClick={handleClose}
                  className="cursor-pointer rounded-lg border border-gray-200 p-2 text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-900 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800 dark:hover:text-gray-100"
                  aria-label={t('common.close')}
                  title={t('common.close')}
                >
                  <X className="h-4 w-4" weight="bold" />
                </button>
              </div>
            </div>

            <div className="min-h-0 flex-1 overflow-auto p-4 lg:p-5">
              {error && (
                <div className="mb-3 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900/70 dark:bg-red-950/30 dark:text-red-300">
                  {error}
                </div>
              )}
              <SessionArtifactPreview artifact={artifact} />
            </div>
          </DialogPanel>
        </div>
      </div>
    </Dialog>
  )
}

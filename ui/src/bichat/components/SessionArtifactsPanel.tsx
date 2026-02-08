import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { CaretLeft, Paperclip } from '@phosphor-icons/react'
import type { ChatDataSource, SessionArtifact } from '../types'
import { useTranslation } from '../hooks/useTranslation'
import { useChatMessaging } from '../context/ChatContext'
import { SessionArtifactList } from './SessionArtifactList'
import { SessionArtifactPreview } from './SessionArtifactPreview'

interface SessionArtifactsPanelProps {
  dataSource: ChatDataSource
  sessionId: string
  isStreaming: boolean
  allowDrop?: boolean
  className?: string
}

const PAGE_SIZE = 50

function mergeArtifacts(existing: SessionArtifact[], incoming: SessionArtifact[]): SessionArtifact[] {
  const merged = [...existing]
  const existingIds = new Set(existing.map((artifact) => artifact.id))

  for (const artifact of incoming) {
    if (existingIds.has(artifact.id)) {
      continue
    }
    merged.push(artifact)
    existingIds.add(artifact.id)
  }

  return merged
}

export function SessionArtifactsPanel({
  dataSource,
  sessionId,
  isStreaming,
  allowDrop = true,
  className = '',
}: SessionArtifactsPanelProps) {
  const { t } = useTranslation()
  const { artifactsInvalidationTrigger } = useChatMessaging()

  const [fetching, setFetching] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [loadingMore, setLoadingMore] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [artifacts, setArtifacts] = useState<SessionArtifact[]>([])
  const [selectedArtifactId, setSelectedArtifactId] = useState<string | null>(null)
  const [hasMore, setHasMore] = useState(false)
  const [isDragging, setIsDragging] = useState(false)
  const [dropSuccess, setDropSuccess] = useState(false)

  const requestSeq = useRef(0)
  const hasLoadedRef = useRef(false)
  const prevStreamingRef = useRef(isStreaming)
  const artifactsRef = useRef<SessionArtifact[]>([])
  const nextOffsetRef = useRef(0)
  const dragDepthRef = useRef(0)
  const dropSuccessTimerRef = useRef<number | null>(null)

  const canFetchArtifacts = typeof dataSource.fetchSessionArtifacts === 'function'
  const canDropFiles = allowDrop && typeof dataSource.uploadSessionArtifacts === 'function'

  const tRef = useRef(t)
  tRef.current = t

  const fetchArtifacts = useCallback(
    async (opts: { reset: boolean; manual: boolean }) => {
      if (!canFetchArtifacts || !dataSource.fetchSessionArtifacts) {
        setFetching(false)
        setRefreshing(false)
        setLoadingMore(false)
        setArtifacts([])
        setError(null)
        setHasMore(false)
        nextOffsetRef.current = 0
        return
      }

      const requestID = ++requestSeq.current
      const offset = opts.reset ? 0 : nextOffsetRef.current

      if (!hasLoadedRef.current || opts.reset) {
        if (opts.manual && hasLoadedRef.current) {
          setRefreshing(true)
        } else {
          setFetching(true)
        }
      } else {
        setLoadingMore(true)
      }
      setError(null)

      try {
        const response = await dataSource.fetchSessionArtifacts(sessionId, {
          limit: PAGE_SIZE,
          offset,
        })
        if (requestID !== requestSeq.current) {
          return
        }

        const page = [...(response.artifacts || [])].sort(
          (a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt)
        )

        const nextList = opts.reset ? page : mergeArtifacts(artifactsRef.current, page)

        setArtifacts(nextList)
        artifactsRef.current = nextList
        hasLoadedRef.current = true

        const resolvedHasMore = Boolean(response.hasMore)
        const resolvedNextOffset =
          typeof response.nextOffset === 'number'
            ? response.nextOffset
            : offset + page.length

        setHasMore(resolvedHasMore)
        nextOffsetRef.current = resolvedNextOffset

        setSelectedArtifactId((current) => {
          if (!current) {
            return null
          }
          return nextList.some((artifact) => artifact.id === current) ? current : null
        })
      } catch (err) {
        if (requestID !== requestSeq.current) {
          return
        }
        setError(err instanceof Error ? err.message : tRef.current('artifacts.failedToLoad'))
      } finally {
        if (requestID === requestSeq.current) {
          setFetching(false)
          setRefreshing(false)
          setLoadingMore(false)
        }
      }
    },
    [canFetchArtifacts, dataSource, sessionId]
  )

  useEffect(() => {
    hasLoadedRef.current = false
    setFetching(true)
    setRefreshing(false)
    setLoadingMore(false)
    setError(null)
    setArtifacts([])
    artifactsRef.current = []
    setSelectedArtifactId(null)
    setHasMore(false)
    nextOffsetRef.current = 0
    void fetchArtifacts({ reset: true, manual: false })
  }, [fetchArtifacts, sessionId])

  useEffect(() => {
    const wasStreaming = prevStreamingRef.current
    if (wasStreaming && !isStreaming) {
      void fetchArtifacts({ reset: true, manual: false })
    }
    prevStreamingRef.current = isStreaming
  }, [fetchArtifacts, isStreaming])

  useEffect(() => {
    if (artifactsInvalidationTrigger > 0 && sessionId && canFetchArtifacts) {
      void fetchArtifacts({ reset: true, manual: false })
    }
  }, [artifactsInvalidationTrigger, sessionId, canFetchArtifacts, fetchArtifacts])

  useEffect(() => {
    const handler = () => {
      if (document.visibilityState === 'visible' && sessionId && canFetchArtifacts) {
        void fetchArtifacts({ reset: true, manual: false })
      }
    }
    document.addEventListener('visibilitychange', handler)
    return () => document.removeEventListener('visibilitychange', handler)
  }, [sessionId, canFetchArtifacts, fetchArtifacts])

  const selectedArtifact = useMemo(
    () => artifacts.find((artifact) => artifact.id === selectedArtifactId) ?? null,
    [artifacts, selectedArtifactId]
  )

  const clearDropSuccessTimer = useCallback(() => {
    if (dropSuccessTimerRef.current === null) return
    window.clearTimeout(dropSuccessTimerRef.current)
    dropSuccessTimerRef.current = null
  }, [])

  useEffect(() => {
    return () => {
      clearDropSuccessTimer()
    }
  }, [clearDropSuccessTimer])

  const setDropSuccessState = useCallback(() => {
    setDropSuccess(true)
    clearDropSuccessTimer()
    dropSuccessTimerRef.current = window.setTimeout(() => {
      setDropSuccess(false)
      dropSuccessTimerRef.current = null
    }, 1400)
  }, [clearDropSuccessTimer])

  const hasDragFiles = useCallback((e: React.DragEvent): boolean => {
    return Array.from(e.dataTransfer.types || []).includes('Files')
  }, [])

  const handleDragEnter = useCallback((e: React.DragEvent) => {
    if (!canDropFiles || !hasDragFiles(e)) return
    e.preventDefault()
    e.stopPropagation()
    dragDepthRef.current += 1
    setIsDragging(true)
  }, [canDropFiles, hasDragFiles])

  const handleDragOver = useCallback((e: React.DragEvent) => {
    if (!canDropFiles || !hasDragFiles(e)) return
    e.preventDefault()
    e.stopPropagation()
    e.dataTransfer.dropEffect = 'copy'
  }, [canDropFiles, hasDragFiles])

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    if (!canDropFiles || !hasDragFiles(e)) return
    e.preventDefault()
    e.stopPropagation()
    dragDepthRef.current = Math.max(0, dragDepthRef.current - 1)
    if (dragDepthRef.current === 0) {
      setIsDragging(false)
    }
  }, [canDropFiles, hasDragFiles])

  const handleDrop = useCallback(async (e: React.DragEvent) => {
    if (!canDropFiles || !dataSource.uploadSessionArtifacts || !hasDragFiles(e)) return
    e.preventDefault()
    e.stopPropagation()
    dragDepthRef.current = 0
    setIsDragging(false)

    const itemFiles = Array.from(e.dataTransfer.items || [])
      .filter((item) => item.kind === 'file')
      .map((item) => item.getAsFile())
      .filter((file): file is File => file !== null)
    const droppedFiles = itemFiles.length > 0 ? itemFiles : Array.from(e.dataTransfer.files || [])

    if (droppedFiles.length === 0) return
    try {
      const result = await dataSource.uploadSessionArtifacts(sessionId, droppedFiles)
      if ((result.artifacts || []).length > 0) {
        setDropSuccessState()
        void fetchArtifacts({ reset: true, manual: false })
      }
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : tRef.current('artifacts.failedToLoad'))
    }
  }, [canDropFiles, dataSource, fetchArtifacts, hasDragFiles, sessionId, setDropSuccessState])

  return (
    <aside
      className={[
        'relative flex min-w-0 flex-1 flex-col border-l border-gray-200 bg-white dark:border-gray-700/80 dark:bg-gray-900',
        isDragging ? 'bg-primary-50/40 dark:bg-primary-950/20' : '',
        className,
      ].join(' ')}
      aria-label={t('artifacts.title')}
      onDragEnter={handleDragEnter}
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
    >
      {(isDragging || dropSuccess) && (
        <div className="pointer-events-none absolute inset-0 z-20 flex items-center justify-center bg-white/85 dark:bg-gray-900/85">
          <div
            className={[
              'mx-4 flex w-full max-w-xs flex-col items-center gap-2 rounded-xl border-2 border-dashed px-4 py-6 text-center',
              dropSuccess
                ? 'border-emerald-400 bg-emerald-50 text-emerald-700 dark:border-emerald-500 dark:bg-emerald-950/30 dark:text-emerald-300'
                : 'border-primary-400 bg-primary-50 text-primary-700 dark:border-primary-500 dark:bg-primary-950/30 dark:text-primary-300',
            ].join(' ')}
          >
            <Paperclip className="h-5 w-5" weight="bold" />
            <span className="text-sm font-medium">
              {dropSuccess ? t('input.filesAdded') : t('input.dropFiles')}
            </span>
          </div>
        </div>
      )}

      <header className="flex items-center justify-between border-b border-gray-200 px-3 py-2 dark:border-gray-700/80">
          <div className="min-w-0 flex-1">
            {selectedArtifact ? (
              <button
                type="button"
                onClick={() => setSelectedArtifactId(null)}
                className="cursor-pointer inline-flex items-center gap-1 rounded-md px-1 py-1 text-xs font-medium text-gray-700 transition-colors hover:text-gray-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 dark:text-gray-300 dark:hover:text-gray-100"
              >
                <CaretLeft className="h-4 w-4" weight="bold" />
                {t('artifacts.back')}
              </button>
            ) : (
              <h2 className="truncate text-sm font-semibold text-gray-900 dark:text-gray-100">
                {t('artifacts.title')} ({artifacts.length})
              </h2>
            )}
          </div>
        </header>

        <div className="min-h-0 flex-1 overflow-y-auto px-3 py-3">
          {fetching ? (
            <div className="flex h-full items-center justify-center text-sm text-gray-500 dark:text-gray-400">
              {t('artifacts.loading')}
            </div>
          ) : error ? (
            <div className="space-y-3 rounded-lg border border-red-200 bg-red-50 p-3 dark:border-red-900/70 dark:bg-red-950/30">
              <p className="text-sm font-medium text-red-800 dark:text-red-300">{t('artifacts.failedToLoad')}</p>
              <p className="text-xs text-red-700 dark:text-red-400">{error}</p>
              <button
                type="button"
                onClick={() => {
                  void fetchArtifacts({ reset: true, manual: true })
                }}
                className="cursor-pointer rounded-md border border-red-300 px-2 py-1 text-xs font-medium text-red-700 hover:bg-red-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-red-400/50 dark:border-red-800 dark:text-red-300 dark:hover:bg-red-900/40"
              >
                {t('alert.retry')}
              </button>
            </div>
          ) : !canFetchArtifacts ? (
            <div className="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-900/70 dark:bg-amber-950/30 dark:text-amber-200">
              {t('artifacts.unsupported')}
            </div>
          ) : selectedArtifact ? (
            <SessionArtifactPreview artifact={selectedArtifact} />
          ) : (
            <>
              <SessionArtifactList
                artifacts={artifacts}
                selectedArtifactId={selectedArtifactId || undefined}
                onSelect={(artifact) => setSelectedArtifactId(artifact.id)}
              />

              {hasMore && (
                <div className="mt-3 flex justify-center">
                  <button
                    type="button"
                    onClick={() => {
                      void fetchArtifacts({ reset: false, manual: true })
                    }}
                    disabled={loadingMore || refreshing || fetching}
                    className="cursor-pointer rounded-md border border-gray-200 px-3 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-gray-700 dark:text-gray-200 dark:hover:bg-gray-800"
                  >
                    {loadingMore ? t('artifacts.loadingMore') : t('artifacts.loadMore')}
                  </button>
                </div>
              )}
            </>
          )}
        </div>
    </aside>
  )
}

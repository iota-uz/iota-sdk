interface SessionSkeletonProps {
  count?: number
}

/**
 * Loading skeleton for session list
 * Displays animated placeholders while sessions are loading
 */
export default function SessionSkeleton({ count = 5 }: SessionSkeletonProps) {
  return (
    <div className="space-y-1 px-2">
      {Array.from({ length: count }).map((_, index) => (
        <div
          key={index}
          className="animate-pulse px-3 py-2 rounded-lg"
        >
          <div className="flex items-center gap-2">
            {/* Icon placeholder */}
            <div className="w-5 h-5 bg-gray-300 dark:bg-gray-700 rounded" />
            {/* Text placeholder */}
            <div className="flex-1">
              <div className="h-4 bg-gray-300 dark:bg-gray-700 rounded w-3/4" />
            </div>
          </div>
        </div>
      ))}
    </div>
  )
}

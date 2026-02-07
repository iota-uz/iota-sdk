interface DateGroupHeaderProps {
  groupName: string
  count: number
}

/**
 * Sticky header for date-based session groups
 * Displays group name and session count
 */
export default function DateGroupHeader({ groupName, count }: DateGroupHeaderProps) {
  return (
    <div className="sticky top-0 bg-surface-300 dark:bg-gray-900 px-4 py-2 text-sm font-medium z-10 border-b border-gray-100 dark:border-gray-800">
      <div className="flex items-center justify-between">
        <span className="text-gray-700 dark:text-gray-300 font-semibold">{groupName}</span>
        <span className="text-xs text-gray-500 dark:text-gray-400 bg-gray-100 dark:bg-gray-800 px-2 py-0.5 rounded-full">
          {count}
        </span>
      </div>
    </div>
  )
}

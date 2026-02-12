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
    <div className="sticky top-0 bg-white/95 dark:bg-gray-900/95 backdrop-blur-sm px-4 py-2 text-sm font-medium z-10 border-b border-gray-100 dark:border-gray-800/80">
      <div className="flex items-center justify-between">
        <span className="text-gray-600 dark:text-gray-400 font-medium">{groupName}</span>
        <span className="text-[10px] text-gray-400 dark:text-gray-500 bg-gray-100 dark:bg-gray-800 px-1.5 py-0.5 rounded-md font-medium tabular-nums">
          {count}
        </span>
      </div>
    </div>
  )
}

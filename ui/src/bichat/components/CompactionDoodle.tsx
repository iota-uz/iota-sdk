interface CompactionDoodleProps {
  title: string
  subtitle: string
}

export function CompactionDoodle({ title, subtitle }: CompactionDoodleProps) {
  return (
    <div className="rounded-2xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 p-4 shadow-sm">
      <div className="flex items-center gap-3">
        <div className="relative w-10 h-10">
          <div className="absolute inset-0 rounded-full bg-primary-500/20 animate-pulse motion-reduce:animate-none" />
          <div className="absolute inset-1 rounded-full bg-primary-500/40 animate-pulse motion-reduce:animate-none" />
          <div className="absolute inset-3 rounded-full bg-primary-600" />
        </div>
        <div>
          <p className="text-sm font-medium text-gray-900 dark:text-gray-100">{title}</p>
          <p className="text-xs text-gray-500 dark:text-gray-400">{subtitle}</p>
        </div>
      </div>
    </div>
  )
}

export default CompactionDoodle

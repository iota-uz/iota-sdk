

export type Scenario = {
  name: string
  description?: string
  content: React.ReactNode
}

export function ScenarioGrid({
  scenarios,
  columns = 2,
  className = '',
}: {
  scenarios: Scenario[]
  columns?: 1 | 2 | 3
  className?: string
}) {
  const gridCols =
    columns === 1 ? 'grid-cols-1' : columns === 3 ? 'grid-cols-1 lg:grid-cols-3' : 'grid-cols-1 lg:grid-cols-2'

  return (
    <div className={['grid gap-4', gridCols, className].filter(Boolean).join(' ')}>
      {scenarios.map((s) => (
        <section
          key={s.name}
          className="rounded-xl border border-gray-200 dark:border-gray-800 bg-white/70 dark:bg-gray-900/40 shadow-sm overflow-hidden"
        >
          <header className="px-4 py-3 border-b border-gray-200 dark:border-gray-800">
            <div className="font-semibold text-sm text-gray-900 dark:text-gray-100">{s.name}</div>
            {s.description ? (
              <div className="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{s.description}</div>
            ) : null}
          </header>
          <div className="p-4">{s.content}</div>
        </section>
      ))}
    </div>
  )
}


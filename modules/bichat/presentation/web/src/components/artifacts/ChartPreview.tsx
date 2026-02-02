import type { Artifact } from '../../types/artifacts'
import { WarningCircle } from '@phosphor-icons/react'

interface ChartPreviewProps {
  artifact: Artifact
}

export function ChartPreview({ artifact }: ChartPreviewProps) {
  const spec = artifact.metadata?.spec

  if (!spec) {
    return (
      <div className="flex items-center gap-2 p-4 bg-yellow-50 border border-yellow-200 rounded-lg">
        <WarningCircle className="w-5 h-5 text-yellow-600" weight="duotone" />
        <span className="text-sm text-yellow-800">Chart specification not available</span>
      </div>
    )
  }

  // TODO: Implement Vega-Lite rendering
  // For now, show a placeholder with the spec
  return (
    <div className="space-y-4">
      <div className="bg-gray-100 rounded-lg p-4">
        <div className="text-center text-gray-500 py-8">
          <p className="text-sm font-medium mb-2">Chart Preview</p>
          <p className="text-xs">Vega-Lite rendering not yet implemented</p>
        </div>
      </div>

      <details className="text-sm">
        <summary className="cursor-pointer text-gray-700 font-medium mb-2">
          Chart Specification
        </summary>
        <pre className="bg-gray-50 p-3 rounded border border-gray-200 overflow-auto text-xs">
          {JSON.stringify(spec, null, 2)}
        </pre>
      </details>
    </div>
  )
}

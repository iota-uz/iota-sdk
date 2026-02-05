/**
 * ChartCard Component
 * Renders chart visualizations using ApexCharts
 *
 * Supports multiple chart types: line, bar, pie, area, donut
 * Includes PNG export functionality and responsive styling
 */

import { useState, useId } from 'react'
import ReactApexChart from 'react-apexcharts'
import ApexCharts, { ApexOptions } from 'apexcharts'
import { Download } from '@phosphor-icons/react'
import type { ChartData } from '../types'

interface ChartCardProps {
  chartData: ChartData
}

// Default color palette if none provided
const DEFAULT_COLORS = ['#008FFB', '#00E396', '#FEB019', '#FF4560', '#775DD0']

/**
 * ChartCard renders a single chart visualization with optional PNG export.
 *
 * Chart types:
 * - line: Line chart with multiple series
 * - bar: Bar chart with multiple series
 * - area: Area chart with multiple series (filled)
 * - pie: Pie chart (single series)
 * - donut: Donut chart (single series)
 *
 * @param chartData - Chart specification from GraphQL API
 */
export function ChartCard({ chartData }: ChartCardProps) {
  // Generate unique chart ID using React's useId hook
  const chartId = useId().replace(/:/g, '_')
  const [isExporting, setIsExporting] = useState(false)

  const { chartType, title, series, labels, colors, height = 350 } = chartData

  // Validate chart data to prevent ApexCharts crash
  // Series must exist, have at least one series, and that series must have data
  const hasValidData =
    series && series.length > 0 && series.some((s) => s.data && s.data.length > 0)

  if (!hasValidData) {
    return (
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-[var(--bichat-border)] p-4 my-2 shadow-sm w-full max-w-full">
        <p className="text-gray-500 dark:text-gray-400 text-sm">
          {title && <span className="font-medium">{title}: </span>}
          No data available for chart visualization.
        </p>
      </div>
    )
  }

  // Map series to ApexCharts format
  // Pie/donut charts expect flat array of values, other charts expect array of series objects
  const apexSeries =
    chartType === 'pie' || chartType === 'donut'
      ? series[0]?.data ?? [] // Pie/donut use flat array
      : series.map((s) => ({ name: s.name, data: s.data }))

  // Build xaxis config for non-pie charts
  const xaxisConfig =
    chartType !== 'pie' && chartType !== 'donut'
      ? { categories: (labels ?? []).filter((l): l is string => l !== null) }
      : {}

  // Build labels config for pie/donut charts
  const labelsConfig =
    chartType === 'pie' || chartType === 'donut'
      ? (labels ?? []).filter((l): l is string => l !== null)
      : []

  const options: ApexOptions = {
    chart: {
      id: chartId,
      type: chartType as 'line' | 'bar' | 'area' | 'pie' | 'donut',
      toolbar: { show: false },
      animations: { enabled: false },
    },
    title: {
      text: title,
      align: 'left',
      style: { fontSize: '14px', fontWeight: 600 },
    },
    colors: colors?.length ? colors : DEFAULT_COLORS,
    xaxis: xaxisConfig,
    labels: labelsConfig,
    legend: { position: 'bottom', horizontalAlign: 'center' },
    dataLabels: { enabled: chartType === 'pie' || chartType === 'donut' },
    stroke: {
      curve: 'smooth',
      width: chartType === 'line' || chartType === 'area' ? 2 : 0,
    },
    fill: { opacity: chartType === 'area' ? 0.4 : 1 },
  }

  /**
   * Export chart as PNG image
   * Uses ApexCharts.getChartByID to access chart instance and dataURI method
   */
  const handleExportPNG = async () => {
    setIsExporting(true)

    try {
      // Access ApexCharts instance via chart ID
      const chart = ApexCharts.getChartByID(chartId)
      if (!chart) {
        console.error('Chart instance not available')
        setIsExporting(false)
        return
      }
      const result = await chart.dataURI({ scale: 2 })

      // dataURI returns { imgURI } when successful
      if (!('imgURI' in result)) {
        console.error('Unexpected dataURI result format')
        return
      }

      // Create download link and trigger download
      const link = document.createElement('a')
      link.href = result.imgURI
      link.download = `${title.replace(/[^a-z0-9]/gi, '_').toLowerCase()}_chart.png`
      link.click()
    } catch (error) {
      console.error('Failed to export chart:', error)
    } finally {
      setIsExporting(false)
    }
  }

  // Fixed width to fit within max-w-2xl container (672px - 32px padding)
  const chartWidth = 600

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg border border-[var(--bichat-border)] p-4 my-2 shadow-sm w-full max-w-[632px] min-w-0 overflow-hidden">
      <div className="w-full min-w-0">
        <ReactApexChart
          options={options}
          series={apexSeries}
          type={chartType}
          width={chartWidth}
          height={height}
        />
      </div>
      <div className="flex justify-end mt-2">
        <button
          onClick={handleExportPNG}
          disabled={isExporting}
          className="text-sm text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200 flex items-center gap-1 transition-colors"
          title="Download chart as PNG"
        >
          {isExporting ? (
            <span>Exporting...</span>
          ) : (
            <>
              <Download className="w-4 h-4" />
              Download PNG
            </>
          )}
        </button>
      </div>
    </div>
  )
}

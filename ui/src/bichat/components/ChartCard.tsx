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
import { DownloadSimple } from '@phosphor-icons/react'
import type { ChartData } from '../types'
import { useTranslation } from '../hooks/useTranslation'

interface ChartCardProps {
  chartData: ChartData
}

// Default color palette if none provided
const DEFAULT_COLORS = ['#6366f1', '#06b6d4', '#f59e0b', '#ef4444', '#8b5cf6']

/**
 * ChartCard renders a single chart visualization with optional PNG export.
 */
export function ChartCard({ chartData }: ChartCardProps) {
  const { t } = useTranslation()
  const chartId = useId().replace(/:/g, '_')
  const [isExporting, setIsExporting] = useState(false)

  const { chartType, title, series, labels, colors, height = 350 } = chartData

  const hasValidData =
    series && series.length > 0 && series.some((s) => s.data && s.data.length > 0)

  if (!hasValidData) {
    return (
      <div className="rounded-xl border border-gray-200/80 bg-white p-4 shadow-sm dark:border-gray-700/60 dark:bg-gray-800">
        <p className="text-sm text-gray-500 dark:text-gray-400">
          {title && <span className="font-medium">{title}: </span>}
          {t('chart.noData')}
        </p>
      </div>
    )
  }

  const apexSeries =
    chartType === 'pie' || chartType === 'donut'
      ? series[0]?.data ?? []
      : series.map((s) => ({ name: s.name, data: s.data }))

  const xaxisConfig =
    chartType !== 'pie' && chartType !== 'donut'
      ? { categories: (labels ?? []).filter((l): l is string => l !== null) }
      : {}

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
      fontFamily: 'inherit',
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
    grid: {
      borderColor: 'rgba(148, 163, 184, 0.15)',
      strokeDashArray: 3,
    },
  }

  const handleExportPNG = async () => {
    setIsExporting(true)

    try {
      const chart = ApexCharts.getChartByID(chartId)
      if (!chart) {
        console.error('Chart instance not available')
        setIsExporting(false)
        return
      }
      const result = await chart.dataURI({ scale: 2 })

      if (!('imgURI' in result)) {
        console.error('Unexpected dataURI result format')
        return
      }

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

  return (
    <div className="group/chart rounded-xl border border-gray-200/80 bg-white p-4 shadow-sm transition-shadow duration-200 hover:shadow dark:border-gray-700/60 dark:bg-gray-800">
      <div className="w-full min-w-0">
        <ReactApexChart
          options={options}
          series={apexSeries}
          type={chartType}
          width="100%"
          height={height}
        />
      </div>
      <div className="flex justify-end pt-2">
        <button
          type="button"
          onClick={handleExportPNG}
          disabled={isExporting}
          className="inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-xs font-medium text-gray-400 opacity-0 transition-all duration-150 hover:bg-gray-100 hover:text-gray-600 focus-visible:opacity-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 group-hover/chart:opacity-100 disabled:opacity-50 dark:text-gray-500 dark:hover:bg-gray-700 dark:hover:text-gray-300"
          title={t('chart.download')}
        >
          {isExporting ? (
            <span className="text-gray-500 dark:text-gray-400">{t('chart.exporting')}</span>
          ) : (
            <>
              <DownloadSimple className="h-3.5 w-3.5" weight="bold" />
              <span>{t('chart.downloadPNG')}</span>
            </>
          )}
        </button>
      </div>
    </div>
  )
}

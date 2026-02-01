'use client'

import React from 'react'

interface BarChartData {
  label: string
  value: number
}

interface BarChartProps {
  title: string
  data: BarChartData[]
  maxValue?: number
}

export const BarChart = ({ title, data, maxValue }: BarChartProps) => {
  const max = maxValue || Math.max(...data.map((d) => d.value), 1)

  return (
    <div className="rounded-lg border border-gray-200 dark:border-gray-700 p-6 bg-white dark:bg-gray-950">
      <h3 className="font-semibold text-lg text-gray-900 dark:text-gray-100 mb-6">{title}</h3>

      <div className="space-y-6">
        {data.map((item, index) => {
          const percentage = (item.value / max) * 100
          const clampedPercentage = Math.min(100, percentage)

          return (
            <div key={index}>
              <div className="flex justify-between items-center mb-2">
                <span className="text-sm font-medium text-gray-700 dark:text-gray-300">{item.label}</span>
                <span className="text-sm font-semibold text-gray-900 dark:text-gray-100">{item.value}</span>
              </div>

              <div className="h-8 bg-gray-100 dark:bg-gray-800 rounded-lg overflow-hidden">
                <div
                  className="h-full bg-gradient-to-r from-blue-400 to-blue-600 dark:from-blue-500 dark:to-blue-700 transition-all duration-300 ease-out flex items-center justify-end pr-2"
                  style={{ width: `${clampedPercentage}%` }}
                >
                  {clampedPercentage > 15 && (
                    <span className="text-xs font-semibold text-white">{clampedPercentage.toFixed(0)}%</span>
                  )}
                </div>
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}

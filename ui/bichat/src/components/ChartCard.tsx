/**
 * ChartCard component
 * Renders charts using recharts
 */

import {
  BarChart,
  Bar,
  LineChart,
  Line,
  PieChart,
  Pie,
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
  Cell,
} from 'recharts'
import { ChartData } from '../types'

interface ChartCardProps {
  chartData: ChartData
}

const COLORS = ['#3b82f6', '#10b981', '#f59e0b', '#ef4444', '#8b5cf6', '#ec4899']

export function ChartCard({ chartData }: ChartCardProps) {
  const { type, title, data, xAxisKey = 'name', yAxisKey = 'value' } = chartData

  const renderChart = () => {
    switch (type) {
      case 'bar':
        return (
          <BarChart data={data}>
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis dataKey={xAxisKey} />
            <YAxis />
            <Tooltip />
            <Legend />
            <Bar dataKey={yAxisKey} fill="#3b82f6" />
          </BarChart>
        )
      case 'line':
        return (
          <LineChart data={data}>
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis dataKey={xAxisKey} />
            <YAxis />
            <Tooltip />
            <Legend />
            <Line type="monotone" dataKey={yAxisKey} stroke="#3b82f6" />
          </LineChart>
        )
      case 'pie':
        return (
          <PieChart>
            <Pie
              data={data}
              dataKey={yAxisKey}
              nameKey={xAxisKey}
              cx="50%"
              cy="50%"
              outerRadius={80}
              label
            >
              {data.map((entry, index) => (
                <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
              ))}
            </Pie>
            <Tooltip />
            <Legend />
          </PieChart>
        )
      case 'area':
        return (
          <AreaChart data={data}>
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis dataKey={xAxisKey} />
            <YAxis />
            <Tooltip />
            <Legend />
            <Area type="monotone" dataKey={yAxisKey} stroke="#3b82f6" fill="#3b82f6" />
          </AreaChart>
        )
      default:
        return <div>Unsupported chart type</div>
    }
  }

  return (
    <div className="border border-[var(--bichat-border)] rounded-lg p-4 bg-white">
      {title && <h3 className="text-lg font-semibold mb-4">{title}</h3>}
      <ResponsiveContainer width="100%" height={300}>
        {renderChart()}
      </ResponsiveContainer>
    </div>
  )
}

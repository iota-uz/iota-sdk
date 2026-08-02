import { describe, expect, it } from 'vitest'
import type { ChartInput } from '../adapter'
import { buildMapOption } from './options'
import type { EChartsTheme } from './theme'

const theme: EChartsTheme = {
  card: '#ffffff', text: '#334155', mutedText: '#64748b', border: '#e2e8f0', divider: '#f1f5f9',
  faintText: '#94a3b8', warn: '#d97706', warnSoft: '#fffbeb', accent: '#2563eb', trend: '#7c3aed',
  selectedBorder: '#0f172a', fontFamily: 'sans-serif',
  popoverShadow: '0 1px 2px rgba(0,0,0,0.1)', cardRadius: 8, type: { xs: 10, sm: 11, base: 12, md: 14 },
  colors: ['#2563eb'], seriesColor: () => undefined,
}

describe('choropleth option', () => {
  it('uses the exact feature property join and preserves stable node keys', () => {
    const input: ChartInput = {
      kind: 'map',
      frame: {
        columns: [{ name: 'code', type: 'string' }, { name: 'label', type: 'string' }, { name: 'value', type: 'number' }],
        rows: [['north', 'North district', 42], ['south', 'South district', 18]],
      },
      encoding: { id: 'code', label: 'label', value: 'value' },
      format: (_field, value) => `${String(value)} policies`,
      theme: { palette: { accent: '#2563eb' }, series: {} },
      map: {
        name: 'synthetic', featureProperty: 'code', labelProperty: 'name',
        geoJSON: {
          type: 'FeatureCollection',
          features: [
            { type: 'Feature', properties: { code: 'north', name: 'North' }, geometry: { type: 'Polygon', coordinates: [] } },
            { type: 'Feature', properties: { code: 'south', name: 'South' }, geometry: { type: 'Polygon', coordinates: [] } },
          ],
        },
      },
    }

    const option = buildMapOption(input, theme) as {
      visualMap: { min: number; max: number }
      series: Array<{
        map: string
        nameProperty: string
        data: Array<Record<string, unknown>>
        label: { padding: number[]; formatter: (params: { name?: string }) => string }
        labelLayout: { hideOverlap: boolean }
      }>
    }
    // The colour domain is the rank the series carries, not the amount: one
    // region three orders of magnitude above the rest would otherwise flatten
    // every other region onto the pale end of a linear ramp.
    expect(option.visualMap).toMatchObject({ min: 0, max: 1 })
    expect(option.series[0]).toMatchObject({ map: 'synthetic', nameProperty: 'code' })
    expect(option.series[0]!.labelLayout).toEqual({ hideOverlap: true })
    expect(option.series[0]!.label.padding).toEqual([4, 8])
    expect(option.series[0]!.data).toEqual([
      { name: 'north', value: 1, amount: 42, nodeKey: 'north', displayLabel: 'North district' },
      { name: 'south', value: 0, amount: 18, nodeKey: 'south', displayLabel: 'South district' },
    ])
    expect(option.series[0]!.label.formatter({ name: 'north' })).toBe('North district')
  })

  it.each([
    ['ru', 'nameRu', 'Север из фрейма', 'Юг'],
    ['uz', 'nameUz', 'Frame shimoli', 'Janub'],
    ['oz', 'nameOz', 'Frame шимоли', 'Жануб'],
    ['en', 'nameEn', 'Frame north', 'South'],
  ])('keeps present and absent %s region labels in one locale', (_locale, labelProperty, presentLabel, absentLabel) => {
    const input: ChartInput = {
      kind: 'map',
      frame: {
        columns: [{ name: 'code', type: 'string' }, { name: 'label', type: 'string' }, { name: 'value', type: 'number' }],
        rows: [['north', presentLabel, 42]],
      },
      encoding: { id: 'code', label: 'label', value: 'value' },
      format: (_field, value) => String(value),
      theme: { palette: {}, series: {} },
      map: {
        name: 'localized', featureProperty: 'code', labelProperty,
        geoJSON: {
          type: 'FeatureCollection',
          features: [
            {
              type: 'Feature',
              properties: {
                code: 'north', nameRu: 'Север', nameUz: 'Shimol', nameOz: 'Шимол', nameEn: 'North',
              },
              geometry: { type: 'Polygon', coordinates: [] },
            },
            {
              type: 'Feature',
              properties: {
                code: 'south', nameRu: 'Юг', nameUz: 'Janub', nameOz: 'Жануб', nameEn: 'South',
              },
              geometry: { type: 'Polygon', coordinates: [] },
            },
          ],
        },
      },
    }

    const option = buildMapOption(input, theme) as {
      tooltip: { formatter: (params: { name?: string; data?: { displayLabel?: string; value?: unknown } }) => string }
      series: Array<{ label: { formatter: (params: { name?: string }) => string } }>
    }
    expect(option.series[0]!.label.formatter({ name: 'north' })).toBe(presentLabel)
    expect(option.series[0]!.label.formatter({ name: 'south' })).toBe(absentLabel)
    expect(option.tooltip.formatter({ name: 'south' })).toContain(absentLabel)
    expect(option.tooltip.formatter({ name: 'south' })).not.toContain('south<br>')
  })
})

describe('choropleth chrome', () => {
  const geo = {
    type: 'FeatureCollection',
    features: [
      { type: 'Feature', properties: { code: 'north', name: 'North' }, geometry: { type: 'Polygon', coordinates: [] } },
      { type: 'Feature', properties: { code: 'south', name: 'South' }, geometry: { type: 'Polygon', coordinates: [] } },
    ],
  }

  function mapInput(): ChartInput {
    return {
      kind: 'map',
      frame: {
        columns: [{ name: 'code', type: 'string' }, { name: 'value', type: 'number' }],
        rows: [['north', 42]],
      },
      encoding: { id: 'code', value: 'value' },
      format: (_field, value) => typeof value === 'number' ? `$${value}` : '—',
      theme: { palette: { accent: '#2563eb' }, series: {} },
      labels: {
        previous: '', trend: '', movingAverage: () => '', estimate: '', ytd: '', forecast: '',
        forecastLower: () => '', forecastConfidence: () => '',
        boxplot: ['', '', '', '', ''],
        noData: 'Нет данных',
      },
      map: { name: 'regions', geoJSON: geo as never, featureProperty: 'code', labelProperty: 'name' },
    }
  }

  it('paints hover in the accent instead of ECharts’ amber default', () => {
    const option = buildMapOption(mapInput(), theme) as {
      series: Array<{ emphasis: { itemStyle: { areaColor?: string; borderColor?: string; borderWidth?: number } } }>
    }
    const emphasis = option.series[0]!.emphasis.itemStyle

    expect(emphasis.areaColor).toMatch(/^#[0-9a-f]{6}$/)
    expect(emphasis.areaColor).not.toBe(theme.card)
    expect(emphasis.borderColor).toBe('#2563eb')
    expect(emphasis.borderWidth).toBe(1.5)
  })

  // Прод-форма данных: Ташкент 31 415 полисов, следующий регион — 10.
  it('keeps regions distinguishable when one dwarfs the rest', () => {
    const input: ChartInput = {
      kind: 'map',
      frame: {
        columns: [{ name: 'code', type: 'string' }, { name: 'label', type: 'string' }, { name: 'value', type: 'number' }],
        rows: [['north', 'North', 31_415], ['south', 'South', 10], ['east', 'East', 5], ['west', 'West', 1]],
      },
      encoding: { id: 'code', label: 'label', value: 'value' },
      format: (_field, value) => `${String(value)} policies`,
      theme: { palette: { accent: '#2563eb' }, series: {} },
      map: {
        name: 'synthetic', featureProperty: 'code', labelProperty: 'name',
        geoJSON: { type: 'FeatureCollection', features: [] },
      },
    }
    const option = buildMapOption(input, theme) as {
      series: Array<{ data: Array<{ value: number }> }>
    }
    const shades = option.series[0]!.data.map((item) => item.value)

    // Linear in the amount these were 1, 0.0003, 0.0001, 0 — one region shaded
    // and three indistinguishable from each other and from "no data".
    expect(shades).toEqual([1, 2 / 3, 1 / 3, 0])
  })

  it('says a region has no reading rather than formatting one it never had', () => {
    const option = buildMapOption(mapInput(), theme) as {
      tooltip: { formatter: (params: { name?: string; data?: { amount?: unknown } }) => string }
    }

    expect(option.tooltip.formatter({ name: 'south' })).toContain('Нет данных')
    expect(option.tooltip.formatter({ name: 'north', data: { amount: 42 } })).toContain('$42')
  })
})

describe('choropleth naming', () => {
  it('names a feature the locale never translated from the geometry default', () => {
    const input: ChartInput = {
      kind: 'map',
      frame: {
        columns: [{ name: 'code', type: 'string' }, { name: 'value', type: 'number' }],
        rows: [['north', 42], ['south', 18]],
      },
      encoding: { id: 'code', value: 'value' },
      format: (_field, value) => `${String(value)} policies`,
      theme: { palette: { accent: '#2563eb' }, series: {} },
      map: {
        name: 'synthetic', featureProperty: 'code', labelProperty: 'nameRu', fallbackLabelProperty: 'nameEn',
        geoJSON: {
          type: 'FeatureCollection',
          features: [
            { type: 'Feature', properties: { code: 'north', nameEn: 'North', nameRu: 'Север' }, geometry: { type: 'Polygon', coordinates: [] } },
            // The boundary file carries no Russian name for this one. Before
            // the fallback existed that was fatal to the whole panel.
            { type: 'Feature', properties: { code: 'south', nameEn: 'South' }, geometry: { type: 'Polygon', coordinates: [] } },
          ],
        },
      },
    }

    const option = buildMapOption(input, theme) as {
      series: Array<{ label: { formatter: (params: { name?: string }) => string } }>
    }
    const formatter = option.series[0]!.label.formatter
    expect(formatter({ name: 'north' })).toBe('Север')
    expect(formatter({ name: 'south' })).toBe('South')
  })
})

import { describe, expect, it } from 'vitest'
import type { ChartInput } from '../adapter'
import { buildMapOption } from './options'
import type { EChartsTheme } from './theme'

const theme: EChartsTheme = {
  card: '#ffffff', text: '#334155', mutedText: '#64748b', border: '#e2e8f0', divider: '#f1f5f9',
  selectedBorder: '#0f172a', fontFamily: 'sans-serif', colors: ['#2563eb'], seriesColor: () => undefined,
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
        label: { formatter: (params: { name?: string }) => string }
      }>
    }
    expect(option.visualMap).toMatchObject({ min: 18, max: 42 })
    expect(option.series[0]).toMatchObject({ map: 'synthetic', nameProperty: 'code' })
    expect(option.series[0]!.data).toEqual([
      { name: 'north', value: 42, nodeKey: 'north', displayLabel: 'North district' },
      { name: 'south', value: 18, nodeKey: 'south', displayLabel: 'South district' },
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
      series: Array<{ label: { formatter: (params: { name?: string }) => string } }>
    }
    expect(option.series[0]!.label.formatter({ name: 'north' })).toBe(presentLabel)
    expect(option.series[0]!.label.formatter({ name: 'south' })).toBe(absentLabel)
  })
})

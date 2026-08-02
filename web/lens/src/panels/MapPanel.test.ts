import { describe, expect, it, vi } from 'vitest'
import type { Frame, GeoJSONFeatureCollection, Panel } from '../contract'
import { loadMapGeometry, mapJoinError, resolveMapLabelProperty } from './MapPanel'

const geometry: GeoJSONFeatureCollection = {
  type: 'FeatureCollection',
  features: [
    { type: 'Feature', properties: { code: 'north', name: 'North' }, geometry: { type: 'Polygon', coordinates: [[[0, 0], [2, 0], [2, 2], [0, 0]]] } },
    { type: 'Feature', properties: { code: 'south', name: 'South' }, geometry: { type: 'Polygon', coordinates: [[[0, 2], [2, 2], [2, 4], [0, 2]]] } },
  ],
}

const panel: Panel = {
  id: 'regions', kind: 'map', title: 'Regions', semantics: 'series', frame: 'regions',
  encoding: { id: 'code', label: 'name', value: 'premium' }, format: {}, terminal: true, actions: [],
  map: { source: { inline: geometry }, featureProperty: 'code', labelProperty: 'name' },
}

const frame: Frame = {
  columns: [{ name: 'code', type: 'string' }, { name: 'name', type: 'string' }, { name: 'premium', type: 'number' }],
  rows: [['north', 'North', 42], ['south', 'South', 18]],
}

describe('map geometry source', () => {
  it('accepts a synthetic inline Polygon collection', async () => {
    await expect(loadMapGeometry({ inline: geometry }, 'code')).resolves.toEqual(geometry)
  })

  it('fetches only a bounded same-origin JSON response', async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(new Response(JSON.stringify(geometry), {
      status: 200, headers: { 'Content-Type': 'application/geo+json' },
    }))
    await expect(loadMapGeometry({ url: '/maps/synthetic.geojson', maxBytes: 4096 }, 'code', fetcher)).resolves.toEqual(geometry)
    expect(fetcher).toHaveBeenCalledWith('/maps/synthetic.geojson', expect.objectContaining({ credentials: 'same-origin' }))
  })

  it('rejects cross-origin, oversized, and malformed geometry', async () => {
    const fetcher = vi.fn<typeof fetch>()
    await expect(loadMapGeometry({ url: 'https://example.com/map.geojson', maxBytes: 4096 }, 'code', fetcher)).rejects.toThrow('same-origin')
    expect(fetcher).not.toHaveBeenCalled()

    const oversized = vi.fn<typeof fetch>().mockResolvedValue(new Response('{}', {
      status: 200, headers: { 'Content-Type': 'application/json', 'Content-Length': '5000' },
    }))
    await expect(loadMapGeometry({ url: '/map.geojson', maxBytes: 100 }, 'code', oversized)).rejects.toThrow('exceeds 100 bytes')

    await expect(loadMapGeometry({ inline: { ...geometry, features: [{ ...geometry.features[0]!, geometry: { type: 'Point', coordinates: [0, 0] } }] } }, 'code')).rejects.toThrow('Polygon or MultiPolygon')
  })
})

describe('map region join', () => {
  it('joins frame IDs to feature properties exactly', () => {
    expect(mapJoinError(panel, frame, geometry)).toBeUndefined()
  })

  it('rejects missing and duplicate region identities', () => {
    expect(mapJoinError(panel, { ...frame, rows: [['west', 'West', 10]] }, geometry)?.message).toContain('has no GeoJSON feature')
    expect(mapJoinError(panel, { ...frame, rows: [['north', 'North', 42], ['north', 'North again', 18]] }, geometry)?.message).toContain('duplicate region key')
  })
})

describe('localized map labels', () => {
  const localizedPanel = {
    ...panel.map!,
    labelProperties: { ru: 'nameRu', uz: 'nameUz', oz: 'nameOz', en: 'nameEn', 'ru-UZ': 'nameRuUz' },
  }

  it.each([
    ['ru-UZ', 'nameRuUz'],
    ['ru-RU', 'nameRu'],
    ['uz-UZ', 'nameUz'],
    ['oz-UZ', 'nameOz'],
    ['en-US', 'nameEn'],
  ])('resolves %s by exact locale and then base language', (locale, expected) => {
    expect(resolveMapLabelProperty(localizedPanel, locale)).toBe(expected)
  })

  it('falls back to the legacy label property for an unmapped locale', () => {
    expect(resolveMapLabelProperty(localizedPanel, 'de-DE')).toBe('name')
  })
})

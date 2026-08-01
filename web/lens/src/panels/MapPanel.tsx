/* eslint-disable react-refresh/only-export-components */
import { useCallback, useEffect, useMemo, useState } from 'react'
import type { Frame, GeoJSONFeatureCollection, GeoJSONSource, MapConfig, NodeKey, Panel } from '../contract'
import type { ChartActivation, ChartAdapter, ChartInput, ChartFormatResolver } from '../charts/adapter'
import { useDashboard, useDrill, useFormat, usePanelFrame, useTranslate, type PanelFrameState } from '../runtime'
import { ChartDataEquivalent } from './ChartDataEquivalent'
import { ChartHost } from './ChartHost'
import { usePanelNavigation } from './actions'
import { PanelFrame } from './PanelFrame'

export const MAX_MAP_GEOJSON_BYTES = 5 * 1024 * 1024

/** Selects the localized GeoJSON label without ever mixing in another locale. */
export function resolveMapLabelProperty(config: MapConfig, locale: string): string | undefined {
  const exact = locale.trim()
  const base = exact.split(/[-_]/, 1)[0] ?? ''
  return config.labelProperties?.[exact]
    ?? config.labelProperties?.[exact.toLowerCase()]
    ?? config.labelProperties?.[base]
    ?? config.labelProperties?.[base.toLowerCase()]
    ?? config.labelProperty
}

function object(value: unknown): Record<string, unknown> | undefined {
  return value !== null && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : undefined
}

function validateGeometry(input: unknown, featureProperty: string, labelProperty?: string): GeoJSONFeatureCollection {
  const collection = object(input)
  if (collection?.type !== 'FeatureCollection' || !Array.isArray(collection.features) || collection.features.length === 0) {
    throw new Error('Map geometry must be a non-empty GeoJSON FeatureCollection')
  }
  const keys = new Set<string>()
  for (const [index, rawFeature] of collection.features.entries()) {
    const feature = object(rawFeature)
    const properties = object(feature?.properties)
    const geometry = object(feature?.geometry)
    if (feature?.type !== 'Feature' || !properties || !geometry) throw new Error(`Map feature ${index} is invalid`)
    const key = properties[featureProperty]
    if (typeof key !== 'string' || key.trim() === '') throw new Error(`Map feature ${index} has no string ${featureProperty} key`)
    if (keys.has(key)) throw new Error(`Map geometry has duplicate region key ${key}`)
    keys.add(key)
    if (labelProperty && (typeof properties[labelProperty] !== 'string' || String(properties[labelProperty]).trim() === '')) {
      throw new Error(`Map feature ${key} has no string ${labelProperty} label`)
    }
    if ((geometry.type !== 'Polygon' && geometry.type !== 'MultiPolygon') || geometry.coordinates === undefined) {
      throw new Error(`Map feature ${key} must use Polygon or MultiPolygon geometry`)
    }
  }
  return input as GeoJSONFeatureCollection
}

async function readBounded(response: Response, maxBytes: number): Promise<string> {
  const declared = Number(response.headers.get('Content-Length'))
  if (Number.isFinite(declared) && declared > maxBytes) throw new Error(`Map geometry exceeds ${maxBytes} bytes`)
  if (!response.body) {
    const bytes = new Uint8Array(await response.arrayBuffer())
    if (bytes.byteLength > maxBytes) throw new Error(`Map geometry exceeds ${maxBytes} bytes`)
    return new TextDecoder().decode(bytes)
  }
  const reader = response.body.getReader()
  const chunks: Uint8Array[] = []
  let size = 0
  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    size += value.byteLength
    if (size > maxBytes) {
      await reader.cancel()
      throw new Error(`Map geometry exceeds ${maxBytes} bytes`)
    }
    chunks.push(value)
  }
  const bytes = new Uint8Array(size)
  let offset = 0
  for (const chunk of chunks) {
    bytes.set(chunk, offset)
    offset += chunk.byteLength
  }
  return new TextDecoder().decode(bytes)
}

/** Loads only an inline collection or a bounded, same-origin URL. */
export async function loadMapGeometry(
  source: GeoJSONSource,
  featureProperty: string,
  labelProperty?: string,
  fetcher: typeof fetch = fetch,
  signal?: AbortSignal,
): Promise<GeoJSONFeatureCollection> {
  const inline = source.inline !== undefined
  const remote = typeof source.url === 'string' && source.url.trim() !== ''
  if (inline === remote) throw new Error('Map source requires exactly one of inline or URL')
  if (inline) return validateGeometry(source.inline, featureProperty, labelProperty)

  const maxBytes = source.maxBytes ?? 0
  if (!Number.isInteger(maxBytes) || maxBytes <= 0 || maxBytes > MAX_MAP_GEOJSON_BYTES) {
    throw new Error(`Map source maxBytes must be between 1 and ${MAX_MAP_GEOJSON_BYTES}`)
  }
  const path = source.url!
  if (!path.startsWith('/') || path.startsWith('//') || path.includes('\\')) throw new Error('Map source URL must be same-origin')
  const base = typeof window === 'undefined' ? 'http://localhost' : window.location.href
  const requested = new URL(path, base)
  if (requested.origin !== new URL(base).origin || requested.hash) throw new Error('Map source URL must be same-origin')
  const response = await fetcher(requested.pathname + requested.search, {
    credentials: 'same-origin', headers: { Accept: 'application/geo+json, application/json' }, signal,
  })
  if (!response.ok) throw new Error(`Map geometry request failed (${response.status})`)
  if (response.url && new URL(response.url, base).origin !== requested.origin) throw new Error('Map source redirected cross-origin')
  const contentType = response.headers.get('Content-Type')?.toLowerCase() ?? ''
  if (contentType && !contentType.includes('json')) throw new Error('Map geometry response must be JSON')
  const payload = await readBounded(response, maxBytes)
  let parsed: unknown
  try {
    parsed = JSON.parse(payload)
  } catch {
    throw new Error('Map geometry response is not valid JSON')
  }
  return validateGeometry(parsed, featureProperty, labelProperty)
}

export function mapJoinError(panel: Panel, frame: Frame, geometry: GeoJSONFeatureCollection): Error | undefined {
  const config = panel.map
  if (!config) return new Error('Map panel has no map config')
  const idIndex = frame.columns.findIndex(({ name }) => name === panel.encoding.id)
  const valueIndex = frame.columns.findIndex(({ name }) => name === panel.encoding.value)
  if (idIndex < 0 || valueIndex < 0) return new Error('Map frame is missing its region ID or value column')
  const featureKeys = new Set(geometry.features.map(({ properties }) => properties[config.featureProperty]).filter((key): key is string => typeof key === 'string'))
  const rowKeys = new Set<string>()
  for (const [index, row] of frame.rows.entries()) {
    const key = row[idIndex]
    if (typeof key !== 'string' || key.trim() === '') return new Error(`Map row ${index} has no string region key`)
    if (rowKeys.has(key)) return new Error(`Map frame has duplicate region key ${key}`)
    rowKeys.add(key)
    if (!featureKeys.has(key)) return new Error(`Map region ${key} has no GeoJSON feature`)
    const value = row[valueIndex]
    if (typeof value !== 'number' || !Number.isFinite(value)) return new Error(`Map region ${key} has no finite numeric value`)
  }
  return undefined
}

export interface MapPanelProps {
  panel: Panel
  adapter?: ChartAdapter
  fetcher?: typeof fetch
  frame?: PanelFrameState
}

type GeometryState = { loading: true; data?: undefined; error?: undefined }
  | { loading: false; data: GeoJSONFeatureCollection; error?: undefined }
  | { loading: false; data?: undefined; error: Error }

export function MapPanel({ panel, adapter, fetcher, frame: frameOverride }: MapPanelProps) {
  const runtimeFrame = usePanelFrame(panel.id)
  const frame = frameOverride ?? runtimeFrame
  const { document } = useDashboard()
  const { drillInto } = useDrill()
  const translate = useTranslate()
  const formatValue = useFormat(panel.encoding.value ? panel.format[panel.encoding.value] : undefined)
  const navigation = usePanelNavigation(panel)
  const [attempt, setAttempt] = useState(0)
  const [geometry, setGeometry] = useState<GeometryState>({ loading: true })
  const config = panel.map
  const labelProperty = config ? resolveMapLabelProperty(config, document.meta?.locale ?? '') : undefined

  useEffect(() => {
    const controller = new AbortController()
    setGeometry({ loading: true })
    if (!config) {
      setGeometry({ loading: false, error: new Error('Map panel has no map config') })
      return () => controller.abort()
    }
    void loadMapGeometry(config.source, config.featureProperty, labelProperty, fetcher, controller.signal)
      .then((data) => setGeometry({ loading: false, data }))
      .catch((cause: unknown) => {
        if (!controller.signal.aborted) {
          console.error(`[lens] map ${panel.id} geometry failed`, cause)
          setGeometry({ loading: false, error: cause instanceof Error ? cause : new Error('Map geometry failed') })
        }
      })
    return () => controller.abort()
  }, [attempt, config, fetcher, labelProperty, panel.id])

  const joinError = useMemo(() => geometry.data && frame.data ? mapJoinError(panel, frame.data, geometry.data) : undefined, [frame.data, geometry.data, panel])
  useEffect(() => {
    if (joinError) console.error(`[lens] map ${panel.id} data join failed`, joinError)
  }, [joinError, panel.id])
  const sourceFrame = useMemo(() => {
    if (frame.isLoading || (frame.error && !frame.data) || !frame.data || frame.data.rows.length === 0) return frame
    if (geometry.loading) return { ...frame, isLoading: true }
    const error = geometry.error ?? joinError
    if (error) return {
      isLoading: false,
      isStale: false,
      error: new Error(translate('chart.error', 'Unable to render chart.')),
      retry: () => setAttempt((value) => value + 1),
    }
    return frame
  }, [frame, geometry, joinError, translate])

  const input = useMemo<ChartInput | undefined>(() => {
    if (!frame.data || frame.data.rows.length === 0 || !geometry.data || joinError || !config) return undefined
    const format: ChartFormatResolver = (_field, value) => formatValue(value)
    return {
      kind: 'map', frame: frame.data, encoding: panel.encoding, format, formatAxis: format,
      theme: document.theme,
      map: { name: `lens-map:${panel.id}`, geoJSON: geometry.data, featureProperty: config.featureProperty, labelProperty },
    }
  }, [config, document.theme, formatValue, frame.data, geometry.data, joinError, labelProperty, panel.encoding, panel.id])

  const interactive = Boolean(panel.drillRoot || navigation.action)
  const select = useCallback((key: NodeKey, _anchor?: unknown, activation?: ChartActivation) => {
    if (panel.drillRoot) {
      drillInto(key, panel.id)
      return
    }
    if (!frame.data) return
    const idIndex = frame.data.columns.findIndex(({ name }) => name === panel.encoding.id)
    const row = frame.data.rows.find((candidate) => candidate[idIndex] === key)
    navigation.activate(navigation.urlForRow(frame.data, row), undefined, { newTab: activation?.newTab })
  }, [drillInto, frame.data, navigation, panel.drillRoot, panel.encoding.id, panel.id])

  return (
    <PanelFrame panel={panel} frame={sourceFrame}>
      {input && (
        <>
          <ChartDataEquivalent
            actionable={interactive}
            format={input.format}
            frame={input.frame}
            label={translate('chart.data', 'Chart data for {name}', { name: panel.title })}
            onSelect={select}
            panel={panel}
            translate={translate}
          />
          <ChartHost
            adapter={adapter}
            drillable={interactive}
            input={input}
            label={translate('chart.label', '{name} chart', { name: panel.title })}
            onSelect={interactive ? select : undefined}
            panelId={panel.id}
          />
          {config?.attribution && (
            <p aria-label={translate('map.attribution', 'Map attribution')} className="lens-map-attribution" role="note">
              {config.attribution}
            </p>
          )}
        </>
      )}
    </PanelFrame>
  )
}

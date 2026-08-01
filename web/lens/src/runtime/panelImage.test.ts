import { afterEach, describe, expect, it, vi } from 'vitest'
import { panelSVG } from './panelImage'

afterEach(() => {
  document.body.replaceChildren()
  document.head.querySelectorAll('[data-panel-image-test]').forEach((element) => element.remove())
  vi.restoreAllMocks()
})

describe('panelSVG', () => {
  it('serializes the complete panel and preserves chart canvases as images', async () => {
    const style = document.createElement('style')
    style.dataset.panelImageTest = 'true'
    style.textContent = `
      .panel-image-test { --label: "A&B"; --syntax: "<length-percentage>"; }
      .panel-image-external { background-image: url("/external.png"); }
    `
    document.head.append(style)
    const panel = document.createElement('section')
    panel.className = 'lens-panel'
    panel.dataset.panelId = 'claims'
    panel.innerHTML = '<h3>Claims</h3><canvas width="640" height="320"></canvas>'
    vi.spyOn(panel, 'getBoundingClientRect').mockReturnValue({
      width: 800, height: 420, x: 0, y: 0, top: 0, left: 0, right: 800, bottom: 420, toJSON: () => ({}),
    })
    vi.spyOn(panel.querySelector('canvas')!, 'toDataURL').mockReturnValue('data:image/png;base64,chart')
    document.body.append(panel)

    const exported = panelSVG('claims')
    expect(exported.width).toBe(800)
    expect(exported.height).toBe(420)
    const markup = await new Promise<string>((resolve, reject) => {
      const reader = new FileReader()
      reader.onerror = () => reject(reader.error ?? new Error('blob read failed'))
      reader.onload = () => resolve(typeof reader.result === 'string' ? reader.result : '')
      reader.readAsText(exported.blob)
    })
    expect(markup).toContain('<foreignObject')
    expect(markup).toContain('data:image/png;base64,chart')
    expect(markup).toContain('data-panel-id="claims"')
    expect(markup).toContain('A&amp;B')
    expect(markup).toContain('&lt;length-percentage&gt;')
    expect(markup).not.toContain('/external.png')
    const parsed = new DOMParser().parseFromString(markup, 'image/svg+xml')
    expect(parsed.querySelector('parsererror')).toBeNull()
  })

  it('exports resolved chart content without interactive chrome or hidden legend rows', async () => {
    const panel = document.createElement('section')
    panel.className = 'lens-panel'
    panel.dataset.panelId = 'revenue'
    panel.innerHTML = `
      <header><h3>Revenue</h3><span class="lens-info-tip">Info</span><div class="lens-panel-actions"><button>Export</button></div></header>
      <p class="lens-panel-export-scope">2026-01-01 — 2026-07-22</p>
      <div class="lens-chart-reset-zoom">Reset zoom</div>
      <div class="lens-chart-keyboard-actions"><button>Open mark</button></div>
      <div class="lens-chart-legend-shell"><div class="lens-chart-legend-scroll-frame">
        <div class="lens-chart-legend-tools"><button>Hide all</button></div>
        <label class="lens-chart-legend-search"><input placeholder="Search legend" /></label>
        <ul class="lens-chart-legend">
          <li class="lens-chart-legend-item"><button><span>Visible</span></button><button class="lens-chart-legend-solo">Solo</button></li>
          <li class="lens-chart-legend-item"><button class="lens-chart-legend-hidden"><span>Hidden</span></button></li>
        </ul>
      </div></div>
      <div class="lens-chart-canvas">Resolved chart</div>
    `
    vi.spyOn(panel, 'getBoundingClientRect').mockReturnValue({
      width: 800, height: 420, x: 0, y: 0, top: 0, left: 0, right: 800, bottom: 420, toJSON: () => ({}),
    })
    document.body.append(panel)

    const markup = await new Promise<string>((resolve, reject) => {
      const reader = new FileReader()
      reader.onerror = () => reject(reader.error ?? new Error('blob read failed'))
      reader.onload = () => resolve(typeof reader.result === 'string' ? reader.result : '')
      reader.readAsText(panelSVG('revenue').blob)
    })
    expect(markup).toContain('Revenue')
    expect(markup).toContain('Resolved chart')
    expect(markup).toContain('2026-01-01 — 2026-07-22')
    expect(markup).toContain('Visible')
    expect(markup).not.toContain('Export')
    expect(markup).not.toContain('Info')
    expect(markup).not.toContain('Reset zoom')
    expect(markup).not.toContain('Open mark')
    expect(markup).not.toContain('Hide all')
    expect(markup).not.toContain('Search legend')
    expect(markup).not.toContain('Solo')
    expect(markup).not.toContain('Hidden')
  })
})

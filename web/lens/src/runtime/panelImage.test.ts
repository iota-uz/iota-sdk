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
})

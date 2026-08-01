export type PanelImageFormat = 'png' | 'svg'

const fallbackPanelWidth = 960
const fallbackPanelHeight = 540
const maximumExportPixelRatio = 2

function panelNode(panelId: string): HTMLElement {
  const panel = Array.from(document.querySelectorAll<HTMLElement>('.lens-panel'))
    .find((candidate) => candidate.dataset.panelId === panelId)
  if (!panel) throw new Error(`panel ${panelId} is not rendered`)
  return panel
}

function stylesheetText(): string {
  const rules: string[] = []
  for (const sheet of Array.from(document.styleSheets)) {
    try {
      for (const rule of Array.from(sheet.cssRules)) {
        // The SVG is loaded from a blob URL. Any stylesheet URL (most notably
        // @font-face) is therefore an external subresource from the SVG's
        // opaque origin; drawing it taints the target canvas and makes PNG
        // encoding illegal. Export with the declared fallback fonts and omit
        // URL-bearing decoration rather than producing an unusable PNG.
        if (/url\s*\(/i.test(rule.cssText)) continue
        rules.push(rule.cssText)
      }
    } catch {
      // A host may load cross-origin CSS. Lens' own same-origin stylesheet is
      // still captured; inaccessible host rules must not abort the export.
    }
  }
  return rules.join('\n')
}

function safeFilename(value: string): string {
  const normalized = value.trim().toLocaleLowerCase().replace(/[^\p{L}\p{N}]+/gu, '-').replace(/^-|-$/g, '')
  return normalized || 'lens-panel'
}

/** CSS is text inside an XML document, so Tailwind selectors containing `&`
 * and modern `@property` syntax containing `<length-percentage>` must be
 * escaped before the SVG can be decoded as an image. */
function xmlText(value: string): string {
  return value.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;')
}

export function panelSVG(panelId: string): { background: string; blob: Blob; width: number; height: number } {
  const source = panelNode(panelId)
  const bounds = source.getBoundingClientRect()
  const width = Math.max(1, Math.ceil(bounds.width || source.offsetWidth || fallbackPanelWidth))
  const height = Math.max(1, Math.ceil(bounds.height || source.offsetHeight || fallbackPanelHeight))
  const clone = source.cloneNode(true) as HTMLElement
  clone.querySelectorAll('.lens-panel-actions').forEach((node) => node.remove())
  clone.style.width = `${width}px`
  clone.style.height = `${height}px`
  const sourceCanvases = Array.from(source.querySelectorAll('canvas'))
  const clonedCanvases = Array.from(clone.querySelectorAll('canvas'))
  sourceCanvases.forEach((canvas, index) => {
    const target = clonedCanvases[index]
    if (!target) return
    const image = document.createElement('img')
    image.src = canvas.toDataURL('image/png')
    image.alt = ''
    image.width = canvas.width
    image.height = canvas.height
    image.style.cssText = target.style.cssText
    target.replaceWith(image)
  })
  const markup = new XMLSerializer().serializeToString(clone)
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}"><foreignObject width="100%" height="100%"><div xmlns="http://www.w3.org/1999/xhtml"><style>${xmlText(stylesheetText())}</style>${markup}</div></foreignObject></svg>`
  return {
    background: getComputedStyle(source).backgroundColor || '#ffffff',
    blob: new Blob([svg], { type: 'image/svg+xml;charset=utf-8' }),
    width,
    height,
  }
}

async function panelPNG(svg: Blob, width: number, height: number, background: string): Promise<Blob> {
  const image = new Image()
  // Chromium treats an SVG containing foreignObject as cross-origin when it
  // is loaded from a blob URL. It can display the image, but drawing it taints
  // the canvas and `toBlob` then throws. A self-contained data URL retains a
  // clean origin and is safe to encode.
  image.src = `data:image/svg+xml;charset=utf-8,${encodeURIComponent(await svg.text())}`
  await image.decode()
  const ratio = Math.max(1, Math.min(maximumExportPixelRatio, globalThis.devicePixelRatio || 1))
  const canvas = document.createElement('canvas')
  canvas.width = Math.ceil(width * ratio)
  canvas.height = Math.ceil(height * ratio)
  const context = canvas.getContext('2d')
  if (!context) throw new Error('canvas is unavailable')
  context.scale(ratio, ratio)
  context.fillStyle = background
  context.fillRect(0, 0, width, height)
  context.drawImage(image, 0, 0, width, height)
  return await new Promise<Blob>((resolve, reject) => canvas.toBlob(
    (blob) => blob ? resolve(blob) : reject(new Error('PNG encoding failed')),
    'image/png',
  ))
}

export async function downloadPanelImage(panelId: string, title: string, format: PanelImageFormat): Promise<void> {
  const exported = panelSVG(panelId)
  const blob = format === 'svg' ? exported.blob : await panelPNG(exported.blob, exported.width, exported.height, exported.background)
  const url = URL.createObjectURL(blob)
  try {
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = `${safeFilename(title)}.${format}`
    anchor.click()
  } finally {
    URL.revokeObjectURL(url)
  }
}

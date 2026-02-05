function toBase64(input: string): string {
  // Works in both browser and Node environments.
  if (typeof btoa === 'function') return btoa(input)
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const B: any = (globalThis as any).Buffer
  return B.from(input, 'utf8').toString('base64')
}

export function svgDataUrl({
  width,
  height,
  text,
  bg = '#111827',
  fg = '#f9fafb',
}: {
  width: number
  height: number
  text: string
  bg?: string
  fg?: string
}) {
  const svg = `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}">
  <defs>
    <linearGradient id="g" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0" stop-color="${bg}" stop-opacity="1"/>
      <stop offset="1" stop-color="${bg}" stop-opacity="0.75"/>
    </linearGradient>
  </defs>
  <rect width="${width}" height="${height}" fill="url(#g)"/>
  <rect x="18" y="18" width="${Math.max(0, width - 36)}" height="${Math.max(0, height - 36)}" fill="none" stroke="${fg}" stroke-opacity="0.35"/>
  <text x="50%" y="50%" dominant-baseline="middle" text-anchor="middle"
        font-family="ui-sans-serif, system-ui" font-size="${Math.max(14, Math.floor(Math.min(width, height) / 14))}"
        fill="${fg}" fill-opacity="0.92">
    ${text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')}
  </text>
</svg>`

  return `data:image/svg+xml;base64,${toBase64(svg)}`
}

export const smallImageDataUrl = svgDataUrl({ width: 240, height: 160, text: 'Small image' })
export const largeImageDataUrl = svgDataUrl({ width: 1600, height: 900, text: 'Large image' })

export function base64FromDataUrl(dataUrl: string): string {
  const idx = dataUrl.indexOf('base64,')
  return idx >= 0 ? dataUrl.slice(idx + 'base64,'.length) : ''
}


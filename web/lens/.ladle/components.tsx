import type { GlobalProvider } from '@ladle/react'
import { useEffect, useState } from 'react'
import './fonts.css'

/* In VR mode stories mount only after fonts are loaded: ECharts measures
   label text once at mount, so a font that arrives later leaves the chart
   laid out with fallback metrics — a subpixel diff lottery at
   maxDiffPixels 0. */
export const Provider: GlobalProvider = ({ children }) => {
  const vr =
    typeof document !== 'undefined' && document.documentElement.dataset['lensVr'] === 'true'
  const [ready, setReady] = useState(!vr)

  useEffect(() => {
    if (ready) return
    let cancelled = false
    void document.fonts.ready.then(() => {
      if (!cancelled) setReady(true)
    })
    return () => {
      cancelled = true
    }
  }, [ready])

  return ready ? children : null
}

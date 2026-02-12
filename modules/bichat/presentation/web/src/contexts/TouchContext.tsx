import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'

type TouchContextValue = {
  isTouchDevice: boolean
}

const TouchContext = createContext<TouchContextValue | null>(null)

function detectTouchDevice(): boolean {
  if (typeof window === 'undefined') return false

  const nav = window.navigator as Navigator & { maxTouchPoints?: number; msMaxTouchPoints?: number }
  const maxTouchPoints = Number(nav.maxTouchPoints ?? (nav as any).msMaxTouchPoints ?? 0)

  return (
    maxTouchPoints > 0 ||
    'ontouchstart' in window ||
    (typeof window.matchMedia === 'function' && window.matchMedia('(pointer: coarse)').matches)
  )
}

export function TouchProvider({ children }: { children: ReactNode }) {
  const [isTouchDevice, setIsTouchDevice] = useState(detectTouchDevice)

  useEffect(() => {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return

    const coarse = window.matchMedia('(pointer: coarse)')
    const onChange = () => setIsTouchDevice(detectTouchDevice())

    onChange()

    if (typeof coarse.addEventListener === 'function') {
      coarse.addEventListener('change', onChange)
      return () => coarse.removeEventListener('change', onChange)
    }

    // Safari < 14
    // eslint-disable-next-line deprecation/deprecation
    coarse.addListener(onChange)
    // eslint-disable-next-line deprecation/deprecation
    return () => coarse.removeListener(onChange)
  }, [])

  const value = useMemo(() => ({ isTouchDevice }), [isTouchDevice])

  return <TouchContext.Provider value={value}>{children}</TouchContext.Provider>
}

export function useTouchDevice(): boolean {
  const ctx = useContext(TouchContext)
  return ctx?.isTouchDevice ?? detectTouchDevice()
}


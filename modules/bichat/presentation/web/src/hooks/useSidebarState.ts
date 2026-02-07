import { useCallback, useEffect, useState } from 'react'

const MOBILE_MEDIA_QUERY = '(max-width: 767px)'

export function useSidebarState() {
  const [isMobile, setIsMobile] = useState(() => {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return false
    return window.matchMedia(MOBILE_MEDIA_QUERY).matches
  })
  const [isMobileOpen, setIsMobileOpen] = useState(false)

  useEffect(() => {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return

    const media = window.matchMedia(MOBILE_MEDIA_QUERY)
    const onChange = () => setIsMobile(media.matches)

    onChange()

    if (typeof media.addEventListener === 'function') {
      media.addEventListener('change', onChange)
      return () => media.removeEventListener('change', onChange)
    }

    // Safari < 14
    // eslint-disable-next-line deprecation/deprecation
    media.addListener(onChange)
    // eslint-disable-next-line deprecation/deprecation
    return () => media.removeListener(onChange)
  }, [])

  useEffect(() => {
    if (!isMobile) {
      setIsMobileOpen(false)
    }
  }, [isMobile])

  const openMobile = useCallback(() => setIsMobileOpen(true), [])
  const closeMobile = useCallback(() => setIsMobileOpen(false), [])
  const toggleMobile = useCallback(() => setIsMobileOpen((v) => !v), [])

  return { isMobile, isMobileOpen, openMobile, closeMobile, toggleMobile }
}


export function shouldEnableAppletDevtools(): boolean {
  if (typeof window === 'undefined') return false

  const url = new URL(window.location.href)
  if (url.searchParams.get('appletDebug') === '1') return true

  try {
    return window.localStorage.getItem('iotaAppletDevtools') === '1'
  } catch {
    return false
  }
}


import { useCallback } from 'react'
import { useTouchDevice } from '../contexts/TouchContext'

function safeVibrate(pattern: number | number[]) {
  if (typeof navigator === 'undefined') return
  if (typeof navigator.vibrate !== 'function') return
  try {
    navigator.vibrate(pattern)
  } catch {
    // noop
  }
}

export function useHapticFeedback() {
  const isTouchDevice = useTouchDevice()

  const light = useCallback(() => {
    if (!isTouchDevice) return
    safeVibrate(10)
  }, [isTouchDevice])

  const success = useCallback(() => {
    if (!isTouchDevice) return
    safeVibrate([10, 30, 10])
  }, [isTouchDevice])

  const error = useCallback(() => {
    if (!isTouchDevice) return
    safeVibrate([20, 40, 20])
  }, [isTouchDevice])

  return { light, success, error }
}


import { useAppletContext } from '../context/AppletContext'
import type { AppConfig } from '../types'

/**
 * useConfig provides access to applet configuration (endpoints, etc.)
 *
 * Usage:
 * const { graphQLEndpoint, streamEndpoint } = useConfig()
 */
export function useConfig(): AppConfig {
  const { config } = useAppletContext()
  return config
}

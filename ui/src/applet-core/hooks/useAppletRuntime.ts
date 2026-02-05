import { useConfig } from './useConfig'

export type ShellMode = 'embedded' | 'standalone'

export interface AppletRuntime {
  basePath: string
  assetsBasePath: string
  rpcEndpoint?: string
  shellMode?: ShellMode
}

export function useAppletRuntime(): AppletRuntime {
  const config = useConfig()

  const basePath = config.basePath ?? ''
  const assetsBasePath = config.assetsBasePath ?? `${basePath || ''}/assets`
  const rpcEndpoint = config.rpcUIEndpoint
  const shellMode = config.shellMode

  return { basePath, assetsBasePath, rpcEndpoint, shellMode }
}


export {
  createAppletViteConfig,
  createAppletBackendProxy,
  createLocalSdkAliases,
  getAppletAssetsBase,
  getAppletVitePort,
  readAppletDevManifest,
} from './vite'
export type { AppletViteOptions, AppletDevManifest } from './vite'

export {
  createAppletStylesVirtualModulePlugin,
  createBichatStylesPlugin,
  VIRTUAL_APPLET_STYLES_ID,
} from './styles-plugin'
export type { AppletStylesVirtualModuleOptions } from './styles-plugin'

/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly LENS_FIXTURE_MODE?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

declare module 'echarts/lib/chart/boxplot/install.js' {
  export const install: (registers: unknown) => void
}
declare module 'echarts/lib/chart/heatmap/install.js' {
  export const install: (registers: unknown) => void
}
declare module 'echarts/lib/chart/map/install.js' {
  export const install: (registers: unknown) => void
}
declare module 'echarts/lib/component/visualMap/install.js' {
  export const install: (registers: unknown) => void
}

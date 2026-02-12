/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly DEV: boolean
  readonly PROD: boolean
  readonly MODE: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

declare module '*.css?raw' {
  const content: string
  export default content
}

declare module 'virtual:applet-styles' {
  const css: string
  export default css
}

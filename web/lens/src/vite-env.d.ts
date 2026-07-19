/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly LENS_FIXTURE_MODE?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

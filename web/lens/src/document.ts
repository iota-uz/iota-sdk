export interface LensDocument {
  version: string
  snapshotId: string
  meta: {
    dashboardId: string
    title: string
    generatedAt: string
    locale: string
  }
  panels: LensPanel[]
  frames: Record<string, LensFrame>
}

export interface LensPanel {
  id: string
  kind: string
  title: string
  frame: string
  encoding: {
    label?: string
    value?: string
  }
}

export interface LensFrame {
  columns: Array<{ name: string; type: string }>
  rows: unknown[][]
}

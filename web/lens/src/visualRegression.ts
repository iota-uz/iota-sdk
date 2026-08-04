export function isVisualRegression(): boolean {
  return globalThis.document?.documentElement.dataset.lensVr === 'true'
}

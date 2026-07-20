/**
 * Full-page navigation seam. Panel-level actions are plain document
 * navigations; routing them through one function keeps the host page able to
 * intercept them and makes the behaviour testable.
 */
export function navigateTo(url: string): void {
  globalThis.location.assign(url)
}

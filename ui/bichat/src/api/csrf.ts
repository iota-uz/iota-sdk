/**
 * CSRF token management for API requests
 */

/**
 * Get CSRF token from window object
 * @returns CSRF token string
 */
export function getCSRFToken(): string {
  const token = window.__CSRF_TOKEN__
  if (!token) {
    console.warn('CSRF token not found in window object')
    return ''
  }
  return token
}

/**
 * Add CSRF token to request headers
 * @param headers - Headers object to modify
 * @returns Modified headers object
 */
export function addCSRFHeader(headers: Headers): Headers {
  const token = getCSRFToken()
  if (token) {
    headers.set('X-CSRF-Token', token)
  }
  return headers
}

/**
 * Create headers with CSRF token
 * @param init - Optional initial headers
 * @returns Headers object with CSRF token
 */
export function createHeadersWithCSRF(init?: HeadersInit): Headers {
  const headers = new Headers(init)
  return addCSRFHeader(headers)
}

export class RetryableRegistryError extends Error {
  constructor(message, cause) {
    super(message, { cause })
    this.name = 'RetryableRegistryError'
  }
}

export function releaseRegistry(fetchRegistry = fetch, now = Date.now) {
  return async version => {
    const url = new URL(`https://registry.npmjs.org/@iota-uz%2fsdk/${version}`)
    url.searchParams.set('verification', String(now()))
    let response
    try {
      response = await fetchRegistry(url, {
        cache: 'no-store', headers: { 'Cache-Control': 'no-cache' }, signal: AbortSignal.timeout(10_000),
      })
    } catch (error) {
      throw new RetryableRegistryError('npm registry request failed', error)
    }
    if (response.status === 404) return null
    if (response.status >= 500) throw new RetryableRegistryError(`npm registry returned ${response.status}`)
    if (!response.ok) throw new Error(`npm registry returned ${response.status}`)
    try { return await response.json() }
    catch (error) {
      if (error instanceof SyntaxError) throw error
      throw new RetryableRegistryError('npm registry response read failed', error)
    }
  }
}

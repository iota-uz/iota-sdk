export function releaseRegistry(fetchRegistry = fetch, now = Date.now) {
  return async version => {
    const url = new URL(`https://registry.npmjs.org/@iota-uz%2fsdk/${version}`)
    url.searchParams.set('verification', String(now()))
    const response = await fetchRegistry(url, {
      cache: 'no-store', headers: { 'Cache-Control': 'no-cache' }, signal: AbortSignal.timeout(10_000),
    })
    if (response.status === 404) return null
    if (!response.ok) throw new Error(`npm registry returned ${response.status}`)
    return response.json()
  }
}

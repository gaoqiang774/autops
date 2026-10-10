function appBasePath() {
  const base = (process.env.BASE_URL || '/').trim()
  if (!base || base === '/') return ''
  return `/${base.replace(/^\/+|\/+$/g, '')}`
}

export function withAppBasePath(path = '/') {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  const base = appBasePath()
  if (!base || normalizedPath === base || normalizedPath.startsWith(`${base}/`)) {
    return normalizedPath
  }
  return `${base}${normalizedPath}`
}

export function buildWebSocketUrl(path) {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const url = new URL(withAppBasePath(path), window.location.origin)
  return `${protocol}//${window.location.host}${url.pathname}${url.search}`
}

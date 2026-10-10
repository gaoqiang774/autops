import storage from '@/utils/storage'
import { buildWebSocketUrl } from '@/utils/url'

const RDP_SESSION_CACHE_KEY = 'rdpSessionCache'
const RDP_SESSION_MAX_AGE_MS = 12 * 60 * 60 * 1000

function loadCache() {
  const cache = storage.getItem(RDP_SESSION_CACHE_KEY)
  return cache && typeof cache === 'object' ? { ...cache } : {}
}

function saveCache(cache) {
  storage.setItem(RDP_SESSION_CACHE_KEY, cache)
}

function normalizeSession(session) {
  if (!session || typeof session !== 'object') {
    return null
  }

  const normalized = session.session && typeof session.session === 'object'
    ? session.session
    : session

  if (!normalized.sessionId) {
    return null
  }

  return {
    ...normalized,
    rdpid: normalized.rdpid ? String(normalized.rdpid) : '',
    sessionId: String(normalized.sessionId),
    updatedAt: Date.now()
  }
}

export function getCachedRDPSession(rdpid) {
  if (!rdpid) {
    return null
  }

  const cache = loadCache()
  const item = cache[String(rdpid)]
  if (!item || !item.sessionId) {
    return null
  }

  if (Date.now() - Number(item.updatedAt || 0) > RDP_SESSION_MAX_AGE_MS) {
    delete cache[String(rdpid)]
    saveCache(cache)
    return null
  }

  return item
}

export function setCachedRDPSession(rdpid, session) {
  const normalized = normalizeSession(session)
  if (!rdpid || !normalized) {
    return null
  }

  const cache = loadCache()
  cache[String(rdpid)] = {
    ...normalized,
    rdpid: normalized.rdpid || String(rdpid)
  }
  saveCache(cache)
  return cache[String(rdpid)]
}

export function clearCachedRDPSession(rdpid) {
  if (!rdpid) {
    return
  }

  const cache = loadCache()
  delete cache[String(rdpid)]
  saveCache(cache)
}

export function extractRDPSession(responseData) {
  if (!responseData || typeof responseData !== 'object') {
    return null
  }

  const payload = responseData.data && typeof responseData.data === 'object'
    ? responseData.data
    : responseData

  return normalizeSession(payload)
}

export function buildRdpWsUrl({
  sessionId,
  token,
  w,
  h,
  monitor,
  scale,
  devscale,
  avc
}) {
  if (!sessionId || !token) {
    return ''
  }

  const params = new URLSearchParams({
    sessionId: String(sessionId),
    token: String(token)
  })

  if (w) params.set('w', String(w))
  if (h) params.set('h', String(h))
  if (monitor !== undefined && monitor !== null && monitor !== '') {
    params.set('monitor', String(monitor))
  }
  if (scale) params.set('scale', String(scale))
  if (devscale) params.set('devscale', String(devscale))
  if (avc) params.set('avc', '1')

  return buildWebSocketUrl(`/api/v1/rdp/ws?${params.toString()}`)
}

const CLUSTER_STORAGE_KEY = 'k8s_selected_cluster_id'
export const CLUSTER_SELECTION_CHANGE_EVENT = 'k8s-selected-cluster-change'

const normalizeClusterId = (clusterId) => (
  clusterId === null || typeof clusterId === 'undefined' ? '' : String(clusterId)
)

export const getStoredClusterId = () => {
  if (typeof window === 'undefined') return ''

  try {
    return localStorage.getItem(CLUSTER_STORAGE_KEY) || ''
  } catch (error) {
    console.error('读取存储的集群失败:', error)
    return ''
  }
}

export const saveStoredClusterId = (clusterId, source = '') => {
  if (typeof window === 'undefined') return

  try {
    const normalizedClusterId = normalizeClusterId(clusterId)

    if (normalizedClusterId) {
      localStorage.setItem(CLUSTER_STORAGE_KEY, normalizedClusterId)
    } else {
      localStorage.removeItem(CLUSTER_STORAGE_KEY)
    }

    window.dispatchEvent(new CustomEvent(CLUSTER_SELECTION_CHANGE_EVENT, {
      detail: { clusterId: normalizedClusterId, source }
    }))
  } catch (error) {
    console.error('保存集群选择失败:', error)
  }
}

export const subscribeStoredClusterId = (callback) => {
  if (typeof window === 'undefined' || typeof callback !== 'function') {
    return () => {}
  }

  const handleClusterChange = (event) => {
    callback(normalizeClusterId(event.detail?.clusterId), event.detail || {})
  }

  const handleStorageChange = (event) => {
    if (event.key === CLUSTER_STORAGE_KEY) {
      callback(normalizeClusterId(event.newValue), { source: 'storage' })
    }
  }

  window.addEventListener(CLUSTER_SELECTION_CHANGE_EVENT, handleClusterChange)
  window.addEventListener('storage', handleStorageChange)

  return () => {
    window.removeEventListener(CLUSTER_SELECTION_CHANGE_EVENT, handleClusterChange)
    window.removeEventListener('storage', handleStorageChange)
  }
}

export const resolvePreferredClusterId = (clusters, preferredClusterId = '') => {
  if (!Array.isArray(clusters) || clusters.length === 0) return ''

  const normalizedPreferredId = normalizeClusterId(preferredClusterId)
  const matchedOnlineCluster = normalizedPreferredId
    ? clusters.find(cluster => String(cluster.id) === normalizedPreferredId && cluster.status === 2)
    : null

  if (matchedOnlineCluster) {
    return matchedOnlineCluster.id
  }

  const onlineCluster = clusters.find(cluster => cluster.status === 2)
  if (onlineCluster) {
    return onlineCluster.id
  }

  const matchedCluster = normalizedPreferredId
    ? clusters.find(cluster => String(cluster.id) === normalizedPreferredId)
    : null

  return matchedCluster ? matchedCluster.id : clusters[0].id
}

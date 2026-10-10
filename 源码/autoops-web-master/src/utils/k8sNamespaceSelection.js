const NAMESPACE_STORAGE_KEY = 'k8s_selected_namespaces'
export const DEFAULT_K8S_NAMESPACE = 'default'

const normalizeClusterId = (clusterId) => (
  clusterId === null || typeof clusterId === 'undefined' ? '' : String(clusterId)
)

const normalizeNamespace = (namespace) => (
  namespace === null || typeof namespace === 'undefined' ? '' : String(namespace)
)

const readStoredNamespaceMap = () => {
  if (typeof window === 'undefined') return {}

  try {
    const stored = window.localStorage.getItem(NAMESPACE_STORAGE_KEY)
    if (!stored) return {}

    const parsed = JSON.parse(stored)
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {}
  } catch (error) {
    console.error('读取存储的 namespace 失败:', error)
    return {}
  }
}

export const getStoredNamespace = (clusterId) => {
  const normalizedClusterId = normalizeClusterId(clusterId)
  if (!normalizedClusterId) return ''

  const namespaces = readStoredNamespaceMap()
  return normalizeNamespace(namespaces[normalizedClusterId])
}

export const saveStoredNamespace = (clusterId, namespace) => {
  if (typeof window === 'undefined') return

  const normalizedClusterId = normalizeClusterId(clusterId)
  const normalizedNamespace = normalizeNamespace(namespace)
  if (!normalizedClusterId) return

  try {
    const namespaces = readStoredNamespaceMap()

    if (normalizedNamespace) {
      namespaces[normalizedClusterId] = normalizedNamespace
    } else {
      delete namespaces[normalizedClusterId]
    }

    window.localStorage.setItem(NAMESPACE_STORAGE_KEY, JSON.stringify(namespaces))
  } catch (error) {
    console.error('保存 namespace 到存储失败:', error)
  }
}

const getNamespaceName = (namespace) => (
  typeof namespace === 'string' ? namespace : normalizeNamespace(namespace?.name)
)

export const resolvePreferredNamespace = (
  namespaces,
  preferredNamespace = '',
  {
    includeAllOption = false,
    allOptionValue = '__all__'
  } = {}
) => {
  const preferred = normalizeNamespace(preferredNamespace)
  const namespaceNames = Array.isArray(namespaces)
    ? namespaces.map(getNamespaceName).filter(Boolean)
    : []

  if (includeAllOption && preferred === allOptionValue) {
    return allOptionValue
  }

  if (preferred && namespaceNames.includes(preferred)) {
    return preferred
  }

  if (includeAllOption) {
    return allOptionValue
  }

  if (namespaceNames.includes(DEFAULT_K8S_NAMESPACE)) {
    return DEFAULT_K8S_NAMESPACE
  }

  return namespaceNames[0] || DEFAULT_K8S_NAMESPACE
}

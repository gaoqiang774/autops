import request from "@/utils/request"

// ==================== 资源管理 ====================

// 获取所有资源列表
export function getResources(params) {
  return request({
    url: '/resources',
    method: 'get',
    params
  })
}

// 根据类型获取资源
export function getResourcesByType(type, params) {
  return request({
    url: `/resources/type/${type}`,
    method: 'get',
    params
  })
}

// 获取单个资源详情
export function getResourceDetail(id) {
  if (!id || id === 'undefined' || id === 'null') {
    return Promise.reject(new Error('资源ID不能为空'))
  }
  
  return request({
    url: `/resources/${id}`,
    method: 'get'
  })
}

// 搜索资源
export function searchResources(params) {
  return request({
    url: '/resources/search',
    method: 'get',
    params
  })
}

// 创建资源
export function createResource(data) {
  return request({
    url: '/resources',
    method: 'post',
    data
  })
}

// 批量导入资源
export function batchImportResources(resourceType, resources) {
  return request({
    url: '/resources/import',
    method: 'post',
    data: { resourceType, resources }
  })
}

// 更新资源
export function updateResource(id, data) {
  if (!id) {
    return Promise.reject(new Error('资源ID不能为空'))
  }
  
  return request({
    url: `/resources/${id}`,
    method: 'put',
    data
  })
}

// 删除资源
export function deleteResource(id) {
  if (!id || id === 'undefined' || id === 'null') {
    return Promise.reject(new Error('资源ID不能为空'))
  }
  
  return request({
    url: `/resources/${id}`,
    method: 'delete'
  })
}

// ==================== 关系管理 ====================

// 获取资源的上游关系
export function getResourceUpstream(id) {
  if (!id || id === 'undefined' || id === 'null') {
    return Promise.reject(new Error('资源ID不能为空'))
  }
  
  return request({
    url: `/resources/${id}/upstream`,
    method: 'get'
  })
}

// 获取资源的下游关系
export function getResourceDownstream(id) {
  if (!id || id === 'undefined' || id === 'null') {
    return Promise.reject(new Error('资源ID不能为空'))
  }
  
  return request({
    url: `/resources/${id}/downstream`,
    method: 'get'
  })
}

// 获取资源关系图谱（层级查询）
export function getResourceGraph(id, params = {}) {
  if (!id) {
    return Promise.reject(new Error('资源ID不能为空'))
  }
  
  return request({
    url: `/resources/${id}/graph`,
    method: 'get',
    params: {
      depth: params.depth || 3,
      direction: params.direction || 'both'
    }
  })
}

// 获取全量拓扑（全部节点）
export function getFullTopology() {
  return request({
    url: '/resources/graph',
    method: 'get'
  })
}

// 创建关系
export function createRelation(data) {
  return request({
    url: '/relations',
    method: 'post',
    data
  })
}

// 删除关系
export function deleteRelation(id) {
  if (!id) {
    return Promise.reject(new Error('关系ID不能为空'))
  }
  
  return request({
    url: `/relations/${id}`,
    method: 'delete'
  })
}

// 更新关系
export function updateRelation(id, data) {
  if (!id) {
    return Promise.reject(new Error('关系ID不能为空'))
  }
  
  return request({
    url: `/relations/${id}`,
    method: 'put',
    data
  })
}

// 获取关系详情
export function getRelationDetail(id) {
  if (!id) {
    return Promise.reject(new Error('关系ID不能为空'))
  }
  
  return request({
    url: `/relations/${id}`,
    method: 'get'
  })
}

// 批量创建关系
export function batchCreateRelations(data) {
  return request({
    url: '/relations/batch',
    method: 'post',
    data
  })
}

// ==================== 类型和状态映射 ====================

// 获取资源类型列表
export function getResourceTypes() {
  return request({
    url: '/resources/types',
    method: 'get'
  })
}

// 获取根节点资源列表
export function getRootResources(params) {
  return request({
    url: '/resources/roots',
    method: 'get',
    params
  })
}

// 获取状态映射
export function getStatusMap() {
  return request({
    url: '/status-map',
    method: 'get'
  })
}

// 获取关系类型映射
export function getRelationTypes() {
  return request({
    url: '/relation-types',
    method: 'get'
  })
}

// ==================== 统计信息 ====================

// 获取资源统计信息
export function getResourceStats() {
  return request({
    url: '/resources/stats',
    method: 'get'
  })
}

// ==================== 模型定义 ====================

// 获取所有模型定义
export function getModelDefinitions() {
  return request({
    url: '/model-definitions',
    method: 'get'
  })
}

// 获取单个模型定义
export function getModelDefinition(typeName) {
  return request({
    url: `/model-definitions/${typeName}`,
    method: 'get'
  })
}

// 创建模型定义
export function createModelDefinition(data) {
  return request({
    url: '/model-definitions',
    method: 'post',
    data
  })
}

// 更新模型定义
export function updateModelDefinition(id, data) {
  return request({
    url: `/model-definitions/${id}`,
    method: 'put',
    data
  })
}

// 删除模型定义
export function deleteModelDefinition(id) {
  return request({
    url: `/model-definitions/${id}`,
    method: 'delete'
  })
}

// ==================== 批量操作 ====================

// 批量删除资源
export function batchDeleteResources(ids) {
  return request({
    url: '/resources/batch',
    method: 'delete',
    data: { ids }
  })
}

// 批量更新资源状态
export function batchUpdateStatus(ids, status) {
  return request({
    url: '/resources/batch/status',
    method: 'put',
    data: { ids, status }
  })
}

// ==================== 导入导出 ====================

// 导出资源数据
export function exportResources(params) {
  return request({
    url: '/resources/export',
    method: 'get',
    params,
    responseType: 'blob'
  })
}

// 导入资源数据
export function importResources(file) {
  const formData = new FormData()
  formData.append('file', file)
  
  return request({
    url: '/resources/import',
    method: 'post',
    data: formData,
    headers: { 'Content-Type': 'multipart/form-data' }
  })
}
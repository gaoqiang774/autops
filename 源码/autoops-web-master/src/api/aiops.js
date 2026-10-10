import request from '@/utils/request'
import storage from '@/utils/storage'
import { buildWebSocketUrl, withAppBasePath } from '@/utils/url'

const normalizePayload = data => (typeof data === 'object' && data !== null ? data : { id: data })

export default {
    listMCPServers(params) {
        return request({
            url: 'aiops/mcp/admin/servers',
            method: 'get',
            params
        })
    },
    updateMCPServerStatus(code, enabled) {
        return request({
            url: `aiops/mcp/admin/servers/${code}/status`,
            method: 'put',
            data: { enabled }
        })
    },
    updateMCPToolConfig(code, toolName, data) {
        return request({
            url: `aiops/mcp/admin/servers/${code}/tools/${encodeURIComponent(toolName)}/config`,
            method: 'put',
            data
        })
    },
    confirmMCPAction(actionType, confirmationId) {
        return request({
            url: `aiops/mcp/actions/${actionType}/confirm`,
            method: 'post',
            data: { confirmationId },
            timeout: 120000,
            skipErrorMessage: true
        })
    },
    confirmMCPCleanupLogs(confirmationId) {
        return this.confirmMCPAction('cleanup-logs', confirmationId)
    },
    confirmMCPK8sAction(confirmationId) {
        return this.confirmMCPAction('k8s', confirmationId)
    },
    cancelMCPAction(confirmationId) {
        return request({
            url: 'aiops/mcp/actions/cancel',
            method: 'post',
            data: { confirmationId },
            skipErrorMessage: true
        })
    },
    getMCPK8sRolloutStatus(endpoint, params) {
        return request({
            url: endpoint || 'aiops/mcp/actions/k8s/rollout-status',
            method: 'get',
            params,
            skipErrorMessage: true
        })
    },
    getLLMProviderTypes() {
        return request({
            url: 'aiops/llm/providers/types',
            method: 'get'
        })
    },
    getEnabledLLMProviders(params) {
        return request({
            url: 'aiops/llm/providers/enabled',
            method: 'get',
            params
        })
    },
    getLLMProviderList(params) {
        return request({
            url: 'aiops/llm/providers/list',
            method: 'get',
            params
        })
    },
    getLLMProviderById(id) {
        return request({
            url: 'aiops/llm/providers',
            method: 'get',
            params: { id }
        })
    },
    createLLMProvider(data) {
        return request({
            url: 'aiops/llm/providers',
            method: 'post',
            data
        })
    },
    updateLLMProvider(data) {
        return request({
            url: 'aiops/llm/providers',
            method: 'put',
            data
        })
    },
    deleteLLMProvider(id) {
        return request({
            url: 'aiops/llm/providers',
            method: 'delete',
            params: { id }
        })
    },
    setDefaultLLMProvider(data) {
        return request({
            url: 'aiops/llm/providers/default',
            method: 'post',
            data: normalizePayload(data)
        })
    },
    getLLMProviderSecret(id) {
        const payload = normalizePayload(id)
        return request({
            url: 'aiops/llm/providers/secret',
            method: 'post',
            params: { id: payload.id }
        })
    },
    testLLMProvider(data) {
        const payload = normalizePayload(data)
        const hasOnlyId = payload.id && Object.keys(payload).length === 1
        return request({
            url: 'aiops/llm/providers/test',
            method: 'post',
            params: hasOnlyId ? { id: payload.id } : undefined,
            data: hasOnlyId ? undefined : payload
        })
    },
    listKnowledgeDocuments(params) {
        return request({
            url: 'aiops/knowledge/documents',
            method: 'get',
            params
        })
    },
    listKnowledgeGroups() {
        return request({
            url: 'aiops/knowledge/groups/tree',
            method: 'get'
        })
    },
    createKnowledgeGroup(data) {
        return request({
            url: 'aiops/knowledge/groups',
            method: 'post',
            data
        })
    },
    updateKnowledgeGroup(id, data) {
        return request({
            url: `aiops/knowledge/groups/${id}`,
            method: 'put',
            data
        })
    },
    deleteKnowledgeGroup(id) {
        return request({
            url: `aiops/knowledge/groups/${id}`,
            method: 'delete'
        })
    },
    getKnowledgeDocument(id) {
        return request({
            url: `aiops/knowledge/documents/${id}`,
            method: 'get'
        })
    },
    createKnowledgeDocument(data) {
        return request({
            url: 'aiops/knowledge/documents',
            method: 'post',
            data
        })
    },
    updateKnowledgeDocument(id, data) {
        return request({
            url: `aiops/knowledge/documents/${id}`,
            method: 'put',
            data
        })
    },
    deleteKnowledgeDocument(id) {
        return request({
            url: `aiops/knowledge/documents/${id}`,
            method: 'delete'
        })
    },
    importKnowledgeDocument(data, config = {}) {
        return request({
            url: 'aiops/knowledge/documents/import',
            method: 'post',
            data,
            headers: {
                'Content-Type': 'multipart/form-data'
            },
            ...config
        })
    },
    summarizeKnowledgeDocument(id, data = {}) {
        return request({
            url: `aiops/knowledge/documents/${id}/summarize`,
            method: 'post',
            data,
            timeout: 120000
        })
    },
    refreshKnowledgeSearchData(id) {
        return request({
            url: `aiops/knowledge/documents/${id}/search-data/refresh`,
            method: 'post',
            timeout: 60000
        })
    },
    searchKnowledge(data) {
        return request({
            url: 'aiops/knowledge/search',
            method: 'post',
            data,
            timeout: 60000
        })
    },
    listHostLogFiles(params) {
        return request({
            url: 'aiops/log/host/files',
            method: 'get',
            params
        })
    },
    queryHostLogs(data) {
        return request({
            url: 'aiops/log/host/query',
            method: 'post',
            data
        })
    },
    queryK8sLogs(data) {
        return request({
            url: 'aiops/log/k8s/query',
            method: 'post',
            data
        })
    },
    listLogSessions(params) {
        return request({
            url: 'aiops/log/sessions',
            method: 'get',
            params
        })
    },
    getLogSessionStatus(sessionId) {
        return request({
            url: 'aiops/log/session/status',
            method: 'get',
            params: { sessionId }
        })
    },
    analyzeLogs(data) {
        return request({
            url: 'aiops/log/analyze',
            method: 'post',
            data,
            timeout: 120000
        })
    },
    analyzeK8sPod(data) {
        return request({
            url: 'aiops/log/k8s/pod/analyze',
            method: 'post',
            data,
            timeout: 180000
        })
    },
    createK8sPodAnalyzeTask(data) {
        return request({
            url: 'aiops/log/k8s/pod/analyze/tasks',
            method: 'post',
            data,
            timeout: 30000
        })
    },
    getK8sPodAnalyzeTask(taskId) {
        return request({
            url: `aiops/log/k8s/pod/analyze/tasks/${taskId}`,
            method: 'get',
            timeout: 30000
        })
    },
    analyzeAlert(data) {
        return request({
            url: 'aiops/agent/alerts/analyze',
            method: 'post',
            data,
            timeout: 120000
        })
    },
    getAgentRun(runId) {
        return request({
            url: `aiops/agent/runs/${runId}`,
            method: 'get'
        })
    },
    queryAssistant(data) {
        return request({
            url: 'aiops/assistant/query',
            method: 'post',
            data,
            timeout: 120000
        })
    },
    chatAssistant(data) {
        return request({
            url: 'aiops/assistant/chat',
            method: 'post',
            data,
            timeout: 120000
        })
    },
    clearAssistantChatHistory(data = {}) {
        return request({
            url: 'aiops/assistant/chat/history',
            method: 'delete',
            data,
            timeout: 30000
        })
    },
    submitAgentFeedback(data) {
        return request({
            url: 'aiops/agent/feedback',
            method: 'post',
            data,
            timeout: 30000
        })
    },
    fetchAssistantChatStream(data) {
        const token = storage.getToken()
        const headers = {
            'Content-Type': 'application/json',
            Accept: 'text/event-stream, application/x-ndjson, application/json, text/plain'
        }
        if (token) headers.Authorization = `Bearer ${token}`

        return fetch('/api/v1/aiops/assistant/chat', {
            method: 'POST',
            headers,
            credentials: 'include',
            cache: 'no-store',
            body: JSON.stringify(data)
        })
    },
    createAgentRunStream(runId, params = {}) {
        const query = new URLSearchParams()
        const token = params.token || storage.getToken()
        const payload = {
            ...params,
            token
        }

        Object.entries(payload).forEach(([key, value]) => {
            if (value === undefined || value === null || value === '') return
            query.set(key, Array.isArray(value) ? value.join(',') : String(value))
        })

        const suffix = query.toString() ? `?${query.toString()}` : ''
        return new EventSource(withAppBasePath(`/api/v1/aiops/agent/runs/${runId}/stream${suffix}`), { withCredentials: true })
    },
    createK8sPodAnalyzeTaskStream(taskId, params = {}) {
        const query = new URLSearchParams()
        const token = params.token || storage.getToken()
        const payload = {
            ...params,
            token
        }

        Object.entries(payload).forEach(([key, value]) => {
            if (value === undefined || value === null || value === '') return
            query.set(key, Array.isArray(value) ? value.join(',') : String(value))
        })

        const suffix = query.toString() ? `?${query.toString()}` : ''
        return new EventSource(withAppBasePath(`/api/v1/aiops/log/k8s/pod/analyze/tasks/${taskId}/stream${suffix}`), { withCredentials: true })
    },
    createHostLogStream(params = {}) {
        const query = new URLSearchParams()
        const token = params.token || storage.getToken()
        const payload = {
            ...params,
            token
        }

        Object.entries(payload).forEach(([key, value]) => {
            if (value === undefined || value === null || value === '') return
            query.set(key, Array.isArray(value) ? value.join(',') : String(value))
        })

        return new WebSocket(buildWebSocketUrl(`/api/v1/aiops/log/host/stream?${query.toString()}`))
    },
    createK8sLogStream(params = {}) {
        const query = new URLSearchParams()
        const token = params.token || storage.getToken()
        const payload = {
            ...params,
            token
        }

        Object.entries(payload).forEach(([key, value]) => {
            if (value === undefined || value === null || value === '') return
            query.set(key, Array.isArray(value) ? value.join(',') : String(value))
        })

        return new WebSocket(buildWebSocketUrl(`/api/v1/aiops/log/k8s/stream?${query.toString()}`))
    }
}

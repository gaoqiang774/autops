import storage from './storage'

export const VISITOR_USERNAME = 'test'
export const VISITOR_DENIED_MESSAGE = '游客无权限！'

export function isVisitor() {
    const admin = storage.getItem('sysAdmin') || {}
    return String(admin.username || '').trim().toLowerCase() === VISITOR_USERNAME
}

export function isReadOnlyMethod(method = 'get') {
    return ['get', 'head', 'options'].includes(String(method).toLowerCase())
}

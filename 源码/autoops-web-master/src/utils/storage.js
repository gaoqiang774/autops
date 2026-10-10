/**
 * storage封装
 *
 * @author xiaoRui
 */
export default {
    getStorage(){
        return JSON.parse(window.localStorage.getItem(process.env.VUE_APP_NAME_SPACE) || "{}")
    },
    setItem(key, val) {
        let storage = this.getStorage()
        storage[key] = val
        window.localStorage.setItem(process.env.VUE_APP_NAME_SPACE, JSON.stringify(storage))
    },
    getItem(key) {
        return this.getStorage()[key]
    },
    getToken() {
        const token = this.getItem('token')
        if (!token) return ''
        if (typeof token === 'object') {
            return token.access_token || token.token || ''
        }
        return String(token)
    },
    clearItem(key) {
        let storage = this.getStorage()
        delete storage[key]
        window.localStorage.setItem(process.env.VUE_APP_NAME_SPACE, JSON.stringify(storage))
    },
    clearAll() {
        window.localStorage.clear()
    }
}

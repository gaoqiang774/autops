import { createApp } from 'vue'
import App from './App.vue'
import router from './router/router'
import store from './store'
import api from './api'
import ElementPlus from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import 'element-plus/dist/index.css'
import 'dayjs/locale/zh-cn'
import handleTree from '@/utils/common'
import AuthorityDirective from "@/permission/index";
import storage from '@/utils/storage'
import { setAppContext } from '@/utils/appContext'

import './assets/css/global.css'
import * as ElIconModules from '@element-plus/icons-vue'

const app = createApp(App)

for (let iconName in ElIconModules) {
    app.component(iconName, ElIconModules[iconName])
}

app.config.globalProperties.$storage = storage
app.config.globalProperties.$api = api
app.config.globalProperties.$handleTree = handleTree

app.config.errorHandler = (err, _, info) => {
    console.error('全局错误处理:', err, info)
    if (err.message && err.message.includes('TOKEN') || err.message && err.message.includes('401')) {
        console.log('检测到 token 相关错误，准备跳转登录页')
    }
}

window.addEventListener('unhandledrejection', event => {
    console.error('未处理的 Promise 错误:', event.reason)
    event.preventDefault()
})

app.use(router)
app.use(store)
app.use(ElementPlus, {
    locale: zhCn
})
app.use(handleTree)
AuthorityDirective.install(app)

setAppContext(app._context)
app.mount('#app')

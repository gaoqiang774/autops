.
.
.
.
.
.
.
<template>
  <div id="app" :class="{ 'visitor-readonly': visitor }">
    <router-view></router-view>
  </div>
</template>

<script>
import { ElMessage } from 'element-plus'
import { isVisitor, VISITOR_DENIED_MESSAGE } from '@/utils/visitor'

export default {
  name: 'App',
  computed: {
    visitor() {
      const admin = this.$store.state.sysAdmin || {}
      return String(admin.username || '').trim().toLowerCase() === 'test'
    }
  },
  mounted() {
    document.addEventListener('click', this.guardVisitorButton, true)
  },
  beforeUnmount() {
    document.removeEventListener('click', this.guardVisitorButton, true)
  },
  methods: {
    guardVisitorButton(event) {
      if (!isVisitor()) return

      const button = event.target.closest('button, .el-button')
      if (!button || this.isVisitorNavigationControl(button)) return

      event.preventDefault()
      event.stopImmediatePropagation()
      ElMessage.warning(VISITOR_DENIED_MESSAGE)
    },
    isVisitorNavigationControl(button) {
      return Boolean(button.closest(
        '.el-pagination, .el-dialog__headerbtn, .visitor-navigation, ' +
        '.nav-trigger-btn, .collapse-btn, .screen-entry'
      ))
    }
  }
}
</script>

<style>
#app.visitor-readonly button:not(.el-dialog__headerbtn):not(.nav-trigger-btn):not(.collapse-btn):not(.screen-entry) {
  opacity: 0.5;
  cursor: not-allowed;
}

#app.visitor-readonly .el-pagination button {
  opacity: 1;
  cursor: pointer;
}

#app.visitor-readonly button.visitor-navigation {
  opacity: 1;
  cursor: pointer;
}
</style>

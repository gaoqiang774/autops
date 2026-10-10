<template>
  <section class="task-view" aria-labelledby="template-title">
    <div class="task-panel">
      <header class="task-panel__header">
        <div class="task-panel__title">
          <h2 id="template-title">模板库</h2>
          <p>维护 Shell、Python 与 Ansible 执行内容</p>
        </div>
        <div class="task-toolbar">
          <el-input
            v-model="filters.name"
            clearable
            placeholder="模板名称"
            aria-label="按模板名称筛选"
            @keyup.enter="loadTemplates"
            @clear="loadTemplates"
          >
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>
          <el-select v-model="filters.type" clearable placeholder="全部类型" aria-label="按模板类型筛选" @change="loadTemplates">
            <el-option v-for="(item, value) in TEMPLATE_TYPES" :key="value" :label="item.label" :value="Number(value)" />
          </el-select>
          <el-button type="primary" :loading="loading" @click="loadTemplates">
            <el-icon><Search /></el-icon>查询
          </el-button>
          <el-button class="ops-refresh-button" type="warning" :loading="loading" aria-label="刷新模板列表" @click="resetFilters">
            <el-icon><Refresh /></el-icon>刷新
          </el-button>
          <el-button type="success" @click="openCreate">
            <el-icon><Plus /></el-icon>新建模板
          </el-button>
        </div>
      </header>

      <div class="task-panel__body">
        <div class="task-table-wrap" v-loading="loading">
          <el-table :data="templates" height="calc(100vh - 300px)" min-height="320" table-layout="fixed">
            <el-table-column prop="name" label="模板名称" min-width="210" show-overflow-tooltip>
              <template #default="{ row }">
                <div class="task-name-cell">
                  <strong>{{ row.name || '--' }}</strong>
                  <span>{{ row.remark || '暂无备注' }}</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column prop="type" label="类型" width="110">
              <template #default="{ row }">
                <el-tag :type="templateType(row.type).type" effect="light">{{ templateType(row.type).label }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="创建人" width="130" show-overflow-tooltip>
              <template #default="{ row }">{{ row.createdBy || row.created_by || '--' }}</template>
            </el-table-column>
            <el-table-column label="更新人" width="130" show-overflow-tooltip>
              <template #default="{ row }">{{ row.updatedBy || row.updated_by || '--' }}</template>
            </el-table-column>
            <el-table-column label="更新时间" width="175">
              <template #default="{ row }">{{ formatDate(row.updatedAt || row.updated_at || row.createdAt || row.created_at) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="178" fixed="right" align="right">
              <template #default="{ row }">
                <el-button link type="primary" @click="previewTemplate(row)">预览</el-button>
                <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
                <el-button link type="danger" @click="removeTemplate(row)">删除</el-button>
              </template>
            </el-table-column>
            <template #empty>
              <el-empty description="暂无任务模板">
                <el-button type="success" @click="openCreate">新建模板</el-button>
              </el-empty>
            </template>
          </el-table>
        </div>
      </div>
    </div>

    <el-dialog
      v-model="dialogVisible"
      class="task-dialog"
      modal-class="task-form-dialog-overlay"
      width="760px"
      destroy-on-close
      :close-on-click-modal="false"
    >
      <template #header>
        <div class="task-dialog-title">
          <span class="task-dialog-title__icon"><el-icon><Document /></el-icon></span>
          <div>
            <h2>{{ editingId ? '编辑模板' : '新建模板' }}</h2>
            <p>脚本将作为任务执行单元，请确认内容可重复执行</p>
          </div>
        </div>
      </template>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" class="task-dialog" @submit.prevent>
        <div class="task-form-grid">
          <el-form-item label="模板名称" prop="name">
            <el-input v-model="form.name" maxlength="100" show-word-limit placeholder="请输入唯一模板名称" />
          </el-form-item>
          <el-form-item label="模板类型" prop="type">
            <el-select v-model="form.type" placeholder="请选择模板类型">
              <el-option v-for="(item, value) in TEMPLATE_TYPES" :key="value" :label="item.label" :value="Number(value)" />
            </el-select>
          </el-form-item>
          <el-form-item class="is-full" label="模板内容" prop="content">
            <el-input
              v-model="form.content"
              class="task-code-input"
              type="textarea"
              resize="vertical"
              spellcheck="false"
              :placeholder="contentPlaceholder"
            />
          </el-form-item>
          <el-form-item class="is-full" label="备注" prop="remark">
            <el-input v-model="form.remark" type="textarea" :rows="3" maxlength="500" show-word-limit placeholder="说明模板用途和适用范围" />
          </el-form-item>
        </div>
      </el-form>
      <template #footer>
        <div class="task-dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="submitting" @click="submitTemplate">保存</el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog
      v-model="previewVisible"
      class="task-dialog"
      modal-class="task-drawer-dialog-overlay"
      width="760px"
      destroy-on-close
    >
      <template #header>
        <div class="task-dialog-title">
          <span class="task-dialog-title__icon"><el-icon><View /></el-icon></span>
          <div><h2>{{ preview.name || '模板预览' }}</h2><p>{{ templateType(preview.type).label }} · 只读内容</p></div>
        </div>
      </template>
      <pre class="task-code-viewer" v-loading="previewLoading">{{ preview.content || '暂无内容' }}</pre>
    </el-dialog>
  </section>
</template>

<script setup>
import { computed, nextTick, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import taskApi from '@/api/task'
import { TEMPLATE_TYPES, errorMessage, formatDate, optionOf } from './taskCommon'

const loading = ref(false)
const submitting = ref(false)
const previewLoading = ref(false)
const dialogVisible = ref(false)
const previewVisible = ref(false)
const editingId = ref(null)
const templates = ref([])
const formRef = ref()
const filters = reactive({ name: '', type: null })
const preview = reactive({ name: '', type: 1, content: '' })
const form = reactive({ name: '', type: 1, content: '', remark: '' })

const rules = {
  name: [{ required: true, message: '请输入模板名称', trigger: 'blur' }],
  type: [{ required: true, message: '请选择模板类型', trigger: 'change' }],
  content: [{ required: true, message: '请输入模板内容', trigger: 'blur' }]
}

const contentPlaceholder = computed(() => ({
  1: '#!/bin/bash\nset -e\n',
  2: '#!/usr/bin/env python3\n',
  3: '---\n- hosts: all\n  tasks:\n'
}[form.type] || '请输入模板内容'))

const templateType = value => optionOf(TEMPLATE_TYPES, value)

function normalizeList(data) {
  if (Array.isArray(data)) return data
  return data?.list || data?.data || []
}

async function loadTemplates() {
  loading.value = true
  try {
    let data
    if (filters.name.trim()) data = await taskApi.queryTemplatesByName(filters.name.trim())
    else if (filters.type) data = await taskApi.queryTemplatesByType(filters.type)
    else data = await taskApi.getTemplates()
    let list = normalizeList(data)
    if (filters.name.trim() && filters.type) {
      list = list.filter(item => Number(item.type) === Number(filters.type))
    }
    templates.value = list
  } catch (error) {
    templates.value = []
    ElMessage.error(errorMessage(error, '模板列表加载失败'))
  } finally {
    loading.value = false
  }
}

function resetForm() {
  editingId.value = null
  Object.assign(form, { name: '', type: 1, content: '', remark: '' })
  nextTick(() => formRef.value?.clearValidate())
}

function openCreate() {
  resetForm()
  dialogVisible.value = true
}

async function openEdit(row) {
  editingId.value = row.id
  dialogVisible.value = true
  try {
    const detail = await taskApi.getTemplate(row.id)
    Object.assign(form, {
      name: detail?.name || row.name || '',
      type: Number(detail?.type || row.type || 1),
      content: detail?.content || row.content || '',
      remark: detail?.remark || row.remark || ''
    })
  } catch (error) {
    dialogVisible.value = false
    ElMessage.error(errorMessage(error, '模板详情加载失败'))
  }
}

async function submitTemplate() {
  try {
    await formRef.value?.validate()
  } catch (_) {
    return
  }
  submitting.value = true
  const payload = {
    name: form.name.trim(),
    type: Number(form.type),
    content: form.content,
    remark: form.remark.trim()
  }
  try {
    if (editingId.value) await taskApi.updateTemplate(editingId.value, payload)
    else await taskApi.createTemplate(payload)
    ElMessage.success(editingId.value ? '模板已更新' : '模板已创建')
    dialogVisible.value = false
    await loadTemplates()
  } catch (error) {
    ElMessage.error(errorMessage(error, '模板保存失败'))
  } finally {
    submitting.value = false
  }
}

async function previewTemplate(row) {
  Object.assign(preview, { name: row.name, type: row.type, content: row.content || '' })
  previewVisible.value = true
  previewLoading.value = true
  try {
    preview.content = await taskApi.getTemplateContent(row.id)
  } catch (error) {
    ElMessage.error(errorMessage(error, '模板内容加载失败'))
  } finally {
    previewLoading.value = false
  }
}

async function removeTemplate(row) {
  try {
    await ElMessageBox.confirm(`确定删除模板“${row.name}”吗？`, '删除模板', {
      type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消'
    })
    await taskApi.deleteTemplate(row.id)
    ElMessage.success('模板已删除')
    await loadTemplates()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(errorMessage(error, '模板删除失败'))
  }
}

loadTemplates()
</script>

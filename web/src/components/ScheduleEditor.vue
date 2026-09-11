<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { NButton, NInput, NInputNumber, NSelect, useMessage } from 'naive-ui'

type Item = { id: string; start: string; title: string; focus_minutes: number; break_minutes: number; until_end?: boolean }
type Template = { id: string; name: string; items: Item[] }

const emit = defineEmits<{ close: [] }>()
const message = useMessage()
const date = ref(new Date().toISOString().slice(0, 10))
const items = ref<Item[]>([])
const templates = ref<Template[]>([])
const templateName = ref('')
const saving = ref(false)

function newItem() {
  items.value.push({ id: crypto.randomUUID(), start: '09:00', title: '新日程', focus_minutes: 50, break_minutes: 10 })
}

async function load() {
  const response = await fetch(`/api/v1/schedules?date=${date.value}`)
  if (!response.ok) {
    message.error('读取日程失败')
    return
  }
  const payload = await response.json()
  items.value = payload.day.items
  templates.value = payload.templates
}

async function save() {
  saving.value = true
  try {
    const response = await fetch('/api/v1/schedules', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ date: date.value, items: items.value }),
    })
    if (!response.ok) throw new Error()
    message.success('当日日程已保存')
  } catch {
    message.error('保存日程失败，请检查时间与内容')
  } finally {
    saving.value = false
  }
}

async function saveTemplate() {
  const name = templateName.value.trim()
  if (!name) {
    message.warning('请输入模板名称')
    return
  }
  const response = await fetch('/api/v1/schedule-templates', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id: crypto.randomUUID(), name, items: items.value }),
  })
  if (!response.ok) {
    message.error('另存模板失败')
    return
  }
  templateName.value = ''
  message.success('模板已保存')
  await load()
}

function applyTemplate(id: string) {
  const template = templates.value.find((item) => item.id === id)
  if (!template) return
  items.value = structuredClone(template.items).map((item: Item) => ({ ...item, id: crypto.randomUUID() }))
  message.success('模板已应用到未保存编辑内容')
}

async function delTemplate(id: string) {
  const response = await fetch(`/api/v1/schedule-templates?id=${encodeURIComponent(id)}`, { method: 'DELETE' })
  if (!response.ok) {
    message.error('删除模板失败')
    return
  }
  message.success('模板已删除')
  await load()
}

onMounted(() => void load())
</script>

<template>
  <main class="editor">
    <div class="editor-date">
      <label for="schedule-date">日期</label>
      <input id="schedule-date" v-model="date" type="date" @change="load" />
    </div>

    <section v-for="(item, index) in items" :key="item.id" class="item">
      <NInput v-model:value="item.start" aria-label="开始时间" placeholder="09:00" class="item-time" />
      <NInput v-model:value="item.title" aria-label="日程内容" placeholder="日程内容" />
      <NInputNumber v-model:value="item.focus_minutes" :min="1" aria-label="专注分钟" />
      <span>分钟</span>
      <NInputNumber v-model:value="item.break_minutes" :min="0" aria-label="休息分钟" />
      <span>休息</span>
      <NButton size="small" @click="items.splice(index, 1)">删除</NButton>
    </section>

    <div class="editor-actions">
      <NButton @click="newItem">添加日程项</NButton>
      <NButton type="primary" :loading="saving" @click="save">保存当日日程</NButton>
    </div>

    <section class="templates">
      <h3>模板</h3>
      <div class="template-actions">
        <NSelect :options="templates.map((item) => ({ label: item.name, value: item.id }))" placeholder="应用模板" @update:value="applyTemplate" />
        <NInput v-model:value="templateName" placeholder="模板名称" />
        <NButton @click="saveTemplate">另存为模板</NButton>
      </div>
      <div v-for="template in templates" :key="template.id" class="template-row">
        <span>{{ template.name }}</span>
        <NButton size="tiny" @click="delTemplate(template.id)">删除模板</NButton>
      </div>
    </section>

    <div class="editor-footer">
      <NButton @click="emit('close')">关闭</NButton>
    </div>
  </main>
</template>

<style scoped>
.editor { display: flex; flex-direction: column; gap: 16px; }
.editor-date, .item, .editor-actions, .template-actions, .template-row, .editor-footer { display: flex; gap: 10px; align-items: center; }
.editor-date input { color-scheme: dark; }
.item > :deep(.n-input), .item > :deep(.n-input-number), .template-actions > :deep(.n-select), .template-actions > :deep(.n-input) { flex: 1; min-width: 0; }
.item-time { max-width: 100px; }
.templates { padding-top: 12px; border-top: 1px solid var(--n-divider-color); }
.templates h3 { margin: 0 0 10px; font-size: 14px; }
.template-row { justify-content: space-between; margin-top: 8px; }
.editor-footer { justify-content: flex-end; }
@media (max-width: 760px) { .item, .template-actions { align-items: stretch; flex-direction: column; } .item-time { max-width: none; } }
</style>

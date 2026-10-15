import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import assert from 'node:assert/strict'
import { test } from 'node:test'
import ts from 'typescript'
import { ref, computed } from 'vue'

const source = readFileSync(new URL('../src/views/SettingsView.vue', import.meta.url), 'utf8')
  .match(/<script setup lang="ts">([\s\S]*?)<\/script>/)[1]
  .replace(/^import .*$/gm, '')
const defaults = {
  version: 1,
  dashboard: { window: '1d', max_active: 1, max_inactive: 1, refresh_seconds: 30, daily_boot_enabled: true },
  attention: { protect_minutes: 5, focus_add_minutes: 15, mask_strength: 1 },
  time_estimate: { minutes_per_message: 3, fallback_ratio: .3 },
  custom_knowledge_tips: [],
}
const clone = (value) => JSON.parse(JSON.stringify(value))
async function editor(initial = defaults) {
  let stored = clone(initial), failSave = false
  const requests = [], events = []
  const context = vm.createContext({
    ref, computed, onMounted() {}, defineExpose() {},
    defineEmits: () => (name) => events.push(name),
    useMessage: () => ({ success() {} }), window: { confirm: () => false },
    fetch: async (url, options = {}) => {
      requests.push([url, options])
      if (url === '/api/v1/project-roots') return { ok: true, json: async () => [] }
      if (options.method === 'PUT') {
        if (failSave) return { ok: false, json: async () => ({ error: '写入失败' }) }
        stored = JSON.parse(options.body)
        return { ok: true, json: async () => clone(stored) }
      }
      return { ok: true, json: async () => ({ ...clone(stored), defaults: clone(defaults) }) }
    },
  })
  vm.runInContext(ts.transpileModule(source, {
    compilerOptions: { target: ts.ScriptTarget.ES2022 },
  }).outputText, context)
  const run = (code) => vm.runInContext(code, context)
  await run('load()')
  return { run, events, requests, stored: () => clone(stored), failSave: () => { failSave = true } }
}

test('settings editor adds, validates, edits, saves, reloads and deletes custom knowledge', async () => {
  const api = await editor()
  api.run('addKnowledgeTip(); addKnowledgeTip()')
  const ids = clone(api.run('value.value.custom_knowledge_tips.map(t => t.id)'))
  assert.equal(new Set(ids).size, 2)
  assert.match(api.run('validationError.value'), /标题和知识正文/)
  await api.run('save()')
  assert.equal(api.requests.filter(([, options]) => options.method === 'PUT').length, 0)
  api.run('value.value.custom_knowledge_tips.forEach((tip, i) => { tip.title = `标题 ${i}`; tip.theory = `正文 ${i}` })')
  assert.equal(api.run('validationError.value'), '')
  assert.equal(api.run('dirty.value'), true)
  assert.equal(api.run('requestClose()'), false, 'unsaved changes can be retained')
  await api.run('save()')
  assert.equal(api.run('dirty.value'), false)
  assert.deepEqual(api.events, ['saved'])
  await api.run('load()')
  assert.deepEqual(api.stored().custom_knowledge_tips.map(t => t.id), ids)
  api.run('value.value.custom_knowledge_tips[0].theory = "edited"')
  await api.run('save()')
  assert.equal(api.stored().custom_knowledge_tips[0].id, ids[0])
  assert.equal(api.stored().custom_knowledge_tips[0].theory, 'edited')
  api.run('removeKnowledgeTip(value.value.custom_knowledge_tips[0].id)')
  await api.run('save()')
  assert.deepEqual(api.stored().custom_knowledge_tips.map(t => t.id), [ids[1]])
})

test('save failure retains the draft and persisted content; legacy settings load as an empty list', async () => {
  const legacy = clone(defaults)
  delete legacy.custom_knowledge_tips
  const api = await editor(legacy)
  assert.equal(api.run('value.value.custom_knowledge_tips.length'), 0)
  api.run('addKnowledgeTip(); value.value.custom_knowledge_tips[0].title = "标题"; value.value.custom_knowledge_tips[0].theory = "正文"')
  api.failSave()
  await api.run('save()')
  assert.equal(api.run('error.value'), '写入失败')
  assert.equal(api.run('dirty.value'), true)
  assert.equal(api.run('value.value.custom_knowledge_tips.length'), 1)
  assert.equal(api.stored().custom_knowledge_tips, undefined)
  assert.deepEqual(api.events, [])
})

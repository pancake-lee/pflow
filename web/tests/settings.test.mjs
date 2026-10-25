import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import assert from 'node:assert/strict'
import { test } from 'node:test'
import ts from 'typescript'
import { ref, computed } from 'vue'

const source = readFileSync(new URL('../src/views/SettingsView.vue', import.meta.url), 'utf8')
  .match(/<script setup lang="ts">([\s\S]*?)<\/script>/)[1].replace(/^import .*$/gm, '')
const baseline = {
  version: 1,
  dashboard: { window: '1d', max_active: 1, max_inactive: 1, agent_filter: 'all', refresh_seconds: 30, daily_boot_enabled: true },
  attention: { protect_minutes: 5, focus_add_minutes: 15, mask_strength: 1 },
  time_estimate: { minutes_per_message: 3, fallback_ratio: .3 },
}
function settings(fetchOverride) {
  const requests = [], events = []
  const context = vm.createContext({
    ref, computed, Error, onMounted: () => {}, defineExpose: () => {},
    defineEmits: () => event => events.push(event),
    useMessage: () => ({ success() {} }), window: { confirm: () => true },
    fetch: async (url, options) => {
      requests.push({ url, options })
      if (fetchOverride) return fetchOverride(url, options)
      return { ok: true, json: async () => JSON.parse(JSON.stringify(url.includes('project-roots') ? []
        : options ? JSON.parse(options.body) : { ...baseline, defaults: baseline })) }
    },
  })
  vm.runInContext(ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText, context)
  return { run: code => vm.runInContext(code, context), requests, events }
}
test('explicit settings save clears dirty state and emits a dashboard refresh', async () => {
  const api = settings()
  await api.run('load()')
  assert.equal(api.run('dirty.value'), false)
  api.run("value.value.dashboard.window = '7d'")
  assert.equal(api.run('dirty.value'), true)
  await api.run('save()')
  assert.equal(api.run('dirty.value'), false)
  assert.deepEqual(api.events, ['saved'])
})
test('Agent type is saved as a shared dashboard setting', async () => {
  const api = settings()
  await api.run('load()')
  api.run("value.value.dashboard.agent_filter = 'codex'")
  await api.run('save()')
  const saved = JSON.parse(api.requests.at(-1).options.body)
  assert.equal(saved.dashboard.agent_filter, 'codex')
  assert.equal(api.run('value.value.dashboard.agent_filter'), 'codex')
  assert.deepEqual(api.events, ['saved'])
})
test('category and individual restore modify only the draft until explicit save', async () => {
  const api = settings()
  await api.run('load()')
  api.run("value.value.dashboard.window = '7d'; value.value.dashboard.max_active = 3; resetField('dashboard','window')")
  assert.equal(api.run('value.value.dashboard.window'), '1d')
  assert.equal(api.run('value.value.dashboard.max_active'), 3)
  api.run("reset('dashboard')")
  assert.equal(api.run('value.value.dashboard.max_active'), 1)
  assert.equal(api.requests.length, 2)
})
test('dirty modal close is protected and busy close is rejected', async () => {
  const api = settings()
  await api.run('load()')
  api.run("value.value.dashboard.window = '7d'; window.confirm = () => false")
  assert.equal(api.run('requestClose()'), false)
  assert.equal(api.run('value.value.dashboard.window'), '7d')
  api.run('window.confirm = () => true; saving.value = true')
  assert.equal(api.run('requestClose()'), false)
  api.run('saving.value = false')
  assert.equal(api.run('requestClose()'), true)
  assert.deepEqual(api.events, ['close'])
})
test('project updates preserve other settings drafts and refresh the dashboard', async () => {
  const api = settings()
  await api.run('load()')
  api.run("value.value.dashboard.window = '7d'; newPath.value = '/work/project'")
  await api.run('addRoot()')
  assert.equal(api.run('value.value.dashboard.window'), '7d')
  assert.equal(api.run('dirty.value'), true)
  assert.deepEqual(api.events, ['saved'])
})
test('invalid or cleared numeric settings cannot be saved', async () => {
  const api = settings()
  await api.run('load()')
  api.run('value.value.dashboard.max_active = null')
  assert.match(api.run('validationError.value'), /Active/)
  await api.run('save()')
  assert.equal(api.requests.length, 2)
})
test('failed save preserves the draft and gives visible feedback', async () => {
  const api = settings((url, options) => {
    if (options) throw new Error('offline')
    return { ok: true, json: async () => JSON.parse(JSON.stringify(url.includes('project-roots') ? [] : { ...baseline, defaults: baseline })) }
  })
  await api.run('load()')
  api.run("value.value.dashboard.window = '7d'")
  await api.run('save()')
  assert.equal(api.run('dirty.value'), true)
  assert.equal(api.run('saving.value'), false)
  assert.equal(api.run('error.value'), 'offline')
})
test('failed initial load leaves a retryable error instead of a fake empty settings page', async () => {
  const api = settings(() => ({ ok: false, json: async () => ({ error: 'unavailable' }) }))
  await api.run('load()')
  assert.equal(api.run('loading.value'), false)
  assert.equal(api.run('value.value'), null)
  assert.equal(api.run('error.value'), 'unavailable')
})

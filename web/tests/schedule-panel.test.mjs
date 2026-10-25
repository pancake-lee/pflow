import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import assert from 'node:assert/strict'
import { test } from 'node:test'
import ts from 'typescript'
import { ref, computed } from 'vue'

const source = readFileSync(new URL('../src/components/SchedulePanel.vue', import.meta.url), 'utf8')
  .match(/<script setup lang="ts">([\s\S]*?)<\/script>/)[1].replace(/^import .*$/gm, '')
function panel(fetch) {
  const errors = []
  const context = vm.createContext({
    ref, computed, fetch, document: { visibilityState: 'visible' }, onMounted: () => {}, onUnmounted: () => {},
    defineEmits: () => () => {}, useMessage: () => ({ error: message => errors.push(message) }),
    highlightToMarquee: () => ({ visible: false }),
  })
  vm.runInContext(ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText, context)
  vm.runInContext("current.value = {id: 'a', title: 'A'}; day.value = '2026-10-13'", context)
  return { run: code => vm.runInContext(code, context), errors }
}
test('a failed action gives feedback and retains the current item for retry', async () => {
  const api = panel(async () => { throw new Error('offline') })
  await api.run("act('completed')")
  assert.equal(api.run('current.value.id'), 'a')
  assert.equal(api.run('acting.value'), false)
  assert.equal(api.errors.length, 1)
})
test('an in-flight action prevents duplicate completion and reloads after success', async () => {
  let release, posts = 0
  const api = panel(async (_, options) => {
    if (options) {
      posts++
      if (JSON.parse(options.body).action === 'completed') await new Promise(resolve => { release = resolve })
      return { ok: true, json: async () => ({ events: [] }) }
    }
    return { ok: true, json: async () => ({ day: {date:'2026-10-13'}, current:{id:'b',title:'B'}, phase:'focus' }) }
  })
  const first = api.run("act('completed')")
  assert.equal(api.run('acting.value'), true)
  await api.run("act('skipped')")
  assert.equal(posts, 1)
  release()
  await first
  assert.equal(api.run('current.value.id'), 'b')
  await new Promise(resolve => setImmediate(resolve))
  assert.equal(posts, 2)
  assert.equal(api.run('acting.value'), false)
})

test('automatic start retries after failure and restarts after a pause', async () => {
  let posts = 0
  const api = panel(async (_, options) => {
    if (options) {
      posts++
      return { ok: posts !== 1, json: async () => ({ events: [] }) }
    }
    return { ok: true, json: async () => ({ day: { date: '2026-10-13' }, current: { id: 'a', title: 'A' }, phase: 'focus' }) }
  })
  api.run('maybeAutoStart()')
  await new Promise(resolve => setImmediate(resolve))
  assert.equal(posts, 1)
  api.run('maybeAutoStart()')
  await new Promise(resolve => setImmediate(resolve))
  assert.equal(posts, 2)
  api.run("events.value = [{ id: 'pause-1', task_id: 'a', action: 'pause', time: '2026-10-13T09:10:00Z', duration_seconds: 600 }]")
  api.run('maybeAutoStart()')
  await new Promise(resolve => setImmediate(resolve))
  assert.equal(posts, 3)
})

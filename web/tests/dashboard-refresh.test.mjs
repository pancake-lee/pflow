import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import assert from 'node:assert/strict'
import { test } from 'node:test'
import ts from 'typescript'
import { ref } from 'vue'

const source = readFileSync(new URL('../src/composables/useDashboard.ts', import.meta.url), 'utf8')
  .replace(/^import .*$/gm, '')
  .replace('export function useDashboard', 'function useDashboard')
  .replace('import.meta.env.VITE_PFLOW_DESKTOP', "'0'")

function dashboard(fetch) {
  const context = vm.createContext({ ref, fetch, URLSearchParams, Error })
  vm.runInContext(ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText, context)
  return vm.runInContext('useDashboard()', context)
}

const opts = { window: '1d', max_active: 1, max_inactive: 1 }
function response(now, cached = false) {
  return {
    ok: true,
    headers: { get: name => name === 'X-Pflow-Cache' && cached ? 'hit' : null },
    json: async () => ({ now, sessions: [] }),
  }
}

test('first request displays cached result and a failed refresh keeps it visible', async () => {
  const urls = []
  const state = dashboard(async url => {
    urls.push(url)
    if (urls.length === 1) return response('2026-10-25T09:00:00Z', true)
    throw new Error('offline')
  })
  assert.equal(await state.fetchDashboard(opts, true), true)
  assert.equal(state.data.value.now, '2026-10-25T09:00:00Z')
  await state.fetchDashboard(opts)
  assert.equal(state.data.value.now, '2026-10-25T09:00:00Z')
  assert.equal(state.error.value, 'offline')
  assert.match(urls[0], /first=1/)
  assert.doesNotMatch(urls[1], /first=1/)
})

test('an older in-flight request cannot replace a newer result', async () => {
  let finishOld
  let count = 0
  const state = dashboard(async () => {
    count++
    if (count === 1) return new Promise(resolve => { finishOld = resolve })
    return response('2026-10-25T10:00:00Z')
  })
  const old = state.fetchDashboard(opts)
  await state.fetchDashboard(opts)
  finishOld(response('2026-10-25T09:00:00Z'))
  await old
  assert.equal(state.data.value.now, '2026-10-25T10:00:00Z')
  assert.equal(state.loading.value, false)
})

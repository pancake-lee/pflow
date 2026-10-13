import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import assert from 'node:assert/strict'
import { test } from 'node:test'
import ts from 'typescript'
import { ref } from 'vue'

const script = readFileSync(new URL('../src/views/DashboardView.vue', import.meta.url), 'utf8')
const source = script.slice(script.indexOf('const windowOptions ='), script.indexOf('// ── Filtering'))
// Only the real preference declarations and functions are needed here.
const preferenceSource = source.slice(0, source.indexOf('watch('))

function preferences(stored, revision = '') {
  let persisted
  const context = vm.createContext({
    ref,
    localStorage: {
      getItem: () => JSON.stringify(stored),
      setItem: (_, value) => { persisted = JSON.parse(value) },
    },
  })
  vm.runInContext(ts.transpileModule(preferenceSource, {
    compilerOptions: { target: ts.ScriptTarget.ES2022 },
  }).outputText, context)
  const run = (code) => vm.runInContext(code, context)
  run(`dashboardRevision.value = ${JSON.stringify(revision)}`)
  return { run, persisted: () => persisted }
}

const stored = { window: '7d', maxActive: 3, maxInactive: 0, refreshInterval: 60, agentFilter: 'codex' }
test('legacy browser preferences are retained until an explicit settings save', () => {
  const p = preferences(stored)
  assert.equal(p.run('loadFilterPreferences().window'), '7d')
})
test('saved settings invalidate only the shared browser preferences', () => {
  const p = preferences({ ...stored, dashboardRevision: 'older' }, 'newer')
  assert.equal(p.run('loadFilterPreferences().window'), undefined)
  assert.equal(p.run('loadFilterPreferences().maxActive'), undefined)
  assert.equal(p.run('loadFilterPreferences().maxInactive'), undefined)
  assert.equal(p.run('loadFilterPreferences().refreshInterval'), undefined)
  assert.equal(p.run('loadFilterPreferences().agentFilter'), 'codex')
})
test('new browser preferences are stamped and remain independent', () => {
  const p = preferences(stored, 'saved')
  p.run("filterPreferencesLoaded.value = true; selectedWindow.value = '3d'; maxInactive.value = 0; persistFilterPreferences()")
  const next = preferences(p.persisted(), 'saved')
  assert.equal(next.run('loadFilterPreferences().window'), '3d')
  assert.equal(next.run('loadFilterPreferences().maxInactive'), 0)
  assert.equal(p.persisted().dashboardRevision, 'saved')
})

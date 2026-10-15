import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import assert from 'node:assert/strict'
import { test } from 'node:test'
import ts from 'typescript'
import { ref } from 'vue'

const source = readFileSync(new URL('../src/App.vue', import.meta.url), 'utf8')
  .match(/<script setup lang="ts">([\s\S]*?)<\/script>/)[1]
  .replace(/import[\s\S]*?from\s+['"][^'"]+['"]/g, '')
  .replaceAll('import.meta.env.VITE_PFLOW_DESKTOP', 'desktopBuild')

function app(fetch, desktop = false) {
  let mounted
  const context = vm.createContext({
    ref, fetch, desktopBuild: desktop ? '1' : undefined, onMounted: (callback) => { mounted = callback },
  })
  vm.runInContext(ts.transpileModule(source, {
    compilerOptions: { target: ts.ScriptTarget.ES2022 },
  }).outputText, context)
  return { mount: async () => { mounted(); await new Promise(setImmediate) }, run: (code) => vm.runInContext(code, context) }
}

test('disabled daily boot finishes startup without fetching the boot API', async () => {
  const requests = []
  const instance = app(async (url) => {
    requests.push(url)
    return { ok: true, json: async () => ({ dashboard: { daily_boot_enabled: false } }) }
  })
  await instance.mount()
  assert.deepEqual(requests, ['/api/v1/settings'])
  assert.equal(instance.run('bootChecked.value && !showBoot.value'), true)
})

for (const completed of [false, true]) {
  test(`enabled daily boot completes startup with completed=${completed}`, async () => {
    const instance = app(async (url) => ({
      ok: true,
      json: async () => url.includes('settings')
        ? { dashboard: { daily_boot_enabled: true } }
        : { completed, goal: 'test goal' },
    }))
    await instance.mount()
    assert.equal(instance.run('bootChecked.value'), true)
    assert.equal(instance.run('showBoot.value'), !completed)
    assert.equal(instance.run('todayGoal.value'), 'test goal')
  })
}

for (const failure of ['settings-network', 'settings-json', 'boot-network', 'boot-http']) {
  test(`startup finishes when ${failure} fails`, async () => {
    const instance = app(async (url) => {
      const settings = url.includes('settings')
      if ((settings && failure === 'settings-network') ||
          (!settings && failure === 'boot-network')) throw new Error('offline')
      return {
        ok: settings || failure !== 'boot-http',
        json: async () => {
          if (settings && failure === 'settings-json') throw new Error('invalid JSON')
          return settings ? { dashboard: { daily_boot_enabled: true } } : {}
        },
      }
    })
    await instance.mount()
    assert.equal(instance.run('bootChecked.value && !showBoot.value'), true)
  })
}

test('modal header and Escape updates are routed to the editor close guard', () => {
  const instance = app(() => {})
  instance.run('showScheduleEditor.value = true; scheduleEditor.value = { requestClose: () => false }')
  instance.run('updateScheduleVisibility(false)')
  assert.equal(instance.run('showScheduleEditor.value'), true)
  instance.run('scheduleEditor.value = { requestClose: () => { showScheduleEditor.value = false; return true } }')
  instance.run('updateScheduleVisibility(false)')
  assert.equal(instance.run('showScheduleEditor.value'), false)
  const template = readFileSync(new URL('../src/App.vue', import.meta.url), 'utf8')
  assert.match(template, /:show="showScheduleEditor"\s+@update:show="updateScheduleVisibility"/)
  assert.match(template, /ScheduleEditor ref="scheduleEditor" @close="showScheduleEditor = false"/)
})

test('settings modal updates respect the settings close guard', () => {
  const instance = app(() => {})
  instance.run('showSettings.value = true; settingsView.value = { requestClose: () => false }')
  instance.run('updateSettingsVisibility(false)')
  assert.equal(instance.run('showSettings.value'), true)
  instance.run('settingsView.value = { requestClose: () => { showSettings.value = false; return true } }')
  instance.run('updateSettingsVisibility(false)')
  assert.equal(instance.run('showSettings.value'), false)
})


test('desktop waits for the connection ready event before requesting remote settings', async () => {
  const requests = []
  const instance = app(async (url) => {
    requests.push(url)
    return { ok: true, json: async () => ({ dashboard: { daily_boot_enabled: false } }) }
  }, true)
  await instance.mount()
  assert.deepEqual(requests, [])
  assert.equal(instance.run('bootChecked.value'), false)
  await instance.run('checkBoot()')
  assert.deepEqual(requests, ['/api/v1/settings'])
  assert.equal(instance.run('bootChecked.value && !showBoot.value'), true)
})

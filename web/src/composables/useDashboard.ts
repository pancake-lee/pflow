import { ref, type Ref } from 'vue'
import type { DashboardResponse, ScanOptions } from '../types/dashboard'

export function useDashboard() {
  const data: Ref<DashboardResponse | null> = ref(null)
  const loading = ref(false)
  const error = ref<string | null>(null)
	let latestRequest = 0

  async function fetchDashboard(opts: ScanOptions, first = false): Promise<boolean> {
    const request = ++latestRequest
    loading.value = true
    error.value = null

    try {
      const params = new URLSearchParams({
        window: opts.window,
        max_active: String(opts.max_active),
        max_inactive: String(opts.max_inactive),
      })
      if (first) params.set('first', '1')
      const resp = await fetch(`/api/v1/dashboard?${params}`)
      if (!resp.ok) {
        const body = resp.headers.get('content-type')?.includes('application/json')
          ? await resp.json() as { error?: string }
          : {}
        throw new Error(body.error || `HTTP ${resp.status}: ${resp.statusText}`)
      }
      const incoming = (await resp.json()) as DashboardResponse
      if (request === latestRequest) {
        if (import.meta.env.VITE_PFLOW_DESKTOP === '1') {
          for (const session of incoming.sessions ?? []) session.has_terminal = false
        }
        data.value = incoming
      }
      return request === latestRequest && resp.headers.get('X-Pflow-Cache') === 'hit'
    } catch (e) {
      if (request === latestRequest) error.value = e instanceof Error ? e.message : 'Unknown error'
      return false
    } finally {
      if (request === latestRequest) loading.value = false
    }
  }

  return { data, loading, error, fetchDashboard }
}

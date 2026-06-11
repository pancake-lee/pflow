import { ref, type Ref } from 'vue'
import type { DashboardResponse, ScanOptions } from '../types/dashboard'

export function useDashboard() {
  const data: Ref<DashboardResponse | null> = ref(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchDashboard(opts: ScanOptions) {
    loading.value = true
    error.value = null

    try {
      const params = new URLSearchParams({
        window: opts.window,
        max_inactive: String(opts.max_inactive),
      })
      const resp = await fetch(`/api/v1/dashboard?${params}`)
      if (!resp.ok) {
        throw new Error(`HTTP ${resp.status}: ${resp.statusText}`)
      }
      data.value = (await resp.json()) as DashboardResponse
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Unknown error'
      data.value = null
    } finally {
      loading.value = false
    }
  }

  return { data, loading, error, fetchDashboard }
}

import { ref, type Ref } from 'vue'
import type {
  DashboardResponse,
  ScanOptions,
  StartSessionRequest,
  StartSessionResponse,
} from '../types/dashboard'

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

  async function startSession(req: StartSessionRequest): Promise<StartSessionResponse> {
    const resp = await fetch('/api/v1/sessions/start', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(req),
    })
    if (!resp.ok) {
      const text = await resp.text()
      throw new Error(text)
    }
    return (await resp.json()) as StartSessionResponse
  }

  async function sendToSession(sessionId: string, prompt: string): Promise<void> {
    const resp = await fetch(`/api/v1/sessions/${sessionId}/send`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ prompt }),
    })
    if (!resp.ok) {
      const text = await resp.text()
      throw new Error(text)
    }
  }

  async function respondPermission(
    sessionId: string,
    requestId: string,
    behavior: 'allow' | 'deny',
  ): Promise<void> {
    const resp = await fetch(`/api/v1/sessions/${sessionId}/permission`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ request_id: requestId, behavior }),
    })
    if (!resp.ok) {
      const text = await resp.text()
      throw new Error(text)
    }
  }

  return { data, loading, error, fetchDashboard, startSession, sendToSession, respondPermission }
}

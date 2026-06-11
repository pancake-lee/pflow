import { ref, onUnmounted, watch, type Ref } from 'vue'
import type { RefreshInterval } from '../types/dashboard'

export function usePolling(callback: () => void, interval: Ref<RefreshInterval>) {
  const timer = ref<ReturnType<typeof setInterval> | null>(null)

  function start(seconds: number) {
    stop()
    if (seconds > 0) {
      timer.value = setInterval(callback, seconds * 1000)
    }
  }

  function stop() {
    if (timer.value !== null) {
      clearInterval(timer.value)
      timer.value = null
    }
  }

  watch(interval, (val) => {
    start(val)
  }, { immediate: true })

  onUnmounted(stop)

  return { start, stop }
}

/**
 * Format an ISO 8601 timestamp as a relative "X ago" string.
 */
export function formatSince(iso: string): string {
  if (!iso) return 'n/a'
  const d = Date.now() - new Date(iso).getTime()
  if (d < 60_000) return 'just now'
  if (d < 3_600_000) return `${Math.floor(d / 60_000)}m ago`
  if (d < 86_400_000) {
    const hours = Math.floor(d / 3_600_000)
    const mins = Math.floor((d % 3_600_000) / 60_000)
    if (hours === 1 && mins === 0) return '1h ago'
    return `${hours}h ${mins}m ago`
  }
  const days = Math.floor(d / 86_400_000)
  if (days === 1) return '1d ago'
  return `${days}d ago`
}

/**
 * Format an ISO 8601 timestamp as a human-readable date-time string.
 */
export function formatTime(iso: string): string {
  if (!iso) return 'n/a'
  const d = new Date(iso)
  return d.toLocaleString()
}

/**
 * Truncate a string to `n` characters, appending "…" if truncated.
 */
export function truncate(s: string, n: number): string {
  if (!s) return '—'
  if (s.length <= n) return s
  return s.slice(0, n) + '…'
}

/**
 * Escape newlines and tabs for display in table cells.
 */
export function escapeNewlines(s: string): string {
  if (!s) return ''
  return s
    .replace(/\r\n/g, '\\r\\n')
    .replace(/\n/g, '\\n')
    .replace(/\r/g, '\\r')
    .replace(/\t/g, '\\t')
}

/**
 * Format cumulative active minutes for display.
 * - < 1 min → "<1m"
 * - < 60 min → "Xm" (e.g., "15m")
 * - 60+ min → "Xh Ym" (e.g., "2h 15m")
 */
export function formatMinutes(mins: number): string {
  if (!mins || mins <= 0) return '0m'
  if (mins < 1) return '<1m'
  const h = Math.floor(mins / 60)
  const m = Math.round(mins % 60)
  if (h === 0) return `${m}m`
  if (m === 0) return `${h}h`
  return `${h}h ${m}m`
}

/**
 * Shorten a session ID for display.
 */
export function shortID(id: string, n = 16): string {
  if (!id) return ''
  return id.length <= n ? id : id.slice(0, n)
}

/**
 * Composable for the dual-dimension attention visual effects:
 *
 *   1. Highlight (高亮) — marquee border animation, speed / width / opacity
 *      mapped from the 0-100 highlight score.
 *   2. Fog (迷雾) — page-background-coloured overlay, opacity mapped from
 *      the 0-100 fog_pct score.
 *
 * Tunable parameters are in @/config/attention.ts — edit there to
 * fine-tune the visual feedback.
 */

import { MARQUEE, FOG, FOCUS } from '../config/attention'

// ── Highlight → marquee mapping ────────────────────────────────────

export interface MarqueeParams {
  /** Animation duration in seconds (lower = faster). */
  speed: number
  /** Border width in px. */
  width: number
  /** Opacity of the marquee (0-1). */
  opacity: number
  /** Whether the marquee is visible at all. */
  visible: boolean
}

/**
 * Linearly map `value` from [0, 100] to [min, max].
 * lerp = 线性插值
 */
function lerp(value: number, min: number, max: number): number {
  if (value <= 0) return min 
  if (value >= 100) return max
  return min + (value / 100) * (max - min)
}

/**
 * Convert a 0-100 highlight score to marquee animation parameters.
 *
 *   HL   0 → invisible
 *   HL   1 → slow, thin, dim (speedMax / widthMin / opacityMin)
 *   HL 100 → fast, thick, bright (speedMin / widthMax / opacityMax)
 */
export function highlightToMarquee(hl: number): MarqueeParams {
  // Debug override: when set (>= 0), bypass the algorithm output
  if (MARQUEE.debugHighlight >= 0) {
    hl = MARQUEE.debugHighlight
  }

  // 让0输出0，保证可以完全无效果，但1则有了分数，则可以提高效果的下限
  if (hl <= 0) {
    return { speed: 0, width: 0, opacity: 0, visible: false }
  }
  // hl=0→low, hl=100→high.  Speed: lowSpeed (slower=more seconds) → highSpeed (faster=fewer seconds).
  const speed = lerp(hl, MARQUEE.lowSpeed, MARQUEE.highSpeed)
  const width = lerp(hl, MARQUEE.lowWidth, MARQUEE.highWidth)
  const opacity = lerp(hl, MARQUEE.lowOpacity, MARQUEE.highOpacity)
  return { speed, width, opacity, visible: true }
}

// ── Fog → CSS opacity mapping ──────────────────────────────────────

/**
 * Convert a 0-100 fog_pct to the CSS opacity for the fog overlay.
 *
 *   fog   0 → opacityMin (clear)
 *   fog 100 → opacityMax (maximum fog)
 */
export function fogPctToOpacity(fog: number): number {
  // Debug override: when set (>= 0), bypass the algorithm output
  if (FOG.debugFogPct >= 0) {
    fog = FOG.debugFogPct
  }
  // 让0输出0，保证可以完全无效果，但1则有了分数，则可以提高效果的下限
  if (fog <= 0) return 0
  return lerp(fog, FOG.lowOpacity, FOG.highOpacity)
}

// Re-export FOG.maskImage for convenience (used in components)
export { FOG as FOG_CONFIG }

// Re-export FOCUS for focus-mode dimming
export { FOCUS as FOCUS_CONFIG }

// ── Level label (unchanged) ────────────────────────────────────────

/**
 * Return a human-readable label for a reminder level.
 */
export function levelLabel(level: string): string {
  switch (level) {
    case 'high': return '⚠️ 高提醒'
    case 'medium': return '🔔 中提醒'
    case 'low': return '💤 低提醒'
    default: return ''
  }
}

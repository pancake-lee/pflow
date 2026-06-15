/**
 * Attention Visual Effects — Tunable Parameters
 *
 * Centralised configuration for the highlight marquee and fog overlay
 * effects.  Edit values here to fine-tune the visual feedback; no need to
 * touch component code or composables.
 *
 * Naming convention:
 *   highXxx → value at score = 100  (max intensity)
 *   lowXxx  → value at score = 0    (min intensity)
 *
 * For speed, "high speed" means fast = fewer seconds per rotation.
 */

// ── Marquee (highlight border animation) ────────────────────────────

export const MARQUEE = {

  // ── Speed (seconds per full rotation) ─────────────────────────────
  //
  // Highlight 100 → highSpeed  (fast marquee)
  // Highlight   0 → lowSpeed   (slow marquee; NOT visible at hl=0, range starts at hl≈1)

  highSpeed: 2,   // seconds at highlight = 100 (fastest)
  lowSpeed:  10,  // seconds at highlight ≈ 1   (slowest)

  // ── Border width (px) ─────────────────────────────────────────────
  //
  // Width of the glowing border strip exposed by the mask.

  highWidth: 4.0,  // px at highlight = 100
  lowWidth:  1.0,  // px at highlight ≈ 1

  // ── Border opacity ─────────────────────────────────────────────────
  //
  // Opacity of the marquee pseudo-element (0–1).

  highOpacity: 1,  // at highlight = 100
  lowOpacity:  0,  // at highlight ≈ 1

  // ── Debug override ──────────────────────────────────────────────────
  //
  // Set to 0–100 to force the highlight score, bypassing the algorithm.
  // Set to -1 (default) to use the real algorithm output.
  //
  //   -1  → disabled (use algorithm)
  //   0   → invisible
  //   100 → max intensity

  debugHighlight: -1,
} as const

// ── Fog (visual suppression overlay) ─────────────────────────────────

export const FOG = {

  // ── Opacity range ──────────────────────────────────────────────────
  //
  // Fog 100 → highOpacity  (maximum fog density)
  // Fog   0 → lowOpacity   (fully clear)

  // 这个写法意味着当分数是60时，不透明度映射到100了
  highOpacity: 1,     // at fog_pct = 100 (max suppression)
  lowOpacity:  0.6,     // at fog_pct = 0   (fully clear)

  // ── Fog mask image ─────────────────────────────────────────────────
  //
  // Reserved for a future fog texture / pattern image.  When set to a
  // non-empty URL, the fog overlay will use this image (cropped to the
  // card bounds) instead of the solid page-background colour.

  maskImage: '',

  // ── Debug override ──────────────────────────────────────────────────
  //
  // Set to 0–100 to force the fog_pct, bypassing the algorithm.
  // Set to -1 (default) to use the real algorithm output.
  //
  //   -1  → disabled (use algorithm)
  //   0   → fully clear
  //   100 → maximum fog density

  debugFogPct: -1,
} as const

// ── Focus mode (专注模式 统一遮罩) ───────────────────────────────────

export const FOCUS = {
  /** Opacity of the dimming overlay on non-focused areas during focus mode (0-1). */
  dimOpacity: 1,
} as const

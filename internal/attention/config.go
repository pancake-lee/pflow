// Package attention implements the reminder score algorithm and attention
// mask infrastructure for pflow's intelligent scheduling (阶段四).
//
// The algorithm computes a per-project reminder score based on session
// states, waiting durations, focus streaks, and project priorities.
// See docs/design/02-reminder_score_algorithm.md for the full design.
package attention

// Configurable constants for the reminder score algorithm.
// Defaults are hardcoded; optionally overridable via ~/.pflow/config.json
// (attention section, see docs/design/02-reminder_score_algorithm.md §5).
const (
	// CurWindow is the lookback window in minutes for determining the
	// current active project. The project with the most recent session
	// activity within this window is considered "cur". If all sessions
	// are older than this, there is no current project.
	CurWindow = 60.0 // 1 hour

	// ProtectMin is the focus protection period in minutes.
	// While streak_cur < ProtectMin, no reminders fire for other projects.
	// TODO: make configurable via ~/.pflow/config.json attention.protect_min
	ProtectMin = 5.0 // lowered from 15 for MVP visual verification

	// WWait is the weight applied to waiting duration (base score).
	WWait = 1.0

	// WStreak is the weight applied to the streak ratio factor.
	// factor_i = min((streak_cur / PROTECT_MIN) * W_STREAK, 2.0)
	WStreak = 0.5

	// PrimaryBonus is the multiplier when the current active project is
	// secondary and the target is primary (战略重要性bonus).
	PrimaryBonus = 2.0

	// WCorrect is the weight for today's cumulative time correction.
	// Added when primary's total < average of secondary totals.
	WCorrect = 0.5

	// ExpPower is the exponent for the power-function differentiation step.
	// final = raw ^ EXP_POWER, widening the gap between high and low scores.
	ExpPower = 2.0

	// MinFactor is the floor for the focus interference factor when streak
	// cannot be measured (no busy sessions, new server start, etc.).
	// Without this, an unmeasurable streak_cur=0 would make the factor 0
	// for all other projects, silently suppressing all reminders.
	// MinFactor=0.5 means: assume neutral-to-gentle focus when unknown.
	MinFactor = 0.5
)

// ReminderThresholds maps score ranges to reminder levels.
// score < 2 → none, 2 ≤ score < 5 → low, 5 ≤ score < 10 → medium, ≥ 10 → high.
var ReminderThresholds = []float64{2, 5, 10}

// Fog score constants for the dual-dimension attention algorithm.
// See docs/design/02-reminder_score_algorithm.md §5 and §7.
const (
	// FogProtectMax is the maximum fog score during the protection period.
	// Applied when focus mode is active and the user hasn't yet accumulated
	// the target focus duration.
	FogProtectMax = 0.9

	// FogProtectMin is the minimum fog score during the protection period.
	// As the protection period nears its end, fog approaches this value.
	FogProtectMin = 0.5

	// FogBaseNonProtect is the base fog score for non-current projects
	// when not in a protection period and there is no reminder urgency.
	FogBaseNonProtect = 0.3

	// NoCurrentOtherFog is the fixed fog score for non-primary projects
	// when there is no current active project (user is away).
	NoCurrentOtherFog = 0.7

	// HighlightLogScale is the multiplier used to convert raw reminder scores
	// to a 0-100 highlight intensity via a logarithmic scale.
	// highlight = min(100, round(log10(reminder_score + 1) * HighlightLogScale))
	HighlightLogScale = 25.0
)

// Package timetrack provides active-time estimation for pflow projects.
//
// It implements a three-tier degradation chain as specified in
// docs/design/09-active-time-calculation.md:
//
//	Priority 1: tmux focus events (highest precision)
//	Priority 2: message count × MinutesPerMessage
//	Priority 3: wall-clock span × FallbackRatio (final fallback)
package timetrack

import "time"

const (
	// MinutesPerMessage is the estimated active interaction time per user
	// message. One message represents roughly: reading agent output
	// (1-2 min) + thinking (1-2 min) + editing prompt (30 sec).
	MinutesPerMessage = 3.0

	// FallbackRatio is the fraction of wall-clock span used when no
	// message count is available. A session open all day (16h) with
	// this ratio reports ~4.8h, which is still an overestimate but far
	// less severe than the raw 16h.
	FallbackRatio = 0.3
)

// SessionData is the minimal per-session input needed for time estimation.
type SessionData struct {
	MessageCount int
	FirstActive  time.Time
	LastActive   time.Time
}

// SessionTodayMinutes estimates today's active minutes for one session
// using the degradation chain:
//
//  1. If msgCount > 0 → msgCount × MinutesPerMessage
//  2. Otherwise → wall-clock span (clamped to [todayStart, now]) × FallbackRatio
//
// firstActive and lastActive are the session's activity timestamps.
// todayStart is midnight of the target day; now is the current time.
func SessionTodayMinutes(msgCount int, firstActive, lastActive, todayStart, now time.Time) float64 {
	if msgCount > 0 {
		return float64(msgCount) * MinutesPerMessage
	}
	return wallClockToday(firstActive, lastActive, todayStart, now) * FallbackRatio
}

// ProjectTodayMinutes computes today's active minutes for a project using
// the full three-tier degradation chain:
//
//  1. If focusLog has data for this project → use focus event aggregation
//  2. Otherwise → sum per-session message estimates (msgCount × 3 min)
//  3. Fallback → per-session wall-clock span × 0.3
//
// sessions contains all sessions belonging to this project.
// focusLog may be nil if focus events are not available.
func ProjectTodayMinutes(projectPath string, sessions []SessionData, focusLog *FocusLog, todayStart, now time.Time) float64 {
	// Tier 1: tmux focus events (highest precision)
	if focusLog != nil {
		if mins := focusLog.ProjectMinutes(projectPath, todayStart, now); mins > 0 {
			return mins
		}
	}

	// Tier 2 & 3: per-session message or wall-clock fallback
	var total float64
	for _, s := range sessions {
		total += SessionTodayMinutes(s.MessageCount, s.FirstActive, s.LastActive, todayStart, now)
	}
	return total
}

// wallClockToday computes the wall-clock overlap between a session's
// [firstActive, lastActive] interval and [todayStart, now].
func wallClockToday(firstActive, lastActive, todayStart, now time.Time) float64 {
	if firstActive.IsZero() || lastActive.IsZero() {
		return 0
	}
	start := firstActive
	if start.Before(todayStart) {
		start = todayStart
	}
	end := lastActive
	if end.After(now) {
		end = now
	}
	if end.After(start) {
		return end.Sub(start).Minutes()
	}
	return 0
}

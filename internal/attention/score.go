package attention

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	plogger "github.com/pancake-lee/pgo/pkg/plogger"
)

// CalculateScores computes reminder and fog scores for a set of projects.
//
// The dual-dimension algorithm follows docs/design/02-reminder_score_algorithm.md:
//
//	1. Determine current active project (cur) — the one with the most
//	   recent session activity within CurWindow (1 hour). If all sessions
//	   are older than CurWindow, there is no current project.
//	2. base_i = waiting_i * W_WAIT
//	3. Focus interference factor:
//	   - When focusActive: if streak_cur < focusMinutes → factor_i = 0
//	   - Otherwise: factor_i = min((streak_cur / PROTECT_MIN) * W_STREAK, 2.0)
//	   - cur=secondary ∧ target=primary → factor_i *= PRIMARY_BONUS
//	4. No active task → only primary gets adjusted = base, rest = 0
//	5. Today correction for primary (fairness vs secondary)
//	6. final = raw ^ EXP_POWER (power function to widen gaps)
//	7. Compute fog_score per §5.2 (MVP concise version)
//
// focusedProject is the project path the user explicitly clicked "专注" on.
// When non-empty, this project is always kept clear (fog=0) and other projects
// receive protection fog during the protection period.
//
// focusActive/focusMinutes: when the user has explicitly enabled focus mode,
// the protection period suppresses reminders for other projects until the
// accumulated focus time exceeds focusMinutes. When focus is not active,
// no protection period is applied.
func CalculateScores(inputs map[string]ReminderInput, now time.Time, focusActive bool, focusedProject string, focusMinutes float64) map[string]ReminderOutput {
	if len(inputs) == 0 {
		return nil
	}

	plogger.Infof("[attention] ======== focus_active=%v focus_project=%s focus_min=%.0f ========", focusActive, focusedProject, focusMinutes)
	for path, in := range inputs {
		label := ""
		if in.IsPrimary {
			label = " (主线)"
		}
		since := now.Sub(in.LastActive).Minutes()
		plogger.Infof("[attention] in  | %s%s | wait=%.0f  streak=%.0f  today=%.0f  last=%.0fm",
			shortPath(path), label, in.Waiting, in.Streak, in.Total, since)
	}

	// Step 1: find current active project — the one with the most recent
	// session activity within CurWindow. If all sessions are older than
	// CurWindow, there is no current project (user has been away).
	curWindowDuration := time.Duration(CurWindow) * time.Minute
	var curProject string
	var mostRecent time.Time
	for path, in := range inputs {
		if in.LastActive.After(mostRecent) {
			mostRecent = in.LastActive
			curProject = path
		}
	}

	// If most recent activity is older than CurWindow, reset cur to nil
	if now.Sub(mostRecent) > curWindowDuration {
		curProject = ""
	}

	sinceRecent := now.Sub(mostRecent).Minutes()

	// streak_cur is used for protection period and factor calculation.
	//
	// NOTE: streak only measures "agent busy" duration (Claude processing
	// time after user submits input). We cannot track user thinking time or
	// prompt-writing time in the CLI — these happen outside our observation.
	// Therefore the measured streak is a *lower bound* of actual focus time.
	//
	// When the server just started or session metadata hasn't caught up,
	// streak may be 0 even though the user has been working for a while.
	// In that case we apply a floor: if the user is clearly active (last
	// activity within 5 minutes) AND focus mode is NOT explicitly active,
	// assume at least PROTECT_MIN minutes of focus — we just couldn't measure it.
	//
	// When focus mode IS active, the user explicitly set a protection window;
	// we trust their intent and don't override with the floor.
	streakCur := 0.0
	measuredStreak := 0.0
	if curProject != "" {
		measuredStreak = inputs[curProject].Streak
		streakCur = measuredStreak
	}

	if !focusActive && streakCur < 1.0 && curProject != "" && sinceRecent < 5.0 {
		streakCur = ProtectMin
		plogger.Infof("[attention]     streak floor=%.0f applied (measured=%.1f, last=%.0fm ago)", streakCur, measuredStreak, sinceRecent)
	}
	if curProject != "" {
		plogger.Infof("[attention] cur: %s (streak=%.0f)", shortPath(curProject), streakCur)
	} else {
		plogger.Infof("[attention] cur: none (last_active=%.0fm ago > %.0fm window)", sinceRecent, CurWindow)
	}

	// Step 2-3: compute base scores with focus interference
	type projectScore struct {
		path    string
		raw     float64
		waiting float64
		streak  float64
	}

	plogger.Infof("[attention] step2-3: base + focus interference:")

	var scored []projectScore
	for path, in := range inputs {
		isCur := (path == curProject)

		// 3.2: base_i = waiting_i * W_WAIT
		base := in.Waiting * WWait

		// 3.3: Focus interference factor
		factor := 1.0
		var factorReason string

		if curProject == "" {
			// No active project: only primary gets its base score
			if in.IsPrimary {
				factorReason = "no_cur_is_primary"
			} else {
				factor = 0
				factorReason = "no_cur_not_primary"
			}
		} else if isCur {
			// Current active project gets no reminder
			factor = 0
			factorReason = "is_cur"
		} else {
			// Other project — compute focus interference
			//
			// Protection period ONLY applies when focus mode is
			// explicitly active. Without explicit focus, we never
			// suppress reminders (the user hasn't opted in).
			if focusActive && streakCur < focusMinutes {
				factor = 0
				factorReason = fmt.Sprintf("protected(streak=%.1f < focus=%.0f)", streakCur, focusMinutes)
			} else if streakCur < 1.0 {
				// Streak unmeasurable (no busy sessions, new server, etc.)
				// Use MinFactor instead of computing streak_ratio=0 which
				// would silently suppress all reminders.
				factor = MinFactor
				factorReason = fmt.Sprintf("min_factor=%.2f", MinFactor)

				// Strategic bonus still applies
				curIsPrimary := inputs[curProject].IsPrimary
				if !curIsPrimary && in.IsPrimary {
					factor *= PrimaryBonus
					factorReason += fmt.Sprintf(" * primary_bonus=%.2f", factor)
				}
			} else {
				streakRatio := (streakCur / ProtectMin) * WStreak
				factor = math.Min(streakRatio, 2.0)
				factorReason = fmt.Sprintf("streak_ratio=%.2f", factor)

				// Strategic bonus: when current is secondary and target is primary
				curIsPrimary := inputs[curProject].IsPrimary
				if !curIsPrimary && in.IsPrimary {
					factor *= PrimaryBonus
					factorReason += fmt.Sprintf(" * primary_bonus=%.2f", factor)
				}
			}
		}

		adjusted := base * factor

		plogger.Infof("[attention]     %s: base=%.0f*%.0f=%.0f factor=%s raw=%.1f",
			shortPath(path), in.Waiting, WWait, base, factorReason, adjusted)

		// 3.5: Today cumulative correction for primary
		correction := 0.0
		if in.IsPrimary && adjusted > 0 {
			correction = computeTodayCorrection(in.Total, inputs)
			if correction > 0 {
				plogger.Infof("[attention]     %s: today_correction +%.2f → %.1f", shortPath(path), correction, adjusted+correction)
				adjusted += correction
			}
		}

		scored = append(scored, projectScore{
			path:    path,
			raw:     adjusted,
			waiting: in.Waiting,
			streak:  in.Streak,
		})
	}

	// Step 6: Power function to widen gaps, then compute fog scores
	// First pass: compute all reminder scores (final = raw ^ ExpPower)
	reminderFinals := make(map[string]float64, len(scored))
	maxReminder := 0.0
	for _, s := range scored {
		final := math.Pow(s.raw, ExpPower)
		reminderFinals[s.path] = final
		if s.path != curProject && final > maxReminder {
			maxReminder = final
		}
	}

	// Second pass: compute fog scores and build result
	result := make(map[string]ReminderOutput, len(scored))
	for _, s := range scored {
		final := reminderFinals[s.path]
		level := scoreToLevel(final)
		isCur := s.path == curProject
		isPrimary := inputs[s.path].IsPrimary
		isFocused := focusActive && focusedProject != "" && s.path == focusedProject
		hasFocused := focusActive && focusedProject != ""
		fog := computeFogScore(isCur, isFocused, hasFocused, isPrimary, curProject != "", final, maxReminder, focusActive, streakCur, focusMinutes)

		hl := reminderToHighlight(final)
		fp := int(math.Round(fog * 100))
		curMark := ""
		if isCur {
			curMark = " (cur)"
		}

		plogger.Infof("[attention] out | %s | scr=%.0f  hl=%d  fog=%.2f  fg%%=%d  lv=%s%s",
			shortPath(s.path), final, hl, fog, fp, level, curMark)

		result[s.path] = ReminderOutput{
			Score:     math.Round(final*100) / 100,
			FogScore:  math.Round(fog*1000) / 1000,
			Highlight: hl,
			FogPct:    fp,
			Level:     level,
			Waiting:   s.waiting,
			Streak:    s.streak,
			IsCurrent: isCur,
		}
	}
	return result
}

// computeFogScore calculates the fog suppression score for a project.
//
// Fog score is in [0, 1], where 0 means fully clear (no mask) and 1 means
// fully fogged (maximum visual suppression).
//
// The logic follows docs/design/02-reminder_score_algorithm.md §5.2:
//
//	if i == focusedProject               → fog = 0
//	elif hasFocused && focusActive        → fog = protection formula for non-focused
//	elif i == curProject                  → fog = 0
//	elif curProject == ""                 → fog = 0 if primary else NoCurrentOtherFog
//	elif focusActive && streakCur < focus → fog = protection formula
//	else                                  → fog based on reminder score ratio
func computeFogScore(
	isCur bool,
	isFocused bool,
	hasFocused bool,
	isPrimary bool,
	hasCur bool,
	reminderScore float64,
	maxReminder float64,
	focusActive bool,
	streakCur float64,
	focusMinutes float64,
) float64 {
	// Rule 1: focused project is always clear
	if isFocused {
		return 0
	}

	// Rule 2: when focus mode is active on a specific project, all other
	// projects get protection fog (regardless of whether they are "cur").
	if hasFocused && focusActive && streakCur < focusMinutes {
		remain := (focusMinutes - streakCur) / focusMinutes
		return FogProtectMax*remain + FogProtectMin*(1-remain)
	}

	// Rule 3: current project is always clear (when no focused project)
	if isCur {
		return 0
	}

	// Rule 4: no current active project
	if !hasCur {
		if isPrimary {
			return 0
		}
		return NoCurrentOtherFog
	}

	// Rule 5: protection period (focus mode active, streak not yet reached target)
	if focusActive && streakCur < focusMinutes {
		remain := (focusMinutes - streakCur) / focusMinutes
		return FogProtectMax*remain + FogProtectMin*(1-remain)
	}

	// Rule 6: non-protection period — fog inversely proportional to reminder score.
	// When no project has a positive reminder score (maxReminder == 0), there is
	// no attention competition. Keep the primary project clear as the default
	// focus target; other projects get base fog.
	if maxReminder == 0 {
		if isPrimary {
			return 0
		}
		return FogBaseNonProtect
	}
	ratio := reminderScore / maxReminder
	if ratio > 1 {
		ratio = 1
	}
	return FogBaseNonProtect * (1 - ratio)
}

// computeTodayCorrection adds a fairness correction when the primary project
// has less cumulative focus time today than the average of secondary projects.
//
//	correction = max(0, avg_secondary_total - primary_total) * W_CORRECT
//
// All secondary projects contribute to the average, including the current
// active project if it is secondary (§4.1 step 5, test case 7).
func computeTodayCorrection(primaryTotal float64, inputs map[string]ReminderInput) float64 {
	var secondaryTotals []float64
	for _, in := range inputs {
		if in.IsPrimary {
			continue
		}
		secondaryTotals = append(secondaryTotals, in.Total)
	}
	if len(secondaryTotals) == 0 {
		return 0
	}

	// Use median instead of mean to be robust against outliers
	sort.Float64s(secondaryTotals)
	var avg float64
	n := len(secondaryTotals)
	if n%2 == 0 {
		avg = (secondaryTotals[n/2-1] + secondaryTotals[n/2]) / 2
	} else {
		avg = secondaryTotals[n/2]
	}

	if primaryTotal < avg {
		return (avg - primaryTotal) * WCorrect
	}
	return 0
}

// reminderToHighlight converts a raw reminder score to a 0-100 highlight
// intensity using a logarithmic scale. This makes the score human-readable
// and directly usable by the frontend for visual effects.
//
//	highlight = min(100, round(log10(score + 1) * HighlightLogScale))
func reminderToHighlight(score float64) int {
	if score <= 0 {
		return 0
	}
	h := int(math.Round(math.Log10(score+1) * HighlightLogScale))
	if h > 100 {
		return 100
	}
	return h
}

// scoreToLevel maps a final score to a reminder level string.
func scoreToLevel(score float64) string {
	for i, threshold := range ReminderThresholds {
		if score < threshold {
			switch i {
			case 0:
				return "none"
			case 1:
				return "low"
			case 2:
				return "medium"
			}
		}
	}
	return "high"
}

// shortPath returns the last two path components for log readability.
func shortPath(path string) string {
	parts := strings.Split(strings.TrimRight(path, "/"), "/")
	if len(parts) <= 2 {
		return path
	}
	return strings.Join(parts[len(parts)-2:], "/")
}

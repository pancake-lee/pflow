package timetrack

import (
	"testing"
	"time"
)

func TestProjectMinutesMergesBriefSameProjectFocusGap(t *testing.T) {
	base := time.Date(2026, 9, 11, 9, 0, 0, 0, time.UTC)
	log := &FocusLog{ByProject: map[string][]FocusEvent{"/work/a": {
		{Timestamp: base, Action: "focus-in"},
		{Timestamp: base.Add(20 * time.Minute), Action: "focus-out"},
		{Timestamp: base.Add(23 * time.Minute), Action: "focus-in"},
		{Timestamp: base.Add(40 * time.Minute), Action: "focus-out"},
	}}}
	if got := log.ProjectMinutes("/work/a", base, base.Add(time.Hour)); got != 40 {
		t.Fatalf("minutes=%v, want 40", got)
	}
}

func TestProjectMinutesDoesNotMergeLongGapOrInvalidSequence(t *testing.T) {
	base := time.Date(2026, 9, 11, 9, 0, 0, 0, time.UTC)
	log := &FocusLog{ByProject: map[string][]FocusEvent{"/work/a": {
		{Timestamp: base, Action: "focus-out"},
		{Timestamp: base.Add(time.Minute), Action: "focus-in"},
		{Timestamp: base.Add(11 * time.Minute), Action: "focus-out"},
		{Timestamp: base.Add(20 * time.Minute), Action: "focus-in"},
		{Timestamp: base.Add(30 * time.Minute), Action: "focus-out"},
	}}}
	if got := log.ProjectMinutes("/work/a", base, base.Add(time.Hour)); got != 20 {
		t.Fatalf("minutes=%v, want 20", got)
	}
}

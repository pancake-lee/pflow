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

func TestProjectMinutesSeparatesProjectsAndClosesOpenSegment(t *testing.T) {
	base := time.Date(2026, 9, 11, 9, 0, 0, 0, time.UTC)
	log := &FocusLog{ByProject: map[string][]FocusEvent{
		"/work/a": {{Timestamp: base, Action: "focus-in"}, {Timestamp: base.Add(20 * time.Minute), Action: "focus-out"}},
		"/work/b": {{Timestamp: base.Add(20 * time.Minute), Action: "focus-in"}},
	}}
	if got := log.ProjectMinutes("/work/a", base, base.Add(40*time.Minute)); got != 20 {
		t.Fatalf("project a minutes=%v, want 20", got)
	}
	if got := log.ProjectMinutes("/work/b", base, base.Add(40*time.Minute)); got != 20 {
		t.Fatalf("project b minutes=%v, want 20", got)
	}
}

func TestProjectMinutesDoesNotMergeAcrossOtherProject(t *testing.T) {
	base := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	for _, hasOut := range []bool{true, false} {
		log := &FocusLog{ByProject: map[string][]FocusEvent{
			"a": {{Timestamp: base, Action: "focus-in"}, {Timestamp: base.Add(23 * time.Minute), Action: "focus-in"}, {Timestamp: base.Add(40 * time.Minute), Action: "focus-out"}},
			"b": {{Timestamp: base.Add(20 * time.Minute), Action: "focus-in"}, {Timestamp: base.Add(23 * time.Minute), Action: "focus-out"}},
		}}
		if hasOut {
			log.ByProject["a"] = append(log.ByProject["a"], FocusEvent{Timestamp: base.Add(20 * time.Minute), Action: "focus-out"})
		}
		if got := log.ProjectMinutes("a", base, base.Add(time.Hour)); got != 37 {
			t.Fatalf("hasOut=%v a=%v want 37", hasOut, got)
		}
		if got := log.ProjectMinutes("b", base, base.Add(time.Hour)); got != 3 {
			t.Fatalf("b=%v want 3", got)
		}
	}
}

func TestProjectMinutesClipsMidnightAndIgnoresFuture(t *testing.T) {
	base := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	log := &FocusLog{ByProject: map[string][]FocusEvent{"a": {
		{Timestamp: base.Add(-30 * time.Minute), Action: "focus-in"},
		{Timestamp: base.Add(20 * time.Minute), Action: "focus-out"},
		{Timestamp: base.Add(time.Hour), Action: "focus-in"},
	}}}
	if got := log.ProjectMinutes("a", base, base.Add(30*time.Minute)); got != 20 {
		t.Fatalf("got %v want 20", got)
	}
}

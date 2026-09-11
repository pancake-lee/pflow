package codex

import (
	"testing"
	"time"

	"github.com/pancake-lee/pflow/internal/config"
)

func TestApplySessionLimitsSeparatesActiveAndInactivePerProject(t *testing.T) {
	now := time.Now()
	input := []SessionSummary{
		{SessionID: "a-new", Project: "/a", Status: "busy", LastActive: now},
		{SessionID: "a-old", Project: "/a", Status: "idle", LastActive: now.Add(-time.Minute)},
		{SessionID: "a-inactive", Project: "/a", Status: "unknown", LastActive: now.Add(-2 * time.Minute)},
		{SessionID: "b", Project: "/b", Status: "busy", LastActive: now.Add(-3 * time.Minute)},
	}
	got := applySessionLimits(input, 1, 1)
	if len(got) != 3 {
		t.Fatalf("got %d sessions, want 3", len(got))
	}
	for _, session := range got {
		if session.SessionID == "a-old" {
			t.Fatal("second active session was not limited")
		}
	}
}

func TestApplySessionLimitsZeroHidesAll(t *testing.T) {
	now := time.Now()
	input := []SessionSummary{
		{SessionID: "a-new", Project: "/a", Status: "busy", LastActive: now},
		{SessionID: "a-inactive", Project: "/a", Status: "unknown", LastActive: now.Add(-2 * time.Minute)},
		{SessionID: "b-inactive", Project: "/b", Status: "completed", LastActive: now.Add(-3 * time.Minute)},
	}
	got := applySessionLimits(input, config.NoSessionLimit, 0)
	if len(got) != 1 || got[0].SessionID != "a-new" {
		t.Fatalf("maxInactive=0 must hide all inactive sessions, got %v", got)
	}
}

func TestApplySessionLimitsNoLimitKeepsEverything(t *testing.T) {
	now := time.Now()
	input := []SessionSummary{
		{SessionID: "a-new", Project: "/a", Status: "busy", LastActive: now},
		{SessionID: "a-old", Project: "/a", Status: "idle", LastActive: now.Add(-time.Minute)},
		{SessionID: "a-inactive", Project: "/a", Status: "unknown", LastActive: now.Add(-2 * time.Minute)},
	}
	got := applySessionLimits(input, config.NoSessionLimit, config.NoSessionLimit)
	if len(got) != len(input) {
		t.Fatalf("NoSessionLimit must keep all sessions, got %d of %d", len(got), len(input))
	}
}

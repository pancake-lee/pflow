package codex

import (
	"testing"
	"time"
)

func TestApplySessionLimitsSeparatesActiveAndInactivePerProject(t *testing.T) {
	now := time.Now()
	input := []SessionSummary{
		{SessionID: "a-new", Project: "/a", Status: "busy", LastActive: now},
		{SessionID: "a-old", Project: "/a", Status: "idle", LastActive: now.Add(-time.Minute)},
		{SessionID: "a-inactive", Project: "/a", Status: "unknown", LastActive: now.Add(-2 * time.Minute)},
		{SessionID: "b", Project: "/b", Status: "busy", LastActive: now.Add(-3 * time.Minute)},
	}
	got := applySessionLimits(input, 1, 0)
	if len(got) != 3 {
		t.Fatalf("got %d sessions, want 3", len(got))
	}
	for _, session := range got {
		if session.SessionID == "a-old" {
			t.Fatal("second active session was not limited")
		}
	}
}

package api

import (
	"testing"
	"time"
)

func TestComputeReminderScoresIncludesCodexSession(t *testing.T) {
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	scores := computeReminderScores([]DashboardEntry{{
		SessionID:   "codex-1",
		AgentType:   "codex",
		Project:     "/work/pflow",
		Status:      "waiting",
		FirstActive: now.Add(-20 * time.Minute),
		LastActive:  now.Add(-8 * time.Minute),
	}}, nil, now, nil)
	score, ok := scores["/work/pflow"]
	if !ok {
		t.Fatal("Codex project missing from reminder scores")
	}
	if score.Waiting != 8 {
		t.Fatalf("waiting=%v, want 8", score.Waiting)
	}
}

package schedule

import (
	"testing"
	"time"
)

func TestSetCurrentLocatesOnceAndDoesNotAutoAdvance(t *testing.T) {
	day := Day{Items: []Item{{ID: "a", Start: "09:00", Title: "A", FocusMinutes: 50}, {ID: "b", Start: "10:00", Title: "B", FocusMinutes: 50}}}
	now := time.Date(2026, 9, 11, 9, 30, 0, 0, time.Local)
	current, _ := SetCurrent(&day, now)
	if current.ID != "a" {
		t.Fatalf("current=%s", current.ID)
	}
	current, _ = SetCurrent(&day, now.Add(2*time.Hour))
	if current.ID != "a" {
		t.Fatalf("advanced without action: %s", current.ID)
	}
	day.Items[0].Status = "completed"
	day.CurrentID = ""
	current, _ = SetCurrent(&day, now.Add(2*time.Hour))
	if current.ID != "b" {
		t.Fatalf("after action=%s", current.ID)
	}
}

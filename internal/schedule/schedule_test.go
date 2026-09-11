package schedule

import (
	"testing"
	"time"
)

func TestSetCurrentLocatesOnceAndDoesNotAutoAdvance(t *testing.T) {
	day := Day{Items: []Item{{ID: "a", Start: "09:00", Title: "A", FocusMinutes: 50}, {ID: "b", Start: "10:00", Title: "B", FocusMinutes: 50}}}
	now := time.Date(2026, 9, 11, 9, 30, 0, 0, time.Local)
	current, next := SetCurrent(&day, now)
	if current.ID != "a" {
		t.Fatalf("current=%s", current.ID)
	}
	if next == nil || next.ID != "b" {
		t.Fatalf("next=%v, want b", next)
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

func TestCurrentReturnsFirstPendingItemAfterFreeTime(t *testing.T) {
	day := Day{Items: []Item{
		{ID: "a", Start: "10:00", Title: "A", FocusMinutes: 50},
		{ID: "b", Start: "11:00", Title: "B", FocusMinutes: 50},
	}}
	now := time.Date(2026, 9, 11, 9, 30, 0, 0, time.Local)

	current, next := SetCurrent(&day, now)
	if current.ID != "" || current.Title != "自由日程" {
		t.Fatalf("current=%+v, want free schedule", current)
	}
	if next == nil || next.ID != "a" {
		t.Fatalf("next=%v, want a", next)
	}
}

func TestCurrentOmitsNextAfterLastPendingItem(t *testing.T) {
	day := Day{Items: []Item{
		{ID: "a", Start: "09:00", Title: "A", FocusMinutes: 50, Status: "completed"},
		{ID: "b", Start: "10:00", Title: "B", FocusMinutes: 50},
	}}
	now := time.Date(2026, 9, 11, 10, 30, 0, 0, time.Local)

	current, next := SetCurrent(&day, now)
	if current.ID != "b" {
		t.Fatalf("current=%s, want b", current.ID)
	}
	if next != nil {
		t.Fatalf("next=%+v, want nil", next)
	}
}

func TestAdvancePromotesDisplayedNextAsCurrent(t *testing.T) {
	day := Day{Items: []Item{
		{ID: "a", Start: "09:00", Title: "A", FocusMinutes: 50},
		{ID: "b", Start: "10:00", Title: "B", FocusMinutes: 50},
		{ID: "c", Start: "11:00", Title: "C", FocusMinutes: 50},
	}}
	// Panel shows b as current, c as next; user completes b at 10:15, before
	// c starts and while the earlier pending a would win a wall-time rederive.
	now := time.Date(2026, 9, 11, 10, 15, 0, 0, time.Local)
	day.CurrentID = "b"
	SetCurrent(&day, now)
	for i := range day.Items {
		if day.Items[i].ID == "b" {
			day.Items[i].Status = "completed"
		}
	}

	current, next := Advance(&day, "b")
	if current.ID != "c" {
		t.Fatalf("current=%s, want c (displayed next must be promoted)", current.ID)
	}
	if next != nil {
		t.Fatalf("next=%+v, want nil after last pending item", next)
	}
	if day.CurrentID != "c" {
		t.Fatalf("CurrentID=%s, want c", day.CurrentID)
	}
}

func TestAdvanceFallsBackToFreeAfterLastItem(t *testing.T) {
	day := Day{CurrentID: "a", Items: []Item{
		{ID: "a", Start: "09:00", Title: "A", FocusMinutes: 50, Status: "skipped"},
	}}

	current, next := Advance(&day, "a")
	if current.ID != "" || current.Title != "自由日程" {
		t.Fatalf("current=%+v, want free schedule", current)
	}
	if next != nil {
		t.Fatalf("next=%+v, want nil", next)
	}
	if day.CurrentID != "" {
		t.Fatalf("CurrentID=%s, want empty", day.CurrentID)
	}
}

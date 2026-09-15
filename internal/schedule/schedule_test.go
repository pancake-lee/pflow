package schedule

import (
	"errors"
	"path/filepath"
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

func TestPrepareDaySavePreservesUnchangedRecordAndRequiresReplacementConfirmation(t *testing.T) {
	completedAt := time.Date(2026, 9, 11, 9, 50, 0, 0, time.UTC)
	previous := Day{Date: "2026-09-11", CurrentID: "b", Items: []Item{
		{ID: "a", Start: "09:00", Title: "A", FocusMinutes: 50, Status: "completed", CompletedAt: completedAt},
		{ID: "b", Start: "10:00", Title: "B", FocusMinutes: 50},
	}}
	next := Day{Date: previous.Date, Items: []Item{
		{ID: "a", Start: "09:00", Title: "A", FocusMinutes: 50},
		{ID: "b", Start: "10:00", Title: "B", FocusMinutes: 50},
	}}

	prepared, err := PrepareDaySave(previous, next, false)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Items[0].Status != "completed" || !prepared.Items[0].CompletedAt.Equal(completedAt) {
		t.Fatalf("record was not preserved: %+v", prepared.Items[0])
	}
	if prepared.CurrentID != "b" {
		t.Fatalf("current ID=%q, want b", prepared.CurrentID)
	}

	next.Items[0].Title = "renamed"
	_, err = PrepareDaySave(previous, next, false)
	if !errors.Is(err, ErrRecordedItemsNeedConfirmation) {
		t.Fatalf("error=%v, want replacement confirmation", err)
	}
	prepared, err = PrepareDaySave(previous, next, true)
	if err != nil || prepared.Items[0].Status != "" {
		t.Fatalf("confirmed replacement=%+v err=%v", prepared.Items[0], err)
	}
}

func TestSaveSanitizesTemplateAndRejectsInvalidTimeline(t *testing.T) {
	manager := NewManagerAt(filepath.Join(t.TempDir(), "schedules.json"))
	file := File{Days: map[string]Day{"2026-09-11": {Date: "2026-09-11", Items: []Item{
		{ID: "a", Start: "09:00", Title: "A", FocusMinutes: 50, BreakMinutes: 10},
		{ID: "b", Start: "09:30", Title: "B", FocusMinutes: 50},
	}}}}
	if err := manager.Save(file); err == nil {
		t.Fatal("overlapping timeline was accepted")
	}

	file.Days["2026-09-11"] = Day{Date: "2026-09-11", Items: []Item{{ID: "a", Start: "09:00", Title: "A", FocusMinutes: 50}}}
	file.Templates = []Template{{ID: "t", Name: "template", Items: []Item{{
		ID: "a", Start: "09:00", Title: "A", FocusMinutes: 50, Status: "completed", CompletedAt: time.Now(),
	}}}}
	if err := manager.Save(file); err != nil {
		t.Fatal(err)
	}
	loaded, err := manager.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Templates[0].Items[0].Status != "" || !loaded.Templates[0].Items[0].CompletedAt.IsZero() {
		t.Fatalf("template retained execution data: %+v", loaded.Templates[0].Items[0])
	}
}

func TestSaveAllowsItemsAcrossMidnight(t *testing.T) {
	manager := NewManagerAt(filepath.Join(t.TempDir(), "schedules.json"))
	file := File{Days: map[string]Day{"2026-09-15": {
		Date: "2026-09-15",
		Items: []Item{
			{ID: "late", Start: "23:30", Title: "late work", FocusMinutes: 30, BreakMinutes: 10},
			{ID: "next", Start: "00:15", DayOffset: 1, Title: "after midnight", FocusMinutes: 50},
		},
	}}}
	if err := manager.Save(file); err != nil {
		t.Fatalf("cross-midnight schedule rejected: %v", err)
	}

	loaded, err := manager.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Days["2026-09-15"].Items[1].DayOffset != 1 {
		t.Fatalf("day offset=%d, want 1", loaded.Days["2026-09-15"].Items[1].DayOffset)
	}
}

func TestCurrentSelectsNextDayItem(t *testing.T) {
	day := Day{Date: "2026-09-15", Items: []Item{
		{ID: "late", Start: "23:30", Title: "late work", FocusMinutes: 20},
		{ID: "next", Start: "00:15", DayOffset: 1, Title: "after midnight", FocusMinutes: 50},
	}}
	now := time.Date(2026, 9, 16, 0, 20, 0, 0, time.Local)
	current, next := SetCurrent(&day, now)
	if current.ID != "next" || next != nil {
		t.Fatalf("current=%+v next=%+v, want next-day item only", current, next)
	}
	if !HasNextDayPending(day, "2026-09-16") {
		t.Fatal("next-day pending item was not detected")
	}
}

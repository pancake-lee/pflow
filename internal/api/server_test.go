package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/pancake-lee/pflow/internal/schedule"
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
	}}, nil, now, nil, 5)
	score, ok := scores["/work/pflow"]
	if !ok {
		t.Fatal("Codex project missing from reminder scores")
	}
	if score.Waiting != 8 {
		t.Fatalf("waiting=%v, want 8", score.Waiting)
	}
}

func TestComputeReminderScoresAttentionTraceIsOptional(t *testing.T) {
	now := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	sessionList := []DashboardEntry{{
		SessionID:   "session-1",
		Project:     "/work/pflow",
		Status:      "waiting",
		FirstActive: now.Add(-20 * time.Minute),
		LastActive:  now.Add(-8 * time.Minute),
	}}
	withoutTrace := computeReminderScoresWithTrace(sessionList, nil, now, nil, 5, nil)

	var traceLineList []string
	withTrace := computeReminderScoresWithTrace(sessionList, nil, now, nil, 5, func(format string, args ...any) {
		traceLineList = append(traceLineList, fmt.Sprintf(format, args...))
	})

	if !reflect.DeepEqual(withoutTrace, withTrace) {
		t.Fatalf("scores differ when attention trace is enabled: without=%+v with=%+v", withoutTrace, withTrace)
	}
	if len(traceLineList) != 4 {
		t.Fatalf("trace lines=%d, want 4: %v", len(traceLineList), traceLineList)
	}
}

func TestPutScheduleRequiresConfirmationBeforeReplacingRecordedItem(t *testing.T) {
	server := NewServer(nil, nil)
	server.scheduleMgr = schedule.NewManagerAt(filepath.Join(t.TempDir(), "schedules.json"))
	completedAt := time.Date(2026, 9, 11, 9, 50, 0, 0, time.UTC)
	file := schedule.File{Days: map[string]schedule.Day{"2026-09-11": {
		Date:  "2026-09-11",
		Items: []schedule.Item{{ID: "a", Start: "09:00", Title: "old", FocusMinutes: 50, Status: "completed", CompletedAt: completedAt}},
	}}}
	if err := server.scheduleMgr.Save(file); err != nil {
		t.Fatal(err)
	}

	body, err := json.Marshal(map[string]any{"date": "2026-09-11", "items": []map[string]any{{
		"id": "a", "start": "09:00", "title": "renamed", "focus_minutes": 50, "break_minutes": 0,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPut, "/api/v1/schedules", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	server.handlePutSchedule(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s, want %d", recorder.Code, recorder.Body.String(), http.StatusConflict)
	}

	confirmedBody, err := json.Marshal(map[string]any{"date": "2026-09-11", "replace_recorded": true, "items": []map[string]any{{
		"id": "a", "start": "09:00", "title": "renamed", "focus_minutes": 50, "break_minutes": 0,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodPut, "/api/v1/schedules", bytes.NewReader(confirmedBody))
	recorder = httptest.NewRecorder()
	server.handlePutSchedule(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want %d", recorder.Code, recorder.Body.String(), http.StatusOK)
	}
}

func TestPutTemplateReturnsSnapshotWithoutExecutionRecord(t *testing.T) {
	server := NewServer(nil, nil)
	server.scheduleMgr = schedule.NewManagerAt(filepath.Join(t.TempDir(), "schedules.json"))
	body, err := json.Marshal(map[string]any{"name": "weekday", "items": []map[string]any{{
		"id": "a", "start": "09:00", "title": "plan", "focus_minutes": 50, "break_minutes": 10, "status": "completed",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPut, "/api/v1/schedule-templates", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	server.handlePutTemplate(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response schedule.Template
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Items[0].Status != "" || !response.Items[0].CompletedAt.IsZero() {
		t.Fatalf("template response retained execution data: %+v", response.Items[0])
	}
}

func TestPutTemplateRequiresConfirmationBeforeReplacingSameName(t *testing.T) {
	server := NewServer(nil, nil)
	server.scheduleMgr = schedule.NewManagerAt(filepath.Join(t.TempDir(), "schedules.json"))
	if err := server.scheduleMgr.Save(schedule.File{Templates: []schedule.Template{{
		ID: "existing", Name: "weekday", Items: []schedule.Item{{
			ID: "old-item", Start: "09:00", Title: "old plan", FocusMinutes: 50, BreakMinutes: 10,
		}},
	}}}); err != nil {
		t.Fatal(err)
	}

	newItems := []map[string]any{{
		"id": "new-item", "start": "10:00", "title": "new plan", "focus_minutes": 60, "break_minutes": 0,
	}}
	requestBody, err := json.Marshal(map[string]any{"name": "weekday", "items": newItems})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPut, "/api/v1/schedule-templates", bytes.NewReader(requestBody))
	recorder := httptest.NewRecorder()
	server.handlePutTemplate(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s, want %d", recorder.Code, recorder.Body.String(), http.StatusConflict)
	}

	requestBody, err = json.Marshal(map[string]any{"name": "weekday", "items": newItems, "replace_existing": true})
	if err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodPut, "/api/v1/schedule-templates", bytes.NewReader(requestBody))
	recorder = httptest.NewRecorder()
	server.handlePutTemplate(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	file, err := server.scheduleMgr.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Templates) != 1 || file.Templates[0].ID != "existing" || file.Templates[0].Items[0].Title != "new plan" {
		t.Fatalf("templates=%+v, want one replaced template", file.Templates)
	}
}

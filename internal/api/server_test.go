package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/pancake-lee/pflow/internal/config"
	"github.com/pancake-lee/pflow/internal/schedule"
	"github.com/pancake-lee/pflow/internal/settings"
)

func TestSettingsAPIExposesDefaultsAndSavedRevision(t *testing.T) {
	server := NewServer(nil, nil)
	server.settingsMgr = settings.NewManagerAt(filepath.Join(t.TempDir(), "settings.json"))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil))
	var body struct {
		settings.File
		Defaults settings.File `json:"defaults"`
	}
	err := json.Unmarshal(response.Body.Bytes(), &body)
	if err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || body.Defaults.Dashboard.RefreshSeconds != 30 {
		t.Fatalf("body=%s", response.Body.String())
	}
	value := body.File
	value.Dashboard.Window = "7d"
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/api/v1/settings", bytes.NewReader(data)))
	var saved settings.File
	err = json.Unmarshal(response.Body.Bytes(), &saved)
	if err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || saved.DashboardRevision == "" || saved.Dashboard.Window != "7d" {
		t.Fatalf("save=%s", response.Body.String())
	}
}

func TestDashboardFirstRequestUsesOnlyMatchingCachedResult(t *testing.T) {
	server := NewServer(nil, nil)
	server.settingsMgr = settings.NewManagerAt(filepath.Join(t.TempDir(), "settings.json"))
	server.dashboardCacheKey = dashboardCacheKey{window: 24 * time.Hour, maxActive: 1, maxInactive: 1, source: config.DefaultHermesSourceFilter}
	server.dashboardCache = &DashboardResponse{Now: time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC), Window: "1d"}

	hit := httptest.NewRecorder()
	server.ServeHTTP(hit, httptest.NewRequest(http.MethodGet, "/api/v1/dashboard?first=1&window=1d&max_active=1&max_inactive=1", nil))
	if hit.Code != http.StatusOK || hit.Header().Get("X-Pflow-Cache") != "hit" {
		t.Fatalf("cache hit: status=%d header=%q body=%s", hit.Code, hit.Header().Get("X-Pflow-Cache"), hit.Body.String())
	}
	var cached DashboardResponse
	if err := json.Unmarshal(hit.Body.Bytes(), &cached); err != nil || !cached.Now.Equal(server.dashboardCache.Now) {
		t.Fatalf("cached response=%+v err=%v", cached, err)
	}

	miss := httptest.NewRecorder()
	server.ServeHTTP(miss, httptest.NewRequest(http.MethodGet, "/api/v1/dashboard?first=1&window=7d&max_active=1&max_inactive=1", nil))
	if miss.Header().Get("X-Pflow-Cache") == "hit" {
		t.Fatal("different scan window returned cached data")
	}
}

func TestTemplateRenameCannotBypassNameConflictWithAnExistingID(t *testing.T) {
	server := NewServer(nil, nil)
	server.scheduleMgr = schedule.NewManagerAt(filepath.Join(t.TempDir(), "schedules.json"))
	file := schedule.File{Templates: []schedule.Template{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}}}
	err := server.scheduleMgr.Save(file)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPut, "/api/v1/schedule-templates", bytes.NewBufferString(`{"id":"a","name":"B","items":[]}`))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	reloaded, err := server.scheduleMgr.Load()
	if err != nil || reloaded.Templates[0].Name != "A" {
		t.Fatal("conflict mutated the other template")
	}
	request = httptest.NewRequest(http.MethodPut, "/api/v1/schedule-templates", bytes.NewBufferString(`{"id":"a","name":"B","items":[],"replace_existing":true}`))
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d", response.Code)
	}
	var saved schedule.Template
	err = json.Unmarshal(response.Body.Bytes(), &saved)
	if err != nil || saved.ID != "b" {
		t.Fatalf("saved=%+v err=%v", saved, err)
	}
	reloaded, err = server.scheduleMgr.Load()
	if err != nil || len(reloaded.Templates) != 2 || reloaded.Templates[0].Name != "A" {
		t.Fatal("confirmed overwrite removed or renamed unrelated template")
	}
}

func TestScheduleActionPersistsOnceAndRejectsDistantCompletion(t *testing.T) {
	server := NewServer(nil, nil)
	server.scheduleMgr = schedule.NewManagerAt(filepath.Join(t.TempDir(), "schedules.json"))
	day := schedule.Day{Date: "2026-10-25", CurrentID: "a", Items: []schedule.Item{
		{ID: "a", Start: "09:00", Title: "A", FocusMinutes: 30},
		{ID: "b", Start: "10:00", Title: "B", FocusMinutes: 30},
		{ID: "c", Start: "11:00", Title: "C", FocusMinutes: 30},
	}}
	file := schedule.File{Days: map[string]schedule.Day{day.Date: day}}
	if err := server.scheduleMgr.Save(file); err != nil {
		t.Fatal(err)
	}
	requestBody := `{"date":"2026-10-25","id":"c","action":"completed"}`
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(
		http.MethodPost, "/api/v1/schedules/action", bytes.NewBufferString(requestBody),
	))
	if response.Code != http.StatusConflict {
		t.Fatalf("distant status=%d body=%s", response.Code, response.Body.String())
	}
	requestBody = `{"date":"2026-10-25","id":"a","action":"completed"}`
	for i := 0; i < 2; i++ {
		response = httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(
			http.MethodPost, "/api/v1/schedules/action", bytes.NewBufferString(requestBody),
		))
		if response.Code != http.StatusOK {
			t.Fatalf("completion status=%d body=%s", response.Code, response.Body.String())
		}
	}
	loaded, err := server.scheduleMgr.Load()
	if err != nil {
		t.Fatal(err)
	}
	saved := loaded.Days[day.Date]
	if len(saved.Events) != 1 || schedule.Score(saved.Events) != 10 || saved.CurrentID != "b" {
		t.Fatalf("saved=%+v", saved)
	}
}

func TestActiveLeaveRemainsVisibleAfterScheduleDateChanges(t *testing.T) {
	server := NewServer(nil, nil)
	server.scheduleMgr = schedule.NewManagerAt(filepath.Join(t.TempDir(), "schedules.json"))
	today := time.Now().Format("2006-01-02")
	past := time.Now().AddDate(0, 0, -2).Format("2006-01-02")
	file := schedule.File{Days: map[string]schedule.Day{
		past: {
			Date: past, CurrentID: "a",
			Items:  []schedule.Item{{ID: "a", Start: "09:00", Title: "A", FocusMinutes: 30}},
			Events: []schedule.Event{{ID: "leave-1", TaskID: "a", Action: "leave", Note: "返回线索", Time: time.Now().AddDate(0, 0, -2)}},
		},
		today: {Date: today, Items: []schedule.Item{}},
	}}
	if err := server.scheduleMgr.Save(file); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/schedules", nil))
	var got struct {
		Day         schedule.Day    `json:"day"`
		ActiveLeave *schedule.Event `json:"active_leave"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil ||
		response.Code != http.StatusOK || got.Day.Date != past || got.ActiveLeave == nil {
		t.Fatalf("active leave=%s err=%v", response.Body.String(), err)
	}
	response = httptest.NewRecorder()
	requestBody := fmt.Sprintf(`{"date":"%s","id":"a","action":"return"}`, past)
	server.ServeHTTP(response, httptest.NewRequest(
		http.MethodPost, "/api/v1/schedules/action", bytes.NewBufferString(requestBody),
	))
	if response.Code != http.StatusOK {
		t.Fatalf("return status=%d body=%s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/schedules", nil))
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil ||
		got.Day.Date != today || got.ActiveLeave != nil {
		t.Fatalf("after return=%s err=%v", response.Body.String(), err)
	}
}

func TestScheduleReviewNotePersistsByDate(t *testing.T) {
	server := NewServer(nil, nil)
	server.scheduleMgr = schedule.NewManagerAt(filepath.Join(t.TempDir(), "schedules.json"))
	day := schedule.Day{Date: "2026-10-25", Items: []schedule.Item{{
		ID: "a", Start: "09:00", Title: "A", FocusMinutes: 30,
	}}, ReviewNote: "旧备注"}
	if err := server.scheduleMgr.Save(schedule.File{Days: map[string]schedule.Day{day.Date: day}}); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(
		http.MethodGet, "/api/v1/schedules/review?date=2026-10-25", nil,
	))
	var review struct {
		Note string `json:"note"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &review); err != nil || review.Note != "旧备注" {
		t.Fatalf("legacy review=%s err=%v", response.Body.String(), err)
	}

	// A date without a schedule can still hold its own review note.
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(
		http.MethodPut, "/api/v1/schedules/review",
		bytes.NewBufferString(`{"date":"2026-10-24","note":"  明天提前开始  "}`),
	))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	loaded, err := server.scheduleMgr.Load()
	if err != nil || loaded.ReviewNotes["2026-10-24"] != "明天提前开始" || loaded.Days[day.Date].ReviewNote != "旧备注" {
		t.Fatalf("loaded=%+v err=%v", loaded, err)
	}
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(
		http.MethodGet, "/api/v1/schedules/review?date=2026-10-24", nil,
	))
	var saved struct {
		Note   string                 `json:"note"`
		Review schedule.ReviewSummary `json:"review"`
		Events []schedule.Event       `json:"events"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &saved); err != nil ||
		response.Code != http.StatusOK || saved.Note != "明天提前开始" ||
		saved.Review.Score != 0 || len(saved.Events) != 0 {
		t.Fatalf("saved review=%s err=%v", response.Body.String(), err)
	}
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(
		http.MethodPut, "/api/v1/schedules/review",
		bytes.NewBufferString(`{"date":"2026-10-25","note":"新备注"}`),
	))
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(
		http.MethodGet, "/api/v1/schedules/review?date=2026-10-25", nil,
	))
	if err := json.Unmarshal(response.Body.Bytes(), &review); err != nil || review.Note != "新备注" {
		t.Fatalf("updated review=%s err=%v", response.Body.String(), err)
	}
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(
		http.MethodPut, "/api/v1/schedules/review",
		bytes.NewBufferString(`{"date":"2026-10-25","note":""}`),
	))
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(
		http.MethodGet, "/api/v1/schedules/review?date=2026-10-25", nil,
	))
	if err := json.Unmarshal(response.Body.Bytes(), &review); err != nil || review.Note != "" {
		t.Fatalf("cleared review=%s err=%v", response.Body.String(), err)
	}
}

func TestScheduleParkingPersistsWithoutAdvancing(t *testing.T) {
	server := NewServer(nil, nil)
	server.scheduleMgr = schedule.NewManagerAt(filepath.Join(t.TempDir(), "schedules.json"))
	day := schedule.Day{Date: "2026-10-25", CurrentID: "a", Items: []schedule.Item{{
		ID: "a", Start: "09:00", Title: "A", FocusMinutes: 30,
	}}}
	if err := server.scheduleMgr.Save(schedule.File{Days: map[string]schedule.Day{day.Date: day}}); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(
		http.MethodPost, "/api/v1/schedules/parking",
		bytes.NewBufferString(`{"date":"2026-10-25","content":"发票"}`),
	))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	loaded, err := server.scheduleMgr.Load()
	if err != nil || loaded.Days[day.Date].CurrentID != "a" ||
		len(loaded.Days[day.Date].Parking) != 1 {
		t.Fatalf("loaded=%+v err=%v", loaded.Days[day.Date], err)
	}
	id := loaded.Days[day.Date].Parking[0].ID
	body := fmt.Sprintf(`{"date":"%s","id":"%s"}`, day.Date, id)
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(
		http.MethodPatch, "/api/v1/schedules/parking", bytes.NewBufferString(body),
	))
	if response.Code != http.StatusOK {
		t.Fatalf("process status=%d body=%s", response.Code, response.Body.String())
	}
	loaded, err = server.scheduleMgr.Load()
	if err != nil || loaded.Days[day.Date].Parking[0].ProcessedAt.IsZero() {
		t.Fatalf("processed=%+v err=%v", loaded.Days[day.Date], err)
	}
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(
		http.MethodDelete, "/api/v1/schedules/parking", bytes.NewBufferString(body),
	))
	if response.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", response.Code, response.Body.String())
	}
	loaded, err = server.scheduleMgr.Load()
	if err != nil || loaded.Days[day.Date].CurrentID != "a" || len(loaded.Days[day.Date].Parking) != 0 ||
		len(loaded.Days[day.Date].Events) != 2 {
		t.Fatalf("deleted=%+v err=%v", loaded.Days[day.Date], err)
	}
}

func TestConcurrentScheduleAndTemplateUpdatesDoNotOverwrite(t *testing.T) {
	server := NewServer(nil, nil)
	server.scheduleMgr = schedule.NewManagerAt(filepath.Join(t.TempDir(), "schedules.json"))
	var workers sync.WaitGroup
	start := make(chan struct{})
	for i := 1; i <= 24; i++ {
		for _, kind := range []string{"day", "template"} {
			workers.Add(1)
			go func(i int, kind string) {
				defer workers.Done()
				<-start
				body := fmt.Sprintf(`{"date":"2026-10-%02d","items":[]}`, i)
				url := "/api/v1/schedules"
				if kind == "template" {
					body = fmt.Sprintf(`{"name":"template-%d","items":[]}`, i)
					url = "/api/v1/schedule-templates"
				}
				r := httptest.NewRequest(http.MethodPut, url, bytes.NewBufferString(body))
				w := httptest.NewRecorder()
				server.ServeHTTP(w, r)
				if w.Code != http.StatusOK {
					t.Errorf("%s status=%d body=%s", kind, w.Code, w.Body.String())
				}
			}(i, kind)
		}
	}
	close(start)
	workers.Wait()
	file, err := server.scheduleMgr.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Days) != 24 || len(file.Templates) != 24 {
		t.Fatalf("days=%d templates=%d", len(file.Days), len(file.Templates))
	}
}

func TestScheduleStorageFailureReturnsServerError(t *testing.T) {
	server := NewServer(nil, nil)
	server.scheduleMgr = schedule.NewManagerAt(t.TempDir())
	request := httptest.NewRequest(http.MethodPut, "/api/v1/schedules", bytes.NewBufferString(`{"date":"2026-10-13","items":[]}`))
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

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

func TestCustomKnowledgeSettingsAndDashboard(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	server := NewServer(nil, nil)
	value := settings.Default()
	value.CustomKnowledgeTips = []settings.CustomKnowledgeTip{
		{ID: "custom_api", Title: "知识", Theory: "<b>按文本显示</b>"},
	}
	request := func(method, path string, value any) *httptest.ResponseRecorder {
		t.Helper()
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(method, path, bytes.NewReader(data)))
		return response
	}
	saved := request(http.MethodPut, "/api/v1/settings", value)
	if saved.Code != http.StatusOK {
		t.Fatalf("save: %s", saved.Body.String())
	}
	// Both endpoints expose exactly the saved content and identity.
	for _, path := range []string{"/api/v1/settings", "/api/v1/dashboard"} {
		response := request(http.MethodGet, path, nil)
		var body struct {
			Tips []settings.CustomKnowledgeTip `json:"custom_knowledge_tips"`
		}
		err := json.Unmarshal(response.Body.Bytes(), &body)
		if response.Code != http.StatusOK || err != nil || !reflect.DeepEqual(body.Tips, value.CustomKnowledgeTips) {
			t.Fatalf("%s: %d %s, %v", path, response.Code, response.Body.String(), err)
		}
	}
	invalid := value
	invalid.CustomKnowledgeTips = []settings.CustomKnowledgeTip{{ID: "custom_invalid", Title: "", Theory: "body"}}
	rejected := request(http.MethodPut, "/api/v1/settings", invalid)
	if rejected.Code != http.StatusBadRequest {
		t.Fatalf("invalid accepted: %s", rejected.Body.String())
	}
	reloaded, err := server.settingsMgr.Load()
	if err != nil || !reflect.DeepEqual(reloaded.CustomKnowledgeTips, value.CustomKnowledgeTips) {
		t.Fatal("invalid request overwrote settings")
	}
	value.CustomKnowledgeTips = []settings.CustomKnowledgeTip{}
	deleted := request(http.MethodPut, "/api/v1/settings", value)
	if deleted.Code != http.StatusOK {
		t.Fatalf("delete: %s", deleted.Body.String())
	}
	response := request(http.MethodGet, "/api/v1/dashboard", nil)
	var body DashboardResponse
	err = json.Unmarshal(response.Body.Bytes(), &body)
	if err != nil || len(body.CustomKnowledgeTips) != 0 {
		t.Fatalf("deleted knowledge still present: %s", response.Body.String())
	}
}

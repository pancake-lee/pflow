package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/pancake-lee/pflow/internal/config"
)

// TestReadSessionMetasDuplicateID verifies that when the same session is
// resumed in a new process (claude -r), leaving two metadata files with one
// sessionId, the most recently updated metadata wins — a stale "waiting"
// from an abandoned old process must not shadow the live session's state.
func TestReadSessionMetasDuplicateID(t *testing.T) {
	dir := t.TempDir()
	sessions := filepath.Join(dir, "sessions")
	if err := os.Mkdir(sessions, 0o755); err != nil {
		t.Fatal(err)
	}

	sid := "20aa3620-1895-4bf8-84c3-11a676140f26"
	files := []SessionMeta{
		// Old process: stuck at a permission prompt since yesterday.
		{PID: 944733, SessionID: sid, CWD: "/root/code/alanz-site",
			Status: "waiting", UpdatedAt: 1788253166168},
		// New process: resumed the same session, now idle.
		{PID: 1230917, SessionID: sid, CWD: "/root/code/alanz-site",
			Status: "idle", UpdatedAt: 1788339973559},
	}
	for _, m := range files {
		data, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Join(sessions, strconv.Itoa(m.PID)+".json")
		if err := os.WriteFile(name, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	metas := readSessionMetas(dir)
	got, ok := metas[sid]
	if !ok {
		t.Fatal("session metadata not found")
	}
	if got.Status != "idle" || got.PID != 1230917 {
		t.Errorf("got status=%s pid=%d, want status=idle pid=1230917", got.Status, got.PID)
	}
}

// writeClaudeFixture builds a minimal ~/.claude layout for scanDir tests:
// history entries, session metadata and transcript files.
func writeClaudeFixture(t *testing.T) (dir, recentSID, oldSID, missingSID string, now time.Time) {
	t.Helper()
	dir = t.TempDir()
	now = time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	recentSID = "aaaaaaaa-0000-0000-0000-000000000001"
	oldSID = "bbbbbbbb-0000-0000-0000-000000000002"
	missingSID = "cccccccc-0000-0000-0000-000000000003"

	// history.jsonl: one in-window and two out-of-window sessions.
	history := []HistoryEntry{
		{Display: "fix login bug", Timestamp: now.Add(-2 * time.Hour).UnixMilli(), Project: "/work/pflow", SessionID: recentSID},
		{Display: "old work", Timestamp: now.Add(-72 * time.Hour).UnixMilli(), Project: "/work/pflow", SessionID: oldSID},
	}
	var lines []byte
	for _, h := range history {
		data, err := json.Marshal(h)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, data...)
		lines = append(lines, '\n')
	}
	if err := os.WriteFile(filepath.Join(dir, "history.jsonl"), lines, 0o644); err != nil {
		t.Fatal(err)
	}

	// projects/<project>/<session-id>.jsonl transcripts for recent and old.
	projDir := filepath.Join(dir, "projects", "-work-pflow")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTranscript := func(sid, userText, assistantText string, modTime time.Time) {
		t.Helper()
		events := []string{
			`{"type":"user","message":{"role":"user","content":"` + userText + `"}}`,
			`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"` + assistantText + `"}]}}`,
		}
		path := filepath.Join(projDir, sid+".jsonl")
		body := []byte(events[0] + "\n" + events[1] + "\n")
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			t.Fatal(err)
		}
	}
	writeTranscript(recentSID, "recent question", "recent answer", now.Add(-2*time.Hour))
	writeTranscript(oldSID, "old question", "old answer", now.Add(-96*time.Hour))
	return dir, recentSID, oldSID, missingSID, now
}

// TestScanDirOnlyKeepsWindowSessionsAndEnrichesTranscripts verifies the
// aggregation → limit → transcript ordering: in-window sessions are enriched
// from transcripts, out-of-window ones never appear.
func TestScanDirOnlyKeepsWindowSessionsAndEnrichesTranscripts(t *testing.T) {
	dir, recentSID, oldSID, _, now := writeClaudeFixture(t)
	opts := config.ScanOptions{Window: 24 * time.Hour, MaxActive: config.NoSessionLimit, MaxInactive: config.NoSessionLimit}
	res, err := scanDir(dir, opts, now)
	if err != nil {
		t.Fatal(err)
	}

	var recent, old *SessionSummary
	for i := range res.Sessions {
		switch res.Sessions[i].SessionID {
		case recentSID:
			recent = &res.Sessions[i]
		case oldSID:
			old = &res.Sessions[i]
		}
	}
	if recent == nil {
		t.Fatal("in-window session missing")
	}
	if recent.LastReq != "recent question" || recent.LastResp != "recent answer" {
		t.Fatalf("recent not enriched: %+v", recent)
	}
	// The old session is revived by its metadata heartbeat but its history
	// and transcript predate the window; it must not show up with a 24h window.
	if old != nil {
		t.Fatalf("out-of-window session leaked: %+v", old)
	}
}

// TestScanDirEnrichesWantedSessionWithStaleTranscript verifies that a session
// kept alive by a fresh metadata heartbeat (waiting process without new
// messages) is still enriched even when its transcript file has not been
// written since long before the window: transcript selection follows the
// wanted session IDs, not file mtime.
func TestScanDirEnrichesWantedSessionWithStaleTranscript(t *testing.T) {
	dir, _, oldSID, _, now := writeClaudeFixture(t)
	if err := os.Mkdir(filepath.Join(dir, "sessions"), 0o755); err != nil {
		t.Fatal(err)
	}
	meta := SessionMeta{PID: 42, SessionID: oldSID, CWD: "/work/pflow", Status: "waiting",
		StartedAt: now.Add(-96 * time.Hour).UnixMilli(), UpdatedAt: now.Add(-1 * time.Hour).UnixMilli()}
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sessions", "42.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	opts := config.ScanOptions{Window: 24 * time.Hour, MaxActive: config.NoSessionLimit, MaxInactive: config.NoSessionLimit}
	res, err := scanDir(dir, opts, now)
	if err != nil {
		t.Fatal(err)
	}
	var old *SessionSummary
	for i := range res.Sessions {
		if res.Sessions[i].SessionID == oldSID {
			old = &res.Sessions[i]
		}
	}
	if old == nil {
		t.Fatal("heartbeat-revived session missing")
	}
	if old.LastReq != "old question" || old.LastResp != "old answer" {
		t.Fatalf("stale-mtime transcript not enriched: %+v", old)
	}
}

// TestScanDirMissingTranscriptIsNotAnError verifies that a wanted session
// without any transcript file scans fine and simply lacks request/response
// summaries.
func TestScanDirMissingTranscriptIsNotAnError(t *testing.T) {
	dir, _, _, missingSID, now := writeClaudeFixture(t)
	// Put the missing session in-window via history.
	histPath := filepath.Join(dir, "history.jsonl")
	entry := HistoryEntry{Display: "no transcript", Timestamp: now.Add(-1 * time.Hour).UnixMilli(), Project: "/work/pflow", SessionID: missingSID}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(histPath, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := config.ScanOptions{Window: 24 * time.Hour, MaxActive: config.NoSessionLimit, MaxInactive: config.NoSessionLimit}
	res, err := scanDir(dir, opts, now)
	if err != nil {
		t.Fatal(err)
	}
	var missing *SessionSummary
	for i := range res.Sessions {
		if res.Sessions[i].SessionID == missingSID {
			missing = &res.Sessions[i]
		}
	}
	if missing == nil {
		t.Fatal("session without transcript missing")
	}
	if missing.LastReq != "" || missing.LastResp != "" {
		t.Fatalf("unexpected enrichment: %+v", missing)
	}
}

// TestReadTranscriptsSkipsUnwantedFiles verifies that transcript files whose
// name is not a wanted session ID are skipped without being parsed.
func TestReadTranscriptsSkipsUnwantedFiles(t *testing.T) {
	dir, recentSID, oldSID, _, _ := writeClaudeFixture(t)
	result := readTranscripts(dir, map[string]bool{recentSID: true})
	if len(result) != 1 {
		t.Fatalf("result size=%d, want 1", len(result))
	}
	if _, ok := result[recentSID]; !ok {
		t.Fatalf("wanted transcript missing: %+v", result)
	}
	if _, ok := result[oldSID]; ok {
		t.Fatal("unwanted transcript was parsed")
	}
}

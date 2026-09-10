package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
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

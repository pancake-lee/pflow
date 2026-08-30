package codex

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pancake-lee/pflow/internal/config"
)

func TestScanDirAggregatesRollout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "2026", "08", "27")
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
	data := `{"timestamp":"2026-08-27T10:00:00Z","type":"session_meta","payload":{"session_id":"abc","cwd":"/work/pflow","timestamp":"2026-08-27T10:00:00Z"}}
{"timestamp":"2026-08-27T10:01:00Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"implement codex support"}]}}
{"timestamp":"2026-08-27T10:01:30Z","type":"event_msg","payload":{"type":"user_message","message":"Give Codex sessions readable names"}}
{"timestamp":"2026-08-27T10:02:00Z","type":"event_msg","payload":{"type":"task_started"}}
{"timestamp":"2026-08-27T10:03:00Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"done"}]}}
`
	if err := os.WriteFile(filepath.Join(path, "rollout-test.jsonl"), []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	r, err := ScanDir(dir, config.ScanOptions{Window: time.Hour}, time.Date(2026, 8, 27, 10, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Sessions) != 1 {
		t.Fatalf("sessions=%d", len(r.Sessions))
	}
	s := r.Sessions[0]
	if s.SessionID != "abc" || s.Project != "/work/pflow" || s.Name != "implement codex support" || s.Status != "idle" || s.MessageCount != 1 {
		t.Fatalf("unexpected summary: %+v", s)
	}
	if s.LastReq != "implement codex support" || s.LastResp != "done" {
		t.Fatalf("unexpected summaries: %+v", s)
	}
}

func TestScanDirIgnoresMalformedLineAndOldSession(t *testing.T) {
	dir := t.TempDir()
	data := "not json\n" + `{"timestamp":"2026-08-27T01:00:00Z","type":"session_meta","payload":{"session_id":"old","cwd":"/old"}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "rollout-test.jsonl"), []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	r, err := ScanDir(dir, config.ScanOptions{Window: time.Hour}, time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Sessions) != 0 || len(r.Diagnostics) == 0 {
		t.Fatalf("sessions=%d diagnostics=%v", len(r.Sessions), r.Diagnostics)
	}
}

func TestScanDirInfersBusyAndUnknown(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	data := `{"timestamp":"2026-08-27T09:58:00Z","type":"session_meta","payload":{"session_id":"busy","cwd":"/work"}}
{"timestamp":"2026-08-27T09:59:00Z","type":"event_msg","payload":{"type":"task_started"}}
`
	unknown := `{"timestamp":"2026-08-27T09:58:00Z","type":"session_meta","payload":{"session_id":"unknown","cwd":"/other"}}
{"timestamp":"2026-08-27T09:59:00Z","type":"turn_context","payload":{}}
`
	if err := os.WriteFile(filepath.Join(dir, "rollout-busy.jsonl"), []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rollout-unknown.jsonl"), []byte(unknown), 0644); err != nil {
		t.Fatal(err)
	}
	r, err := ScanDir(dir, config.ScanOptions{Window: time.Hour}, now)
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]string{}
	for _, s := range r.Sessions {
		states[s.SessionID] = s.Status
	}
	if states["busy"] != "busy" || states["unknown"] != "unknown" {
		t.Fatalf("states=%v", states)
	}
}

func TestSessionNameStripsInjectedInstructions(t *testing.T) {
	message := "Plan Codex name extraction\n# AGENTS.md instructions for /work/pflow\n..."
	if got := sessionName(message); got != "Plan Codex name extraction" {
		t.Fatalf("name=%q", got)
	}
}

func TestScanDirUsesFirstRealUserMessageAsStableName(t *testing.T) {
	dir := t.TempDir()
	data := `{"timestamp":"2026-08-27T10:00:00Z","type":"session_meta","payload":{"session_id":"stable","cwd":"/work/pflow"}}
{"timestamp":"2026-08-27T10:01:00Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"The following instructions are provided by the environment."},{"type":"input_text","text":"Implement readable Codex session titles"}]}}
{"timestamp":"2026-08-27T10:01:01Z","type":"event_msg","payload":{"type":"user_message","message":"Implement readable Codex session titles\n# AGENTS.md instructions for /work/pflow\n..."}}
{"timestamp":"2026-08-27T10:02:00Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Now add regression coverage"}]}}
`
	if err := os.WriteFile(filepath.Join(dir, "rollout-stable.jsonl"), []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	r, err := ScanDir(dir, config.ScanOptions{Window: time.Hour}, time.Date(2026, 8, 27, 10, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Sessions) != 1 {
		t.Fatalf("sessions=%d", len(r.Sessions))
	}
	s := r.Sessions[0]
	if s.Name != "Implement readable Codex session titles" {
		t.Fatalf("name=%q", s.Name)
	}
	if s.LastReq != "Now add regression coverage" {
		t.Fatalf("last request=%q", s.LastReq)
	}
}

func TestScanDirFallsBackWhenUserInputIsOnlyInjectedContext(t *testing.T) {
	dir := t.TempDir()
	data := `{"timestamp":"2026-08-27T10:00:00Z","type":"session_meta","payload":{"session_id":"fallback","cwd":"/work/pflow"}}
{"timestamp":"2026-08-27T10:01:00Z","type":"event_msg","payload":{"type":"user_message","message":"<environment_context>\n..."}}
{"timestamp":"2026-08-27T10:02:00Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"The following instructions are provided by the environment."}]}}
`
	if err := os.WriteFile(filepath.Join(dir, "rollout-fallback.jsonl"), []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	r, err := ScanDir(dir, config.ScanOptions{Window: time.Hour}, time.Date(2026, 8, 27, 10, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Sessions) != 1 {
		t.Fatalf("sessions=%d", len(r.Sessions))
	}
	if got := r.Sessions[0].Name; got != "pflow" {
		t.Fatalf("name=%q", got)
	}
}

func TestProjectBaseName(t *testing.T) {
	if got := projectBaseName("/work/pflow"); got != "pflow" {
		t.Fatalf("project name=%q", got)
	}
	if got := projectBaseName(""); got != "Codex session" {
		t.Fatalf("empty project name=%q", got)
	}
}

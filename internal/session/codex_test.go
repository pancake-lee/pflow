package session

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/pancake-lee/pflow/internal/codex"
)

func TestPendingCodexMappingSurvivesUntilEffectiveMessage(t *testing.T) {
	base := time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC)
	manager := &mappingManager{path: filepath.Join(t.TempDir(), "mappings.json")}
	store := &mappingStore{Version: 1, Mappings: []Mapping{{
		TmuxName: "pflow-test", WorkDir: "/work", AgentType: "codex",
		Status: "pending", CreatedAt: base,
	}}}
	err := manager.save(store)
	if err != nil {
		t.Fatal(err)
	}
	list := []codex.SessionSummary{{SessionID: "new", Project: "/work", LastActive: base.Add(time.Minute)}}
	updated, err := manager.syncPendingCodexMappings(list)
	if err != nil || updated != 0 {
		t.Fatalf("premature mapping=%d err=%v", updated, err)
	}
	list[0].FirstUserMessageAt = base.Add(time.Minute)
	updated, err = manager.syncPendingCodexMappings(list)
	if err != nil || updated != 1 {
		t.Fatalf("mapping=%d err=%v", updated, err)
	}
	reloaded, err := manager.load()
	if err != nil {
		t.Fatal(err)
	}
	mapping := reloaded.Mappings[0]
	if mapping.AgentSessionID != "new" || mapping.Status != "active" || !mapping.CreatedAt.Equal(base) {
		t.Fatalf("mapping=%+v", mapping)
	}
	updated, err = manager.syncPendingCodexMappings([]codex.SessionSummary{{SessionID: "other", Project: "/work", FirstUserMessageAt: base.Add(2 * time.Minute)}})
	if err != nil || updated != 0 {
		t.Fatal("existing mapping changed")
	}
}

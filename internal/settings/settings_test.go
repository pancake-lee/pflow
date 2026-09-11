package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManagerDefaultsAndRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	m := NewManagerAt(path)
	got, err := m.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Dashboard.Window != "1d" || got.TimeEstimate.MinutesPerMessage != 3 {
		t.Fatalf("defaults=%+v", got)
	}
	got.Dashboard.RefreshSeconds = 60
	if _, err := m.Update(got); err != nil {
		t.Fatal(err)
	}
	reloaded, err := m.Load()
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Dashboard.RefreshSeconds != 60 {
		t.Fatalf("refresh=%d", reloaded.Dashboard.RefreshSeconds)
	}
}

func TestManagerRejectsInvalidAndUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"dashboard":{"window":"1d","max_inactive":1,"refresh_seconds":30,"daily_boot_enabled":true},"attention":{"protect_minutes":5,"focus_add_minutes":15,"mask_strength":1},"time_estimate":{"minutes_per_message":3,"fallback_ratio":0.3},"future":true}`), 0644); err != nil {
		t.Fatal(err)
	}
	m := NewManagerAt(path)
	if _, err := m.Load(); err != nil {
		t.Fatal(err)
	}
	bad := Default()
	bad.Dashboard.RefreshSeconds = 17
	if _, err := m.Update(bad); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestMaxActiveCoercedToMinimum(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	// Legacy file where max_active=0 meant "no limit": load must normalize to 1.
	if err := os.WriteFile(path, []byte(`{"version":1,"dashboard":{"window":"1d","max_active":0,"max_inactive":0,"refresh_seconds":30,"daily_boot_enabled":true},"attention":{"protect_minutes":5,"focus_add_minutes":15,"mask_strength":1},"time_estimate":{"minutes_per_message":3,"fallback_ratio":0.3}}`), 0644); err != nil {
		t.Fatal(err)
	}
	m := NewManagerAt(path)
	got, err := m.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Dashboard.MaxActive != 1 {
		t.Fatalf("legacy max_active=0 loaded as %d, want 1", got.Dashboard.MaxActive)
	}
	if got.Dashboard.MaxInactive != 0 {
		t.Fatalf("max_inactive=0 must stay 0 (hide), got %d", got.Dashboard.MaxInactive)
	}

	// Saving 0 (stale client) persists 1, so a reload keeps showing 1.
	toSave := Default()
	toSave.Dashboard.MaxActive = 0
	toSave.Dashboard.MaxInactive = 2
	saved, err := m.Update(toSave)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Dashboard.MaxActive != 1 {
		t.Fatalf("saved max_active=%d, want 1", saved.Dashboard.MaxActive)
	}
	reloaded, err := m.Load()
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Dashboard.MaxActive != 1 {
		t.Fatalf("reloaded max_active=%d, want 1", reloaded.Dashboard.MaxActive)
	}
}

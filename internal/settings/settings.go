// Package settings stores device-level pflow preferences.
package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const Version = 1

type Dashboard struct {
	Window           string `json:"window"`
	MaxActive        int    `json:"max_active"`
	MaxInactive      int    `json:"max_inactive"`
	RefreshSeconds   int    `json:"refresh_seconds"`
	DailyBootEnabled bool   `json:"daily_boot_enabled"`
}

type Attention struct {
	ProtectMinutes  float64 `json:"protect_minutes"`
	FocusAddMinutes float64 `json:"focus_add_minutes"`
	MaskStrength    float64 `json:"mask_strength"`
}

type TimeEstimate struct {
	MinutesPerMessage float64 `json:"minutes_per_message"`
	FallbackRatio     float64 `json:"fallback_ratio"`
}

type File struct {
	Version      int          `json:"version"`
	Dashboard    Dashboard    `json:"dashboard"`
	Attention    Attention    `json:"attention"`
	TimeEstimate TimeEstimate `json:"time_estimate"`
}

func Default() File {
	return File{Version: Version, Dashboard: Dashboard{Window: "1d", MaxActive: 0, MaxInactive: 1, RefreshSeconds: 30, DailyBootEnabled: true}, Attention: Attention{ProtectMinutes: 5, FocusAddMinutes: 15, MaskStrength: 1}, TimeEstimate: TimeEstimate{MinutesPerMessage: 3, FallbackRatio: .3}}
}

type Manager struct {
	mu   sync.Mutex
	path string
}

func NewManager() *Manager {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	return NewManagerAt(filepath.Join(home, ".pflow", "settings.json"))
}

func NewManagerAt(path string) *Manager { return &Manager{path: path} }

func (m *Manager) Load() (File, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.loadLocked()
}

func (m *Manager) loadLocked() (File, error) {
	defaults := Default()
	data, err := os.ReadFile(m.path)
	if os.IsNotExist(err) {
		return defaults, nil
	}
	if err != nil {
		return File{}, fmt.Errorf("read settings: %w", err)
	}
	var stored File
	if err := json.Unmarshal(data, &stored); err != nil {
		return File{}, fmt.Errorf("parse settings: %w", err)
	}
	merged := merge(defaults, stored)
	if err := validate(merged); err != nil {
		return File{}, err
	}
	return merged, nil
}

func merge(defaults, stored File) File {
	if stored.Version == 0 {
		return defaults
	}
	if stored.Dashboard.Window != "" {
		defaults.Dashboard.Window = stored.Dashboard.Window
	}
	if stored.Dashboard.MaxInactive >= 0 {
		defaults.Dashboard.MaxInactive = stored.Dashboard.MaxInactive
	}
	if stored.Dashboard.MaxActive >= 0 {
		defaults.Dashboard.MaxActive = stored.Dashboard.MaxActive
	}
	if stored.Dashboard.RefreshSeconds >= 0 {
		defaults.Dashboard.RefreshSeconds = stored.Dashboard.RefreshSeconds
	}
	defaults.Dashboard.DailyBootEnabled = stored.Dashboard.DailyBootEnabled
	if stored.Attention.ProtectMinutes > 0 {
		defaults.Attention.ProtectMinutes = stored.Attention.ProtectMinutes
	}
	if stored.Attention.FocusAddMinutes > 0 {
		defaults.Attention.FocusAddMinutes = stored.Attention.FocusAddMinutes
	}
	if stored.Attention.MaskStrength > 0 {
		defaults.Attention.MaskStrength = stored.Attention.MaskStrength
	}
	if stored.TimeEstimate.MinutesPerMessage > 0 {
		defaults.TimeEstimate.MinutesPerMessage = stored.TimeEstimate.MinutesPerMessage
	}
	if stored.TimeEstimate.FallbackRatio > 0 {
		defaults.TimeEstimate.FallbackRatio = stored.TimeEstimate.FallbackRatio
	}
	defaults.Version = Version
	return defaults
}

func validate(value File) error {
	if value.Dashboard.Window != "1h" && value.Dashboard.Window != "3h" && value.Dashboard.Window != "6h" && value.Dashboard.Window != "1d" && value.Dashboard.Window != "3d" && value.Dashboard.Window != "7d" {
		return fmt.Errorf("invalid dashboard.window")
	}
	if value.Dashboard.MaxInactive < 0 || value.Dashboard.MaxInactive > 10 {
		return fmt.Errorf("dashboard.max_inactive must be between 0 and 10")
	}
	if value.Dashboard.MaxActive < 0 || value.Dashboard.MaxActive > 10 {
		return fmt.Errorf("dashboard.max_active must be between 0 and 10")
	}
	if value.Dashboard.RefreshSeconds != 0 && value.Dashboard.RefreshSeconds != 10 && value.Dashboard.RefreshSeconds != 30 && value.Dashboard.RefreshSeconds != 60 {
		return fmt.Errorf("invalid dashboard.refresh_seconds")
	}
	if value.Attention.ProtectMinutes < 1 || value.Attention.ProtectMinutes > 120 {
		return fmt.Errorf("attention.protect_minutes must be between 1 and 120")
	}
	if value.Attention.FocusAddMinutes < 1 || value.Attention.FocusAddMinutes > 120 {
		return fmt.Errorf("attention.focus_add_minutes must be between 1 and 120")
	}
	if value.Attention.MaskStrength < .25 || value.Attention.MaskStrength > 1.5 {
		return fmt.Errorf("attention.mask_strength must be between 0.25 and 1.5")
	}
	if value.TimeEstimate.MinutesPerMessage < .5 || value.TimeEstimate.MinutesPerMessage > 30 {
		return fmt.Errorf("time_estimate.minutes_per_message must be between 0.5 and 30")
	}
	if value.TimeEstimate.FallbackRatio < .05 || value.TimeEstimate.FallbackRatio > 1 {
		return fmt.Errorf("time_estimate.fallback_ratio must be between 0.05 and 1")
	}
	return nil
}

func (m *Manager) Save(value File) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	value.Version = Version
	if err := validate(value); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0755); err != nil {
		return fmt.Errorf("create settings directory: %w", err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	if err := os.Rename(tmp, m.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace settings: %w", err)
	}
	return nil
}

func (m *Manager) Update(value File) (File, error) {
	if err := m.Save(value); err != nil {
		return File{}, err
	}
	return m.Load()
}

func (m *Manager) Reset(section string) (File, error) {
	current, err := m.Load()
	if err != nil {
		return File{}, err
	}
	defaults := Default()
	switch section {
	case "dashboard":
		current.Dashboard = defaults.Dashboard
	case "attention":
		current.Attention = defaults.Attention
	case "time_estimate":
		current.TimeEstimate = defaults.TimeEstimate
	case "all":
		current = defaults
	default:
		return File{}, fmt.Errorf("unknown settings section %q", section)
	}
	return m.Update(current)
}

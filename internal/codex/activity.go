// Package codex reads local Codex CLI rollout records.
package codex

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pancake-lee/pflow/internal/config"
)

// SessionSummary is the privacy-preserving dashboard view of one Codex session.
type SessionSummary struct {
	SessionID    string
	Project      string
	Status       string
	FirstActive  time.Time
	LastActive   time.Time
	Name         string
	LastReq      string
	LastResp     string
	MessageCount int
}

func (s SessionSummary) IsActive() bool {
	return s.Status == "busy" || s.Status == "waiting" || s.Status == "idle"
}

func (s SessionSummary) TrafficLight() string {
	switch s.Status {
	case "busy":
		return "🟢"
	case "waiting":
		return "🟡"
	case "idle":
		return "⚪"
	default:
		return "⚫"
	}
}

func (s SessionSummary) StatusLabel() string { return s.TrafficLight() + " " + s.Status }

// ScanResult is the result of scanning Codex rollout records.
type ScanResult struct {
	ActiveWindow time.Duration
	Cutoff       time.Time
	Now          time.Time
	Sessions     []SessionSummary
	Diagnostics  []string
}

// Scan reads ~/.codex/sessions. Missing directories are treated as no sessions.
func Scan(opts config.ScanOptions) (*ScanResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot find home directory: %w", err)
	}
	return ScanDir(filepath.Join(home, ".codex", "sessions"), opts, time.Now())
}

// ScanDir scans a rollout root. It is exported to make the file format testable.
func ScanDir(root string, opts config.ScanOptions, now time.Time) (*ScanResult, error) {
	result := &ScanResult{ActiveWindow: opts.Window, Cutoff: now.Add(-opts.Window), Now: now}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, fmt.Errorf("read sessions directory: %w", err)
	}
	_ = entries // ReadDir verifies the root before WalkDir.
	byID := make(map[string]SessionSummary)
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			result.Diagnostics = append(result.Diagnostics, fmt.Sprintf("%s: %v", path, walkErr))
			return nil
		}
		if d.IsDir() || !strings.HasPrefix(d.Name(), "rollout-") || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		s, diags := parseRollout(path)
		result.Diagnostics = append(result.Diagnostics, diags...)
		if s.SessionID == "" || s.LastActive.Before(result.Cutoff) {
			return nil
		}
		if old, ok := byID[s.SessionID]; !ok || s.LastActive.After(old.LastActive) {
			byID[s.SessionID] = s
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk sessions: %w", err)
	}
	for _, s := range byID {
		result.Sessions = append(result.Sessions, s)
	}
	sort.Slice(result.Sessions, func(i, j int) bool { return result.Sessions[i].LastActive.After(result.Sessions[j].LastActive) })
	return result, nil
}

type rolloutLine struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

func parseRollout(path string) (SessionSummary, []string) {
	f, err := os.Open(path)
	if err != nil {
		return SessionSummary{}, []string{fmt.Sprintf("%s: %v", path, err)}
	}
	defer f.Close()
	var s SessionSummary
	var diags []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		var line rolloutLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			diags = append(diags, fmt.Sprintf("%s:%d: invalid JSONL: %v", path, lineNo, err))
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, line.Timestamp)
		if err != nil {
			diags = append(diags, fmt.Sprintf("%s:%d: invalid timestamp", path, lineNo))
			continue
		}
		if s.FirstActive.IsZero() || at.Before(s.FirstActive) {
			s.FirstActive = at
		}
		if at.After(s.LastActive) {
			s.LastActive = at
		}
		switch line.Type {
		case "session_meta":
			var p struct {
				SessionID string `json:"session_id"`
				ID        string `json:"id"`
				CWD       string `json:"cwd"`
				Timestamp string `json:"timestamp"`
			}
			if json.Unmarshal(line.Payload, &p) != nil {
				diags = append(diags, fmt.Sprintf("%s:%d: invalid session metadata", path, lineNo))
				continue
			}
			s.SessionID = firstNonEmpty(p.SessionID, p.ID)
			s.Project = p.CWD
			if p.Timestamp != "" {
				if started, e := time.Parse(time.RFC3339Nano, p.Timestamp); e == nil {
					s.FirstActive = started
				}
			}
		case "event_msg":
			var p struct {
				Type string `json:"type"`
			}
			if json.Unmarshal(line.Payload, &p) != nil {
				diags = append(diags, fmt.Sprintf("%s:%d: invalid event", path, lineNo))
				continue
			}
			switch p.Type {
			case "task_started":
				s.Status = "busy"
			case "task_complete", "turn_aborted":
				s.Status = "idle"
			}
		case "response_item":
			var p struct {
				Type    string `json:"type"`
				Role    string `json:"role"`
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			}
			if json.Unmarshal(line.Payload, &p) != nil {
				diags = append(diags, fmt.Sprintf("%s:%d: invalid response item", path, lineNo))
				continue
			}
			text := ""
			for _, c := range p.Content {
				if c.Text != "" {
					text = c.Text
					break
				}
			}
			switch {
			case p.Type == "message" && p.Role == "user":
				s.MessageCount++
				s.LastReq = truncate(text, 160)
				if s.Name == "" {
					s.Name = truncate(text, 48)
				}
				s.Status = "busy"
			case p.Type == "message" && p.Role == "assistant":
				s.LastResp = truncate(text, 160)
				s.Status = "idle"
			case p.Type == "function_call", p.Type == "custom_tool_call":
				s.Status = "busy"
			}
		}
	}
	if err := scanner.Err(); err != nil {
		diags = append(diags, fmt.Sprintf("%s: read error: %v", path, err))
	}
	if s.Status == "" {
		s.Status = "unknown"
	}
	return s, diags
}

// FindSessionStartedAfter finds the newest session for workDir created after start.
func FindSessionStartedAfter(workDir string, start time.Time) (string, error) {
	result, err := Scan(config.ScanOptions{Window: 24 * time.Hour, MaxInactive: 0})
	if err != nil {
		return "", err
	}
	for _, s := range result.Sessions {
		if s.Project == workDir && !s.FirstActive.Before(start) {
			return s.SessionID, nil
		}
	}
	return "", nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
func truncate(text string, max int) string {
	r := []rune(strings.TrimSpace(text))
	if len(r) <= max {
		return string(r)
	}
	return string(r[:max]) + "..."
}

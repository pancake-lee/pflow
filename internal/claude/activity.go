// Package claude provides access to Claude Code's local data (sessions, history).
package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pancake-lee/pflow/internal/config"
)

// SessionMeta is the JSON structure in ~/.claude/sessions/<pid>.json.
type SessionMeta struct {
	PID        int    `json:"pid"`
	SessionID  string `json:"sessionId"`
	CWD        string `json:"cwd"`
	StartedAt  int64  `json:"startedAt"`
	UpdatedAt  int64  `json:"updatedAt"`
	Version    string `json:"version"`
	Status     string `json:"status"`
	Kind       string `json:"kind"`
	Entrypoint string `json:"entrypoint"`
}

// HistoryEntry is one line from ~/.claude/history.jsonl.
type HistoryEntry struct {
	Display        string `json:"display"`
	PastedContents any    `json:"pastedContents"`
	Timestamp      int64  `json:"timestamp"`
	Project        string `json:"project"`
	SessionID      string `json:"sessionId"`
}

// SessionSummary is the aggregated view of one Claude Code session.
type SessionSummary struct {
	SessionID    string
	Project      string
	Status       string // from session metadata (busy/waiting/idle/unknown)
	PID          int    // 0 if session file not present or process not found
	IsRunning    bool   // whether the PID from session metadata is alive
	MessageCount int    // number of history entries in the active window
	FirstActive  time.Time
	LastActive   time.Time
	Name         string // session title or first user message (first ~15 chars)
	LastReq      string // first ~15 chars of latest user message (for table)
	LastResp     string // first ~15 chars of latest assistant text response (for table)
	LastReqFull  string // full text of latest user message (for detail view)
	LastRespFull string // full text of latest assistant response (for detail view)
}

// IsActive returns true if the session is in an active state (busy, waiting, or idle).
// All other states (unknown / no metadata) are considered inactive.
func (s SessionSummary) IsActive() bool {
	switch s.Status {
	case "busy", "waiting", "idle":
		return true
	default:
		return false
	}
}

// TrafficLight returns the traffic-light icon for the session's status.
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

// StatusLabel returns a human-readable status with traffic light.
func (s SessionSummary) StatusLabel() string {
	light := s.TrafficLight()
	switch s.Status {
	case "busy":
		return light + " busy"
	case "waiting":
		return light + " waiting"
	case "idle":
		return light + " idle"
	default:
		return light + " inactive"
	}
}

// ScanResult holds the full scan output.
type ScanResult struct {
	ActiveWindow time.Duration
	Cutoff       time.Time
	Now          time.Time
	Sessions     []SessionSummary
}

// claudeDir returns the path to ~/.claude.
func claudeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot find home directory: %w", err)
	}
	return filepath.Join(home, ".claude"), nil
}

// Scan reads Claude Code's local data and returns summaries for sessions
// that were active within the configured window.
func Scan(opts config.ScanOptions) (*ScanResult, error) {
	cd, err := claudeDir()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	cutoff := now.Add(-opts.Window)

	// Read session metadata files
	sessionMetas := readSessionMetas(cd)

	// Read history entries in the active window
	historyEntries, err := readHistory(cd, cutoff)
	if err != nil {
		return nil, fmt.Errorf("reading history: %w", err)
	}

	// Aggregate by session ID
	agg := aggregate(sessionMetas, historyEntries)

	// Enrich with last request/response from transcript files
	transcripts := readTranscripts(cd)
	for i := range agg {
		if t, ok := transcripts[agg[i].SessionID]; ok {
			agg[i].LastReq = t.lastReq
			agg[i].LastResp = t.lastResp
			agg[i].LastReqFull = t.lastReqFull
			agg[i].LastRespFull = t.lastRespFull
		}
	}

	// Apply max-inactive filter per project
	agg = applyMaxInactive(agg, opts.MaxInactive)

	// Sort by last active time (most recent first)
	sort.Slice(agg, func(i, j int) bool {
		return agg[i].LastActive.After(agg[j].LastActive)
	})

	return &ScanResult{
		ActiveWindow: opts.Window,
		Cutoff:       cutoff,
		Now:          now,
		Sessions:     agg,
	}, nil
}

// readSessionMetas reads all JSON files from ~/.claude/sessions/.
func readSessionMetas(claudeDir string) map[string]*SessionMeta {
	dir := filepath.Join(claudeDir, "sessions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // sessions dir may not exist
	}

	result := make(map[string]*SessionMeta)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var m SessionMeta
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		result[m.SessionID] = &m
	}
	return result
}

// readHistory reads history.jsonl and returns entries after the cutoff.
func readHistory(claudeDir string, cutoff time.Time) ([]HistoryEntry, error) {
	path := filepath.Join(claudeDir, "history.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var entries []HistoryEntry
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var e HistoryEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		if !e.TimestampTime().After(cutoff) {
			continue
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// TimestampTime converts the millisecond timestamp to time.Time.
func (h HistoryEntry) TimestampTime() time.Time {
	return time.Unix(0, h.Timestamp*int64(time.Millisecond))
}

// aggregate combines session metadata and history entries into summaries.
func aggregate(metas map[string]*SessionMeta, history []HistoryEntry) []SessionSummary {
	// Collect all unique session IDs from both sources
	seen := make(map[string]bool)
	for _, h := range history {
		seen[h.SessionID] = true
	}
	for sid := range metas {
		// Only include sessions that have metadata AND were recently updated
		seen[sid] = true
	}

	// Build per-session history stats
	type histStats struct {
		count       int
		first, last time.Time
		firstMsg    string
		project     string
	}
	hist := make(map[string]*histStats)
	for _, h := range history {
		sid := h.SessionID
		if _, ok := hist[sid]; !ok {
			hist[sid] = &histStats{first: h.TimestampTime(), last: h.TimestampTime(), firstMsg: h.Display, project: h.Project}
		}
		s := hist[sid]
		s.count++
		if t := h.TimestampTime(); t.Before(s.first) {
			s.first = t
			s.firstMsg = h.Display
		}
		if t := h.TimestampTime(); t.After(s.last) {
			s.last = t
		}
		if h.Project != "" {
			s.project = h.Project
		}
	}

	var result []SessionSummary
	for sid := range seen {
		ss := SessionSummary{
			SessionID: sid,
			Status:    "unknown",
		}

		// Merge session metadata
		if m, ok := metas[sid]; ok {
			ss.PID = m.PID
			ss.Status = m.Status
			ss.IsRunning = isProcessAlive(m.PID)
			if m.CWD != "" {
				ss.Project = m.CWD
			}
		}

		// Merge history stats
		if hs, ok := hist[sid]; ok {
			ss.MessageCount = hs.count
			ss.FirstActive = hs.first
			ss.LastActive = hs.last
			if hs.project != "" {
				ss.Project = hs.project
			}
			if hs.firstMsg != "" {
				ss.Name = truncateText(hs.firstMsg, 15)
			}
		}

		// If session metadata has an updatedAt timestamp, use it for LastActive if newer
		if m, ok := metas[sid]; ok {
			t := time.Unix(0, m.UpdatedAt*int64(time.Millisecond))
			if t.After(ss.LastActive) {
				ss.LastActive = t
			}
			// If no history entries, use metadata timestamps
			if ss.FirstActive.IsZero() {
				ss.FirstActive = time.Unix(0, m.StartedAt*int64(time.Millisecond))
			}
		}

		// Only include sessions that have some data and last activity within window
		// (Skip entries that have no metadata, no history, or no time info)
		if ss.Project == "" && ss.MessageCount == 0 {
			continue
		}

		result = append(result, ss)
	}

	return result
}

// transcriptInfo holds the last user request and assistant response extracted
// from a Claude Code transcript file (~/.claude/projects/.../<session>.jsonl).
type transcriptInfo struct {
	lastReq     string // truncated (15 chars) for table display
	lastResp    string // truncated (15 chars) for table display
	lastReqFull string // full text for detail view
	lastRespFull string // full text for detail view
}

// readTranscripts scans ~/.claude/projects/ for transcript files and extracts
// the last user message and last assistant text response for each session.
func readTranscripts(claudeDir string) map[string]*transcriptInfo {
	dir := filepath.Join(claudeDir, "projects")
	result := make(map[string]*transcriptInfo)

	// Walk ~/.claude/projects/ — structure is: projects/<project-name>/<session-id>.jsonl
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		// Session ID is the filename without .jsonl
		sessionID := strings.TrimSuffix(d.Name(), ".jsonl")
		if sessionID == "" {
			return nil
		}

		info := parseTranscriptFile(path)
		if info != nil {
			result[sessionID] = info
		}
		return nil
	})

	return result
}

// parseTranscriptFile reads a transcript file and extracts the last user
// message and last assistant text response.
func parseTranscriptFile(path string) *transcriptInfo {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var lastUser, lastUserFull, lastAssistant, lastAssistantFull string

	for ev := range ParseEvents(f) {
		switch ev.Type {
		case "user":
			ue, err := ev.ParseUser()
			if err != nil {
				continue
			}
			text := ue.Text()
			if text != "" {
				lastUser = truncateText(text, 15)
				lastUserFull = text
			}
		case "assistant":
			ae, err := ev.ParseAssistant()
			if err != nil {
				continue
			}
			text := ae.Text()
			if text != "" {
				lastAssistant = truncateText(text, 15)
				lastAssistantFull = text
			}
		}
	}

	if lastUser == "" && lastAssistant == "" {
		return nil
	}

	return &transcriptInfo{
		lastReq:      lastUser,
		lastResp:     lastAssistant,
		lastReqFull:  lastUserFull,
		lastRespFull: lastAssistantFull,
	}
}

// isProcessAlive checks if a PID corresponds to a running process.
// A simple check — we use /proc on Linux.
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	_, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
	return err == nil
}

// applyMaxInactive limits the number of inactive sessions per project.
// Active sessions are always kept; inactive (unknown) sessions are limited
// to maxInactive per project (the most recent ones). If maxInactive is 0,
// all sessions are kept.
func applyMaxInactive(summaries []SessionSummary, maxInactive int) []SessionSummary {
	if maxInactive <= 0 {
		return summaries
	}

	// Group by project
	type group struct {
		active   []SessionSummary
		inactive []SessionSummary
	}
	groups := make(map[string]*group)
	var projOrder []string

	for _, s := range summaries {
		proj := s.Project
		if _, ok := groups[proj]; !ok {
			groups[proj] = &group{}
			projOrder = append(projOrder, proj)
		}
		if s.IsActive() {
			groups[proj].active = append(groups[proj].active, s)
		} else {
			groups[proj].inactive = append(groups[proj].inactive, s)
		}
	}

	var result []SessionSummary
	for _, proj := range projOrder {
		g := groups[proj]
		// Keep all active sessions
		result = append(result, g.active...)
		// Sort inactive by LastActive descending, then keep only maxInactive most recent.
		// Without sorting, map iteration order makes the truncated result non-deterministic.
		sort.Slice(g.inactive, func(i, j int) bool {
			return g.inactive[i].LastActive.After(g.inactive[j].LastActive)
		})
		if len(g.inactive) > maxInactive {
			g.inactive = g.inactive[:maxInactive]
		}
		result = append(result, g.inactive...)
	}

	return result
}

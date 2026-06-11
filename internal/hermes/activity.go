// Package hermes provides access to Hermes Agent's local data (sessions, gateway state).
package hermes

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ActiveSession mirrors one entry in ~/.hermes/sessions/sessions.json.
type ActiveSession struct {
	SessionKey   string  `json:"session_key"`
	SessionID    string  `json:"session_id"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
	DisplayName  *string `json:"display_name"`
	Platform     string  `json:"platform"`
	ChatType     string  `json:"chat_type"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TotalTokens  int     `json:"total_tokens"`
	Suspended    bool    `json:"suspended"`
	Origin       *struct {
		Platform string `json:"platform"`
		ChatName *string `json:"chat_name"`
		ChatType string `json:"chat_type"`
		UserName *string `json:"user_name"`
	} `json:"origin,omitempty"`
}

// parseTime parses a Hermes timestamp string (ISO 8601 without timezone).
// Hermes stores times in local time without timezone offset, so we use ParseInLocation.
func parseTime(s string) (time.Time, error) {
	// Hermes uses format "2026-06-11T14:41:33.639638"
	t, err := time.ParseInLocation("2006-01-02T15:04:05.999999", s, time.Local)
	if err != nil {
		// Fallback: try without microseconds
		t, err = time.ParseInLocation("2006-01-02T15:04:05", s, time.Local)
	}
	return t, err
}

// GatewayState mirrors ~/.hermes/gateway_state.json.
type GatewayState struct {
	PID          int                       `json:"pid"`
	GatewayState string                   `json:"gateway_state"`
	ActiveAgents int                      `json:"active_agents"`
	Platforms    map[string]PlatformInfo  `json:"platforms"`
	UpdatedAt    string                   `json:"updated_at"`
}

// PlatformInfo describes one messaging platform's connection state.
type PlatformInfo struct {
	State        string `json:"state"`
	ErrorMessage *string `json:"error_message"`
	UpdatedAt    string `json:"updated_at"`
}

// SessionSummary is the aggregated view of one Hermes session.
type SessionSummary struct {
	SessionID    string
	Project      string // working directory (from system prompt) or platform name
	Platform     string // weixin, cli, cron
	ChatType     string // dm, group
	IsSuspended  bool
	IsActive     bool // true = from sessions.json (actively tracked), false = from dump file
	DisplayName  string
	MessageCount int // from request_dump files
	InputTokens  int
	OutputTokens int
	FirstActive  time.Time
	LastActive   time.Time
	Name         string // session title or first user message (first ~15 chars)
	LastReq      string // first ~15 chars of latest user message
	LastResp     string // first ~15 chars of latest assistant response
}

// ScanResult holds the full scan output for Hermes.
type ScanResult struct {
	ActiveWindow time.Duration
	Cutoff       time.Time
	Now          time.Time
	GatewayAlive bool
	Sessions     []SessionSummary
}

// hermesDir returns the path to ~/.hermes.
func hermesDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot find home directory: %w", err)
	}
	return filepath.Join(home, ".hermes"), nil
}

// Scan reads Hermes Agent's local data and returns summaries for sessions
// that were active within the given window.
func Scan(activeWindow time.Duration) (*ScanResult, error) {
	hd, err := hermesDir()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	cutoff := now.Add(-activeWindow)

	// Scan dump files first to build a cwd lookup map for gateway sessions.
	dumpSessions := scanDumpFiles(hd, cutoff)
	dumpCWD := make(map[string]string) // session_id → cwd from dump files
	for _, ds := range dumpSessions {
		if ds.Project != "" {
			dumpCWD[ds.SessionID] = ds.Project
		}
	}

	// Read active sessions from sessions.json (gateway-tracked)
	sessions := readActiveSessions(hd)

	// Read gateway state
	gw := readGatewayState(hd)

	// Track session IDs we've already seen (from sessions.json)
	seen := make(map[string]bool)

	// Build summaries, filtering by active window
	var summaries []SessionSummary
	for _, s := range sessions {
		updatedAt, err := parseTime(s.UpdatedAt)
		if err != nil {
			updatedAt = now
		}

		// Skip sessions that haven't been touched within the active window
		if !updatedAt.After(cutoff) {
			continue
		}

		createdAt, _ := parseTime(s.CreatedAt)

		name := ""
		if s.DisplayName != nil {
			name = *s.DisplayName
		}
		if name == "" && s.Origin != nil && s.Origin.ChatName != nil {
			name = *s.Origin.ChatName
		}
		if name == "" && s.Origin != nil && s.Origin.UserName != nil {
			name = *s.Origin.UserName
		}

		// Session name: use DisplayName if set (e.g. WeChat user name), otherwise empty
		sessionName := ""
		if name != "" {
			sessionName = truncateRunes(name, 15)
		}

		// Try to get working directory from matching dump file
		project := s.Platform // fallback
		if cwd, ok := dumpCWD[s.SessionID]; ok {
			project = cwd
		}

		seen[s.SessionID] = true
		summaries = append(summaries, SessionSummary{
			SessionID:    s.SessionID,
			Project:      project,
			Platform:     s.Platform,
			ChatType:     s.ChatType,
			IsSuspended:  s.Suspended,
			IsActive:     true, // tracked by gateway
			DisplayName:  name,
			InputTokens:  s.InputTokens,
			OutputTokens: s.OutputTokens,
			FirstActive:  createdAt,
			LastActive:   updatedAt,
			Name:         sessionName,
		})
	}

	// Add dump-based sessions (CLI/cron not tracked by gateway)
	for _, ds := range dumpSessions {
		if seen[ds.SessionID] {
			continue
		}
		// For dump-based sessions, use the user message as name (no explicit title)
		dumpName := ""
		if ds.LastReq != "" {
			dumpName = ds.LastReq
		}

		project := ds.Project
		if project == "" {
			project = ds.Platform
		}

		seen[ds.SessionID] = true
		summaries = append(summaries, SessionSummary{
			SessionID:   ds.SessionID,
			Project:     project,
			Platform:    ds.Platform,
			ChatType:    "dm",
			DisplayName: "",
			FirstActive: ds.FirstActive,
			LastActive:  ds.LastActive,
			Name:        dumpName,
			LastReq:     ds.LastReq,
		})
	}

	// Sort by last active (most recent first)
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].LastActive.After(summaries[j].LastActive)
	})

	gwAlive := gw != nil && gw.GatewayState == "running"

	return &ScanResult{
		ActiveWindow: activeWindow,
		Cutoff:       cutoff,
		Now:          now,
		GatewayAlive: gwAlive,
		Sessions:     summaries,
	}, nil
}

// readActiveSessions reads ~/.hermes/sessions/sessions.json.
func readActiveSessions(hd string) map[string]ActiveSession {
	path := filepath.Join(hd, "sessions", "sessions.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var raw map[string]ActiveSession
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	return raw
}

// readGatewayState reads ~/.hermes/gateway_state.json.
func readGatewayState(hd string) *GatewayState {
	path := filepath.Join(hd, "gateway_state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var gw GatewayState
	if err := json.Unmarshal(data, &gw); err != nil {
		return nil
	}
	return &gw
}

// dumpFileInfo holds info parsed from a request_dump filename and body.
type dumpFileInfo struct {
	SessionID   string
	Platform    string // "cli" or "cron"
	FirstActive time.Time
	LastActive  time.Time
	LastReq     string // first ~15 chars of user message extracted from body
	Project     string // working directory from system prompt
}

// scanDumpFiles scans ~/.hermes/sessions/ for request_dump_*.json files
// and extracts session info from filenames + file modification times.
// Filename format:
//
//	CLI:  request_dump_{YYYYMMDD}_{HHMMSS}_{suffix}_{YYYYMMDD}_{HHMMSS}_{id}.json
//	Cron: request_dump_cron_{cronid}_{YYYYMMDD}_{HHMMSS}_{YYYYMMDD}_{HHMMSS}_{id}.json
func scanDumpFiles(hd string, cutoff time.Time) []dumpFileInfo {
	dir := filepath.Join(hd, "sessions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var results []dumpFileInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "request_dump_") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		if name == e.Name() {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		di := parseDumpFilename(name, info.ModTime())
		if di == nil {
			continue
		}

		// Extract user message and cwd from dump body
		meta := readDumpMeta(filepath.Join(dir, e.Name()))
		if meta != nil {
			di.LastReq = truncateRunes(meta.userMsg, 15)
			if meta.cwd != "" && meta.cwd != "/" {
				di.Project = meta.cwd
			}
		}

		// Only include sessions active within the window
		if di.LastActive.After(cutoff) || di.FirstActive.After(cutoff) {
			results = append(results, *di)
		}
	}

	// Sort by last active (most recent first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].LastActive.After(results[j].LastActive)
	})

	return results
}

// dumpBody is a minimal structure for extracting metadata from a request_dump file.
type dumpBody struct {
	Request struct {
		Body struct {
			System   string `json:"system"`
			Messages []struct {
				Role    string `json:"role"`
				Content any    `json:"content"` // string or []contentBlock
			} `json:"messages"`
		} `json:"body"`
	} `json:"request"`
}

// dumpMeta holds metadata extracted from a request_dump file.
type dumpMeta struct {
	userMsg string // last user message text
	cwd     string // working directory from system prompt
}

// readDumpMeta reads a request_dump file and extracts user message + working directory.
func readDumpMeta(path string) *dumpMeta {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var dump dumpBody
	if err := json.Unmarshal(data, &dump); err != nil {
		return nil
	}

	meta := &dumpMeta{}

	// Extract working directory from system prompt
	meta.cwd = extractCWD(dump.Request.Body.System)

	// Find the last user message
	for _, msg := range dump.Request.Body.Messages {
		if msg.Role != "user" {
			continue
		}
		switch c := msg.Content.(type) {
		case string:
			meta.userMsg = c
		case []any:
			var parts []string
			for _, item := range c {
				if block, ok := item.(map[string]any); ok {
					if t, _ := block["type"].(string); t == "text" {
						if text, _ := block["text"].(string); text != "" {
							parts = append(parts, text)
						}
					}
				}
			}
			if len(parts) > 0 {
				meta.userMsg = strings.Join(parts, " ")
			}
		}
	}
	return meta
}

// extractCWD parses the "Current working directory: <path>" line from a system prompt.
func extractCWD(system string) string {
	for _, line := range strings.Split(system, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "Current working directory:") {
			cwd := strings.TrimPrefix(line, "Current working directory:")
			cwd = strings.TrimSpace(cwd)
			return cwd
		}
	}
	return ""
}

// parseDumpFilename extracts session info from a request_dump filename.
func parseDumpFilename(name string, modTime time.Time) *dumpFileInfo {
	// Remove "request_dump_" prefix
	rest := strings.TrimPrefix(name, "request_dump_")
	if rest == name {
		return nil
	}

	parts := strings.Split(rest, "_")
	if len(parts) < 5 {
		return nil
	}

	// Last 3 parts are: endDate, endTime, requestId
	// Session ID is parts[0:len-3]
	sessionParts := parts[:len(parts)-3]
	sessionID := strings.Join(sessionParts, "_")

	// Determine platform
	platform := "cli"
	if len(sessionParts) > 0 && sessionParts[0] == "cron" {
		platform = "cron"
	}

	// Parse end time from parts[len-3] + parts[len-2]
	endDate := parts[len(parts)-3]
	endTime := parts[len(parts)-2]
	endAt, err := time.ParseInLocation("20060102_150405", endDate+"_"+endTime, time.Local)
	if err != nil {
		// Fallback to file modification time
		endAt = modTime
	}

	// Use file mod time as LastActive (more reliable than parsed end time)
	lastActive := modTime
	if endAt.After(lastActive) {
		lastActive = endAt
	}

	return &dumpFileInfo{
		SessionID:   sessionID,
		Platform:    platform,
		FirstActive: endAt, // best estimate from filename
		LastActive:  lastActive,
	}
}

// PlatformIcon returns a simple icon for a platform.
func (s SessionSummary) PlatformIcon() string {
	switch s.Platform {
	case "weixin":
		return "💬"
	case "cli":
		return "⌨️"
	case "cron":
		return "⏰"
	default:
		return "📡"
	}
}

// StatusLabel returns a human-readable status.
func (s SessionSummary) StatusLabel() string {
	if !s.IsActive {
		return "— completed" // from dump file, not actively tracked
	}
	if s.IsSuspended {
		return "⏸ suspended"
	}
	return "▶ running"
}

// ShortID returns a truncated session ID.
func (s SessionSummary) ShortID() string {
	return TruncateID(s.SessionID, 16)
}

// TruncateID truncates a session ID to n characters.
func TruncateID(id string, n int) string {
	if len(id) <= n {
		return id
	}
	return id[:n]
}

// ActivePlatforms returns the list of active (non-paused) platforms from gateway state.
func ActivePlatforms(hd string) []string {
	gw := readGatewayState(hd)
	if gw == nil {
		return nil
	}
	var active []string
	for name, info := range gw.Platforms {
		if info.State == "connected" {
			active = append(active, name)
		}
	}
	sort.Strings(active)
	return active
}

// GatewaySummary returns a human-readable summary of the Hermes gateway.
func GatewaySummary(hd string) string {
	gw := readGatewayState(hd)
	if gw == nil {
		return "gateway not found"
	}
	var parts []string
	parts = append(parts, fmt.Sprintf("state=%s", gw.GatewayState))
	parts = append(parts, fmt.Sprintf("agents=%d", gw.ActiveAgents))
	for name, info := range gw.Platforms {
		if info.State != "paused" && info.State != "disabled" {
			parts = append(parts, fmt.Sprintf("%s=%s", name, info.State))
		}
	}
	return strings.Join(parts, " ")
}

// FindDir finds the .hermes directory or returns an error.
func FindDir() (string, error) {
	return hermesDir()
}

// truncateRunes truncates s to maxLen runes, appending "..." if truncated.
func truncateRunes(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

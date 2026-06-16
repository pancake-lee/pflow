// Package hermes provides access to Hermes Agent's local data (sessions, gateway state).
package hermes

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pancake-lee/pflow/internal/config"
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
	SessionID        string
	Project          string // working directory (from system prompt) or platform name
	Platform         string // weixin, cli, cron
	ChatType         string // dm, group
	IsSuspended      bool
	IsGatewayTracked bool // true = tracked by gateway (sessions.json), false = from dump file
	DisplayName      string
	MessageCount     int // from request_dump files
	InputTokens      int
	OutputTokens     int
	FirstActive      time.Time
	LastActive       time.Time
	Name             string // session title or first user message (first ~15 chars)
	LastReq          string // first ~15 chars of latest user message (for table)
	LastResp         string // first ~15 chars of latest assistant response (for table)
	LastReqFull      string // full text of latest user message (for detail view)
	LastRespFull     string // full text of latest assistant response (for detail view)
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

// exportSession mirrors one line of `hermes sessions export` JSONL output.
type exportSession struct {
	ID           string          `json:"id"`
	Source       string          `json:"source"`
	Title        string          `json:"title"`
	StartedAt    float64         `json:"started_at"`
	EndedAt      *float64        `json:"ended_at"`
	LastActive   float64         `json:"last_active"`
	MessageCount int             `json:"message_count"`
	Messages     []exportMessage `json:"messages"`
	SystemPrompt string          `json:"system_prompt"`
}

// exportMessage mirrors one message in the export JSONL.
type exportMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string or []contentBlock
}

// extractMessageText converts an exportMessage's Content (string or []any) to a
// plain text string.
func extractMessageText(msg exportMessage) string {
	switch c := msg.Content.(type) {
	case string:
		return c
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
		return strings.Join(parts, " ")
	}
	return ""
}

// Scan reads Hermes Agent's local data via `hermes sessions export`
// and returns summaries for sessions active within the configured window.
func Scan(opts config.ScanOptions) (*ScanResult, error) {
	hd, err := hermesDir()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	cutoff := now.Add(-opts.Window)

	// Read gateway state and gateway-tracked sessions (for enrichment)
	gw := readGatewayState(hd)
	gwSessionsByKey := readActiveSessions(hd)
	gwSessionsByID := make(map[string]ActiveSession)
	for _, s := range gwSessionsByKey {
		if s.SessionID != "" {
			gwSessionsByID[s.SessionID] = s
		}
	}

	// Scan dump files for cwd lookup and legacy fallback
	dumpSessions := scanDumpFiles(hd, cutoff)
	dumpCWD := make(map[string]string)
	for _, ds := range dumpSessions {
		if ds.Project != "" {
			dumpCWD[ds.SessionID] = ds.Project
		}
	}

	// ── Primary data source: hermes sessions export ────────────────
	var summaries []SessionSummary
	seen := make(map[string]bool)

	// Parse source filter
	var sourceFilter map[string]bool
	if opts.SourceFilter != "" {
		sourceFilter = make(map[string]bool)
		for _, src := range strings.Split(opts.SourceFilter, ",") {
			sourceFilter[strings.TrimSpace(src)] = true
		}
	}

	exported, exportErr := runSessionsExport(hd)
	if exportErr == nil {
		for _, es := range exported {
			if seen[es.ID] {
				continue
			}

			// Apply source filter
			if sourceFilter != nil {
				if !sourceFilter[es.Source] {
					seen[es.ID] = true // suppress from fallback loops
					continue
				}
			}

			// Determine timestamps
			startedAt := unixToTime(es.StartedAt)
			lastActive := startedAt
			if es.LastActive > 0 {
				lastActive = unixToTime(es.LastActive)
			} else if es.EndedAt != nil && *es.EndedAt > 0 {
				lastActive = unixToTime(*es.EndedAt)
			}

			// Apply time window filter
			if !lastActive.After(cutoff) {
				seen[es.ID] = true // suppress from fallback loops
				continue
			}

			// Extract last user/assistant messages
			lastUser, lastAssistant := extractLastMessages(es.Messages)

			// Name: use title, fallback to last user message
			name := ""
			if es.Title != "" {
				name = truncateRunes(es.Title, 15)
			} else if lastUser != "" {
				name = truncateRunes(lastUser, 15)
			}

			// Project: try cwd from dump files, then from system prompt
			project := es.Source
			if cwd, ok := dumpCWD[es.ID]; ok && cwd != "" {
				project = cwd
			} else if cwd := extractCWD(es.SystemPrompt); cwd != "" && cwd != "/" {
				project = cwd
			}
			if project == "" {
				project = es.Source
			}

			summary := SessionSummary{
				SessionID:   es.ID,
				Project:     project,
				Platform:    platformFromSource(es.Source),
				ChatType:    "dm",
				DisplayName: es.Title,
				MessageCount: es.MessageCount,
				FirstActive: startedAt,
				LastActive:  lastActive,
				Name:        name,
				LastReq:     truncateRunes(lastUser, 15),
				LastReqFull: lastUser,
				LastResp:    truncateRunes(lastAssistant, 15),
				LastRespFull: lastAssistant,
			}

			// Enrich with gateway metadata if available
			if gwSess, ok := gwSessionsByID[es.ID]; ok {
				summary.IsGatewayTracked = true
				summary.IsSuspended = gwSess.Suspended
				summary.InputTokens = gwSess.InputTokens
				summary.OutputTokens = gwSess.OutputTokens
				summary.ChatType = gwSess.ChatType

				if gwLastActive := lastActiveFromSessionsJSON(gwSess, summary.LastActive); gwLastActive.After(summary.LastActive) {
					summary.LastActive = gwLastActive
				}
				if summary.DisplayName == "" && gwSess.DisplayName != nil {
					summary.DisplayName = *gwSess.DisplayName
				}
				if summary.Name == "" && gwSess.DisplayName != nil {
					summary.Name = truncateRunes(*gwSess.DisplayName, 15)
				}
			}

			seen[es.ID] = true
			summaries = append(summaries, summary)
		}
	}

	// ── Add gateway-only sessions not in the export ─────────────────
	for _, s := range gwSessionsByID {
		if seen[s.SessionID] {
			continue
		}
		// Apply source filter
		if sourceFilter != nil && !sourceFilter[s.Platform] {
			continue
		}
		updatedAt, err := parseTime(s.UpdatedAt)
		if err != nil {
			updatedAt = now
		}
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

		project := s.Platform
		if cwd, ok := dumpCWD[s.SessionID]; ok {
			project = cwd
		}

		seen[s.SessionID] = true
		summaries = append(summaries, SessionSummary{
			SessionID:        s.SessionID,
			Project:          project,
			Platform:         s.Platform,
			ChatType:         s.ChatType,
			IsSuspended:      s.Suspended,
			IsGatewayTracked: true,
			DisplayName:      name,
			InputTokens:      s.InputTokens,
			OutputTokens:     s.OutputTokens,
			FirstActive:      createdAt,
			LastActive:       updatedAt,
			Name:             truncateRunes(name, 15),
		})
	}

	// ── Add dump-based sessions not already in summaries ─────────────
	for _, ds := range dumpSessions {
		if seen[ds.SessionID] {
			continue
		}
		// Apply source filter
		if sourceFilter != nil && !sourceFilter[ds.Platform] {
			continue
		}

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
			LastReqFull: ds.LastReqFull,
		})
	}

	// Apply max-inactive filter per project
	summaries = applyHermesMaxInactive(summaries, opts.MaxInactive)

	// Sort by last active (most recent first)
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].LastActive.After(summaries[j].LastActive)
	})

	gwAlive := gw != nil && gw.GatewayState == "running"

	return &ScanResult{
		ActiveWindow: opts.Window,
		Cutoff:       cutoff,
		Now:          now,
		GatewayAlive: gwAlive,
		Sessions:     summaries,
	}, nil
}

// runSessionsExport runs `hermes sessions export` and returns parsed sessions.
func runSessionsExport(hd string) ([]exportSession, error) {
	tmpFile := filepath.Join(hd, ".pflow_cache_export.jsonl")

	// Remove stale cache if exists
	os.Remove(tmpFile)

	cmd := exec.Command("hermes", "sessions", "export", tmpFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("hermes sessions export failed: %w\n%s", err, string(out))
	}

	f, err := os.Open(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("open export file: %w", err)
	}
	defer f.Close()

	var sessions []exportSession
	scanner := bufio.NewScanner(f)
	// Lines can be large (system_prompt contains full prompt text)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)

	for scanner.Scan() {
		var es exportSession
		if err := json.Unmarshal(scanner.Bytes(), &es); err != nil {
			continue // skip malformed lines
		}
		sessions = append(sessions, es)
	}

	// Clean up temp file
	os.Remove(tmpFile)

	return sessions, scanner.Err()
}

// extractLastMessages finds the last user and last assistant message
// from the exported messages array.
func extractLastMessages(messages []exportMessage) (lastUser, lastAssistant string) {
	for i := len(messages) - 1; i >= 0; i-- {
		m := messages[i]
		text := extractMessageText(m)
		if m.Role == "user" && lastUser == "" {
			lastUser = text
		} else if m.Role == "assistant" && lastAssistant == "" {
			lastAssistant = text
		}
		if lastUser != "" && lastAssistant != "" {
			break
		}
	}
	return
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
	LastReq     string // first ~15 chars of user message (for table)
	LastReqFull string // full text of user message (for detail view)
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
				di.LastReqFull = meta.userMsg
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

// IsActive returns true if the session is actively tracked by the gateway and running.
// Suspended and dump-only sessions are both mapped to inactive.
func (s SessionSummary) IsActive() bool {
	return s.IsGatewayTracked && !s.IsSuspended
}

// TrafficLight returns the traffic-light icon for the session's status.
func (s SessionSummary) TrafficLight() string {
	if !s.IsGatewayTracked {
		return "⚫"
	}
	if s.IsSuspended {
		return "⚫"
	}
	return "🟢"
}

// StatusLabel returns a human-readable status with traffic light.
func (s SessionSummary) StatusLabel() string {
	light := s.TrafficLight()
	if !s.IsGatewayTracked {
		return light + " inactive"
	}
	if s.IsSuspended {
		return light + " inactive"
	}
	return light + " running"
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

// ShortID returns a truncated session ID (last n characters).
// Hermes session IDs start with an 8-digit date (YYYYMMDD), which makes
// the prefix non-unique across sessions on the same day. Taking the suffix
// instead gives better uniqueness — matching how `hermes sessions list`
// displays session IDs.
func (s SessionSummary) ShortID() string {
	return SuffixID(s.SessionID, 16)
}

// SuffixID returns the last n characters of id, or the full id if it's
// shorter than n.
func SuffixID(id string, n int) string {
	if len(id) <= n {
		return id
	}
	return id[len(id)-n:]
}

// unixToTime converts a Unix timestamp (seconds as float64) to time.Time.
func unixToTime(ts float64) time.Time {
	sec := int64(ts)
	nsec := int64((ts - float64(sec)) * 1e9)
	return time.Unix(sec, nsec)
}

// platformFromSource maps hermes source values to pflow platform names.
func platformFromSource(source string) string {
	switch source {
	case "weixin":
		return "weixin"
	case "cron":
		return "cron"
	default:
		return "cli"
	}
}

// lastActiveFromSessionsJSON returns the updated_at timestamp from a
// gateway-tracked session, or fallback on error.
func lastActiveFromSessionsJSON(s ActiveSession, fallback time.Time) time.Time {
	t, err := parseTime(s.UpdatedAt)
	if err != nil {
		return fallback
	}
	return t
}

// TruncateID truncates a session ID to its first n characters.
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

// applyHermesMaxInactive limits the number of inactive sessions per project.
// Active sessions are always kept; inactive (completed, suspended) sessions are
// limited to maxInactive per project (the most recent ones). If maxInactive is 0,
// all sessions are kept.
func applyHermesMaxInactive(summaries []SessionSummary, maxInactive int) []SessionSummary {
	if maxInactive <= 0 {
		return summaries
	}

	type group struct {
		active   []SessionSummary
		inactive []SessionSummary
	}
	groups := make(map[string]*group)
	var projOrder []string

	for _, s := range summaries {
		proj := s.Project
		if proj == "" {
			proj = s.Platform
		}
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

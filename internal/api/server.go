// Package api provides the pflow HTTP API server.
package api

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"os"
	"os/exec"
	"strings"

	"github.com/pancake-lee/pflow/internal/attention"
	"github.com/pancake-lee/pflow/internal/claude"
	"github.com/pancake-lee/pflow/internal/config"
	"github.com/pancake-lee/pflow/internal/hermes"
	"github.com/pancake-lee/pflow/internal/project"
	"github.com/pancake-lee/pflow/internal/session"
	"github.com/pancake-lee/pflow/internal/suggest"
	plogger "github.com/pancake-lee/pgo/pkg/plogger"
)

// DashboardEntry is a unified session entry for the Dashboard API response.
type DashboardEntry struct {
	SessionID        string    `json:"session_id"`
	AgentType        string    `json:"agent_type"` // "claude" or "hermes"
	Project          string    `json:"project"`
	Status           string    `json:"status"`
	IsActive         bool      `json:"is_active"`
	TrafficLight     string    `json:"traffic_light"`
	Name             string    `json:"name"`
	FirstActive      time.Time `json:"first_active"`
	LastActive       time.Time `json:"last_active"`
	LastReq          string    `json:"last_req"`          // truncated ~15 chars for table
	LastResp         string    `json:"last_resp"`         // truncated ~15 chars for table
	LastReqFull      string    `json:"last_req_full"`     // full text for detail view
	LastRespFull     string    `json:"last_resp_full"`    // full text for detail view
	Platform         string    `json:"platform,omitempty"`   // Hermes only
	HasTerminal      bool      `json:"has_terminal"`         // true if a tmux mapping exists
	TerminalTmuxName string    `json:"terminal_tmux_name,omitempty"` // matched tmux session name
	MatchedRoot      string    `json:"matched_root,omitempty"`       // matched project root path (empty = unmatched)
}

// ProjectRootJSON is the JSON representation of a project root for API responses.
type ProjectRootJSON struct {
	Path     string `json:"path"`
	Priority string `json:"priority"`
}

// SuggestionJSON is the JSON representation of a suggest.Suggestion.
type SuggestionJSON struct {
	Icon     string `json:"icon"`
	Text     string `json:"text"`
	Priority int    `json:"priority"`
}

// DashboardResponse is the JSON response for GET /api/v1/dashboard.
type DashboardResponse struct {
	Now            time.Time                          `json:"now"`
	Window         string                             `json:"window"`
	ProjectRoots   []ProjectRootJSON                  `json:"project_roots"`
	Sessions       []DashboardEntry                   `json:"sessions"`
	ReminderScores map[string]attention.ReminderOutput `json:"reminder_scores"`
	Suggestions    []SuggestionJSON                   `json:"suggestions"`
	Focus          *attention.FocusSnapshot           `json:"focus,omitempty"`
	Errors         []string                           `json:"errors,omitempty"`
}

// Server is the pflow HTTP API server.
type Server struct {
	http.ServeMux
	staticFS   fs.FS // optional embedded static files (web/dist)
	sessionMgr *session.Manager
	projectMgr *project.Manager
}

// NewServer creates a new API server with registered routes.
// If staticFS is non-nil, static files (the Vue SPA) are served from it.
func NewServer(staticFS fs.FS, sessionMgr *session.Manager) *Server {
	s := &Server{
		staticFS:   staticFS,
		sessionMgr: sessionMgr,
		projectMgr: project.NewManager(),
	}
	s.HandleFunc("/api/v1/dashboard", s.handleDashboard)

	// Terminal management endpoints
	s.HandleFunc("POST /api/v1/terminal/start", s.handleTerminalStart)
	s.HandleFunc("POST /api/v1/terminal/stop", s.handleTerminalStop)
	s.HandleFunc("GET /api/v1/terminal/list", s.handleTerminalList)
	s.HandleFunc("GET /api/v1/terminal/lookup", s.handleTerminalLookup)

	// Project root management endpoints
	s.HandleFunc("GET /api/v1/project-roots", s.handleGetProjectRoots)
	s.HandleFunc("PUT /api/v1/project-roots", s.handlePutProjectRoot)
	s.HandleFunc("DELETE /api/v1/project-roots", s.handleDeleteProjectRoot)

	// Focus mode endpoints
	s.HandleFunc("POST /api/v1/focus/extend", s.handleFocusExtend)
	s.HandleFunc("POST /api/v1/focus/stop", s.handleFocusStop)

	// Serve static files if embedded, falling back to index.html for SPA routing.
	if staticFS != nil {
		s.Handle("/", spaHandler{staticFS: staticFS})
	}

	return s
}

// spaHandler serves static files from an embedded filesystem with
// SPA fallback: any path that doesn't match a real file returns index.html.
type spaHandler struct {
	staticFS fs.FS
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Don't intercept API routes
	if len(r.URL.Path) >= 4 && r.URL.Path[:4] == "/api" {
		http.NotFound(w, r)
		return
	}

	// Try to serve the exact file first
	path := "web/dist" + r.URL.Path
	if r.URL.Path == "/" {
		path = "web/dist/index.html"
	}

	f, err := h.staticFS.Open(path)
	if err == nil {
		defer f.Close()
		stat, _ := f.Stat()
		if stat != nil && !stat.IsDir() {
			http.ServeFileFS(w, r, h.staticFS, path)
			return
		}
	}

	// SPA fallback: serve index.html
	http.ServeFileFS(w, r, h.staticFS, "web/dist/index.html")
}

// handleDashboard handles GET /api/v1/dashboard.
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opts := parseQueryParams(r)

	// Load project roots
	rootsFile, rootsErr := s.projectMgr.Load()
	var projectRoots []ProjectRootJSON
	if rootsErr == nil {
		projectRoots = make([]ProjectRootJSON, 0, len(rootsFile.Roots))
		for _, r := range rootsFile.Roots {
			projectRoots = append(projectRoots, ProjectRootJSON{
				Path:     r.Path,
				Priority: string(r.Priority),
			})
		}
	}

	resp := DashboardResponse{
		Now:          time.Now(),
		Window:       opts.Window.String(),
		ProjectRoots: projectRoots,
	}

	var errors []string
	if rootsErr != nil {
		errors = append(errors, "project_roots: "+rootsErr.Error())
	}

	// Scan Claude Code sessions
	claudeResult, claudeErr := claude.Scan(opts)
	if claudeErr != nil {
		errors = append(errors, "claude: "+claudeErr.Error())
	} else {
		for _, s := range claudeResult.Sessions {
			entry := DashboardEntry{
				SessionID:    truncate8(s.SessionID),
				AgentType:    "claude",
				Project:      s.Project,
				Status:       s.Status,
				IsActive:     s.IsActive(),
				TrafficLight: s.TrafficLight(),
				Name:         s.Name,
					FirstActive:  s.FirstActive,
				LastActive:   s.LastActive,
				LastReq:      s.LastReq,
				LastResp:     s.LastResp,
				LastReqFull:  s.LastReqFull,
				LastRespFull: s.LastRespFull,
			}
			// Match to project root
			if rootsFile != nil {
				if matched := project.MatchRootFromList(rootsFile.Roots, s.Project); matched != nil {
					entry.MatchedRoot = matched.Path
				}
			}
			resp.Sessions = append(resp.Sessions, entry)
		}
	}

	// Scan Hermes sessions
	hermesResult, hermesErr := hermes.Scan(opts)
	if hermesErr != nil {
		errors = append(errors, "hermes: "+hermesErr.Error())
	} else {
		for _, s := range hermesResult.Sessions {
			entry := DashboardEntry{
				SessionID:    hermes.SuffixID(s.SessionID, 8),
				AgentType:    "hermes",
				Project:      s.Project,
				Status:       hermesRawStatus(s),
				IsActive:     s.IsActive(),
				TrafficLight: s.TrafficLight(),
				Name:         s.Name,
					FirstActive:  s.FirstActive,
				LastActive:   s.LastActive,
				LastReq:      s.LastReq,
				LastResp:     s.LastResp,
				LastReqFull:  s.LastReqFull,
				LastRespFull: s.LastRespFull,
				Platform:     s.Platform,
			}
			// Match to project root
			if rootsFile != nil {
				if matched := project.MatchRootFromList(rootsFile.Roots, s.Project); matched != nil {
					entry.MatchedRoot = matched.Path
				}
			}
			resp.Sessions = append(resp.Sessions, entry)
		}
	}

	// Load tmux mappings (used for terminal annotation and suggest).
	mappings, mappingsErr := session.LoadMappings()

	// Annotate sessions with terminal mapping info.
	// Build a lookup map from (agentType, sessionID prefix/suffix) → tmuxName.
	if mappingsErr == nil && len(mappings) > 0 {
		// Build multi-key lookup: for each mapping, add entries for:
		//   - agentType + full session ID (exact match)
		//   - "claude" + SessionIDPrefix (8-char prefix, for dashboard SessionID)
		//   - "hermes" + SessionIDPrefix (8-char prefix)
		type tmuxKey struct {
			agentType string
			sessionID string
		}
		keyToTmux := make(map[tmuxKey]string, len(mappings)*2)
		for _, m := range mappings {
			prefix := m.SessionIDPrefix()
			if m.AgentType != "" && m.AgentSessionID != "" {
				keyToTmux[tmuxKey{m.AgentType, m.AgentSessionID}] = m.TmuxName
			}
			if prefix != "" {
				// Map by agent type + prefix
				agentType := m.AgentType
				if agentType == "" {
					agentType = "claude" // legacy mappings are Claude
				}
				keyToTmux[tmuxKey{agentType, prefix}] = m.TmuxName
			}
		}
		for i := range resp.Sessions {
			entry := &resp.Sessions[i]
			if tmuxName, ok := keyToTmux[tmuxKey{entry.AgentType, entry.SessionID}]; ok {
				entry.HasTerminal = true
				entry.TerminalTmuxName = tmuxName
			}
		}
	}

	// Compute reminder scores per project.
	resp.ReminderScores = computeReminderScores(resp.Sessions, rootsFile, resp.Now)

	// Generate suggest analysis (军情哨)
	resp.Suggestions = generateSuggestions(claudeResult, hermesResult, mappings, rootsFile, resp.Now)

	// Include focus state
	if focusActive, focusedProject, focusMinutes, focusSince := attention.GetFocus().Snapshot(); focusActive {
		resp.Focus = &attention.FocusSnapshot{Active: true, FocusedProject: focusedProject, Minutes: focusMinutes, Since: focusSince.Format(time.RFC3339)}
	}

	if len(errors) > 0 {
		resp.Errors = errors
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// parseQueryParams extracts ScanOptions from query parameters.
func parseQueryParams(r *http.Request) config.ScanOptions {
	q := r.URL.Query()

	opts := config.ScanOptions{
		Window:       config.DefaultWindow,
		MaxInactive:  config.DefaultMaxInactive,
		SourceFilter: config.DefaultHermesSourceFilter,
	}

	if w := q.Get("window"); w != "" {
		if d, err := config.ParseWindow(w); err == nil {
			opts.Window = d
		}
	}

	if m := q.Get("max_inactive"); m != "" {
		if n, err := strconv.Atoi(m); err == nil && n >= 0 {
			opts.MaxInactive = n
		}
	}

	if s := q.Get("source"); s != "" {
		opts.SourceFilter = s
	}

	return opts
}

// ── Project root management handlers ──────────────────────────────

type putProjectRootReq struct {
	Path     string `json:"path"`
	Priority string `json:"priority"`
}

// handleGetProjectRoots handles GET /api/v1/project-roots.
func (s *Server) handleGetProjectRoots(w http.ResponseWriter, r *http.Request) {
	rf, err := s.projectMgr.Load()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	roots := make([]ProjectRootJSON, 0, len(rf.Roots))
	for _, r := range rf.Roots {
		roots = append(roots, ProjectRootJSON{Path: r.Path, Priority: string(r.Priority)})
	}
	if roots == nil {
		roots = []ProjectRootJSON{} // always return array, not null
	}

	writeJSON(w, http.StatusOK, roots)
}

// handlePutProjectRoot handles PUT /api/v1/project-roots.
// Body: {"path": "/home/user/code/pflow", "priority": "primary"}
func (s *Server) handlePutProjectRoot(w http.ResponseWriter, r *http.Request) {
	var req putProjectRootReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body: " + err.Error()})
		return
	}
	if req.Path == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "path is required"})
		return
	}

	// Clean the path
	req.Path = filepath.Clean(req.Path)

	// Reject root directory
	if req.Path == "/" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot mark root directory '/' as a project root"})
		return
	}

	priority := project.Priority(req.Priority)
	if priority == "" {
		priority = project.PriorityNormal
	}

	if err := s.projectMgr.SetPriority(req.Path, priority); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"path":     req.Path,
		"priority": string(priority),
	})
}

// handleDeleteProjectRoot handles DELETE /api/v1/project-roots?path=...
func (s *Server) handleDeleteProjectRoot(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "path query parameter is required"})
		return
	}

	if err := s.projectMgr.RemoveRoot(path); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "path": path})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// hermesRawStatus returns a raw status string for a Hermes session,
// without the traffic-light emoji prefix. The frontend constructs the
// display from the separate traffic_light + status fields.
func hermesRawStatus(s hermes.SessionSummary) string {
	if !s.IsGatewayTracked {
		return "inactive"
	}
	if s.IsSuspended {
		return "inactive"
	}
	return "running"
}

// ── Terminal management handlers ──────────────────────────────────

type terminalStartReq struct {
	WorkDir  string `json:"work_dir"`
	TmuxName string `json:"tmux_name,omitempty"` // optional: attach to existing tmux session
}

type terminalStopReq struct {
	Name string `json:"name"`
}

type terminalResponse struct {
	OK       bool   `json:"ok,omitempty"`
	Error    string `json:"error,omitempty"`
	Name     string `json:"name,omitempty"`
	WorkDir  string `json:"work_dir,omitempty"`
	TtydPort int    `json:"ttyd_port,omitempty"`
	TtydURL  string `json:"ttyd_url,omitempty"`
}

// handleTerminalStart handles POST /api/v1/terminal/start.
func (s *Server) handleTerminalStart(w http.ResponseWriter, r *http.Request) {
	if s.sessionMgr == nil {
		writeTerminalError(w, "terminal management not enabled", http.StatusInternalServerError)
		return
	}

	var req terminalStartReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeTerminalError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.WorkDir == "" {
		writeTerminalError(w, "work_dir is required", http.StatusBadRequest)
		return
	}

	var sess *session.Session
	var err error
	if req.TmuxName != "" {
		// Attach ttyd to an existing tmux session
		sess, err = s.sessionMgr.StartExisting(req.TmuxName, req.WorkDir)
	} else {
		sess, err = s.sessionMgr.Start(req.WorkDir)
	}
	if err != nil {
		writeTerminalError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(terminalResponse{
		OK:       true,
		Name:     sess.Name,
		WorkDir:  sess.WorkDir,
		TtydPort: sess.TtydPort,
		TtydURL:  sess.TtydURL,
	})
}

// handleTerminalStop handles POST /api/v1/terminal/stop.
func (s *Server) handleTerminalStop(w http.ResponseWriter, r *http.Request) {
	if s.sessionMgr == nil {
		writeTerminalError(w, "terminal management not enabled", http.StatusInternalServerError)
		return
	}

	var req terminalStopReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeTerminalError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		writeTerminalError(w, "name is required", http.StatusBadRequest)
		return
	}

	// Don't kill the tmux session on stop — user may want to reattach later
	if err := s.sessionMgr.Stop(req.Name, false); err != nil {
		writeTerminalError(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(terminalResponse{OK: true})
}

// handleTerminalList handles GET /api/v1/terminal/list.
func (s *Server) handleTerminalList(w http.ResponseWriter, r *http.Request) {
	if s.sessionMgr == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]terminalResponse{})
		return
	}

	sessions := s.sessionMgr.List()
	result := make([]terminalResponse, 0, len(sessions))
	for _, sess := range sessions {
		result = append(result, terminalResponse{
			Name:     sess.Name,
			WorkDir:  sess.WorkDir,
			TtydPort: sess.TtydPort,
			TtydURL:  sess.TtydURL,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// terminalLookupResponse is the response for GET /api/v1/terminal/lookup.
type terminalLookupResponse struct {
	Found    bool   `json:"found"`
	Verified bool   `json:"verified"`            // live capture-pane confirmed the prefix
	TmuxName string `json:"tmux_name,omitempty"`
	WorkDir  string `json:"work_dir,omitempty"`
	TtydPort int    `json:"ttyd_port,omitempty"`
	TtydURL  string `json:"ttyd_url,omitempty"`
	Hint     string `json:"hint,omitempty"`      // message when not found
	Warning  string `json:"warning,omitempty"`   // shown when found but not verified
}

// handleTerminalLookup handles GET /api/v1/terminal/lookup?session_id=<id>&agent_type=<type>.
// It finds a pflow-managed tmux session associated with the given agent session.
// agent_type defaults to "claude" for backward compatibility.
func (s *Server) handleTerminalLookup(w http.ResponseWriter, r *http.Request) {
	if s.sessionMgr == nil {
		writeTerminalError(w, "terminal management not enabled", http.StatusInternalServerError)
		return
	}

	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		writeTerminalError(w, "session_id query parameter is required", http.StatusBadRequest)
		return
	}

	agentType := r.URL.Query().Get("agent_type")
	if agentType == "" {
		agentType = "claude" // backward compatible default
	}

	plogger.Debugf("api: terminal lookup request session_id=%s agent_type=%s", truncate8(sessionID), agentType)

	result, err := s.sessionMgr.LookupBySessionID(agentType, sessionID)
	if err != nil {
		plogger.Warnf("api: terminal lookup error session_id=%s agent=%s: %v", truncate8(sessionID), agentType, err)
		writeTerminalError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if result.Session == nil {
		plogger.Infof("api: terminal lookup NOT FOUND session_id=%s agent=%s", truncate8(sessionID), agentType)
		json.NewEncoder(w).Encode(terminalLookupResponse{
			Found: false,
			Hint:  fmt.Sprintf("No pflow-managed tmux session found for this %s session. Start one with: pflow %s", agentType, agentType),
		})
		return
	}

	plogger.Infof("api: terminal lookup FOUND session_id=%s agent=%s tmux=%s verified=%v",
		truncate8(sessionID), agentType, result.Session.Name, result.Verified)
	resp := terminalLookupResponse{
		Found:    true,
		Verified: result.Verified,
		TmuxName: result.Session.Name,
		WorkDir:  result.Session.WorkDir,
		TtydPort: result.Session.TtydPort,
		TtydURL:  result.Session.TtydURL,
		Warning:  result.Warning,
	}

	json.NewEncoder(w).Encode(resp)
}

func truncate8(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

// computeReminderScores groups sessions by matched_root (or project dir),
// extracts per-project metrics (waiting, streak, total), and calls the
// attention score algorithm. Returns a map of project key → ReminderOutput.
func computeReminderScores(sessions []DashboardEntry, rootsFile *project.RootsFile, now time.Time) map[string]attention.ReminderOutput {
	if len(sessions) == 0 {
		return nil
	}

	// Build project root priority lookup
	primarySet := make(map[string]bool)
	if rootsFile != nil {
		for _, r := range rootsFile.Roots {
			if r.Priority == project.PriorityPrimary {
				primarySet[r.Path] = true
			}
		}
	}

	// Group sessions by project key (matched_root or project dir)
	type sessionMetrics struct {
		sessions   []DashboardEntry
		isPrimary  bool
	}
	groups := make(map[string]*sessionMetrics)
	var groupOrder []string

	for _, s := range sessions {
		key := s.MatchedRoot
		if key == "" {
			key = s.Project
		}
		if key == "" {
			continue
		}

		if _, ok := groups[key]; !ok {
			groups[key] = &sessionMetrics{isPrimary: primarySet[key]}
			groupOrder = append(groupOrder, key)
		}
		groups[key].sessions = append(groups[key].sessions, s)
	}

	// Compute per-project metrics
	plogger.Infof("[api] ======== Extracting per-project metrics ========")
	inputs := make(map[string]attention.ReminderInput, len(groups))
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	for key, gm := range groups {
		plogger.Infof("[api] project: %s (primary=%v, %d sessions)", key, gm.isPrimary, len(gm.sessions))
		var waitingMax float64
		var streakEstimate float64
		var totalToday float64
		var lastActiveTime time.Time

		for _, s := range gm.sessions {
			// waiting: max duration of waiting sessions in minutes
			var waitingMin float64
			if s.Status == "waiting" {
				waitingMin = now.Sub(s.LastActive).Minutes()
				if waitingMin > waitingMax {
					waitingMax = waitingMin
				}
			}

			// Track most recent activity
			if s.LastActive.After(lastActiveTime) {
				lastActiveTime = s.LastActive
			}

			// Estimate individual session contribution to today's total
			var sessionContribution float64
			if s.FirstActive.After(todayStart) || s.LastActive.After(todayStart) {
				start := s.FirstActive
				if start.Before(todayStart) {
					start = todayStart
				}
				end := s.LastActive
				if end.After(now) {
					end = now
				}
				if end.After(start) {
					sessionContribution = end.Sub(start).Minutes()
					totalToday += sessionContribution
				}
			}
			plogger.Infof("[api]   session %s: status=%s waiting=%.0fm contrib=%.0fm",
				truncate8(s.SessionID), s.Status, waitingMin, sessionContribution)
		}

		// Cap waiting at 120 minutes (2 hours) — beyond that the
		// session is likely abandoned rather than actively waiting.
		uncappedWaiting := waitingMax
		if waitingMax > 120 {
			waitingMax = 120
		}

		// Estimate streak (continuous focus minutes) from busy sessions.
		//
		// NOTE: streak is a lower bound — we can only measure "agent busy"
		// duration (Claude processing after user submits input). We cannot
		// observe user thinking time or prompt-writing time in the CLI.
		// When the server just started, historical streak data is missing;
		// the score algorithm applies a floor based on last_active recency
		// to avoid falsely triggering the protection period.
		hasBusy := false
		for _, s := range gm.sessions {
			if s.Status == "busy" {
				hasBusy = true
				break
			}
		}
		if hasBusy {
			// Estimate streak from the earliest busy session's first
			// activity today. The "status: busy" metadata field is
			// authoritative; last_active may be slightly stale because
			// the history file reflects the last user message, not the
			// current processing timestamp.
			var earliestBusy time.Time
			for _, s := range gm.sessions {
				if s.Status == "busy" {
					if earliestBusy.IsZero() || s.FirstActive.Before(earliestBusy) {
						earliestBusy = s.FirstActive
					}
				}
			}
			if !earliestBusy.IsZero() {
				if earliestBusy.Before(todayStart) {
					earliestBusy = todayStart
				}
				streakEstimate = now.Sub(earliestBusy).Minutes()
				// Cap at 120 minutes as reasonable max session
				if streakEstimate > 120 {
					streakEstimate = 120
				}
			} else {
				// Fallback: busy but no first_active — assume 5min minimum
				streakEstimate = 5
			}
		}

		inputs[key] = attention.ReminderInput{
			Waiting:    waitingMax,
			Streak:     streakEstimate,
			Total:      totalToday,
			LastActive: lastActiveTime,
			IsPrimary:  gm.isPrimary,
		}
		plogger.Infof("[api]   => input: waiting=%.0fm(capped from %.0fm) streak=%.0fm total=%.0fm last=%s",
			waitingMax, uncappedWaiting, streakEstimate, totalToday,
			lastActiveTime.Format("15:04:05"))
	}

	focusActive, focusedProject, focusMinutes, _ := attention.GetFocus().Snapshot()
	return attention.CalculateScores(inputs, now, focusActive, focusedProject, focusMinutes)
}

// generateSuggestions builds suggest.SessionInfo from raw scan results,
// computes project summaries, and calls suggest.Generate. Returns JSON-ready
// suggestions or an empty slice.
func generateSuggestions(
	claudeResult *claude.ScanResult,
	hermesResult *hermes.ScanResult,
	mappings []session.Mapping,
	rootsFile *project.RootsFile,
	now time.Time,
) []SuggestionJSON {
	// Build suggest.SessionInfo from raw scan results
	var sessions []suggest.SessionInfo

	if claudeResult != nil {
		for _, s := range claudeResult.Sessions {
			si := suggest.SessionInfo{
				AgentType:   "claude",
				AgentName:   s.Name,
				ProjectPath: s.Project,
				Status:      s.Status,
				LastActive:  s.LastActive,
				FirstActive: s.FirstActive,
				PID:         s.PID,
				IsRunning:   s.IsRunning,
				LastReq:     s.LastReq,
			}
			if rootsFile != nil {
				if matched := project.MatchRootFromList(rootsFile.Roots, s.Project); matched != nil {
					si.MatchedRoot = matched.Path
					si.RootPriority = string(matched.Priority)
				}
			}
			sessions = append(sessions, si)
		}
	}

	if hermesResult != nil {
		for _, s := range hermesResult.Sessions {
			// Skip weixin sessions: weixin only has two states
			// (connected→busy, disconnected→idle) and no reliable
			// way to distinguish "agent processing" from "waiting
			// for user input". Suggest analysis is meaningless for
			// weixin—it's a chatbot, not a task agent.
			if s.Platform == "weixin" {
				continue
			}
			si := suggest.SessionInfo{
				AgentType:   "hermes",
				AgentName:   s.Name,
				ProjectPath: s.Project,
				Status:      hermesStatusToSuggest(s),
				LastActive:  s.LastActive,
				FirstActive: s.FirstActive,
				PID:         0,
				IsRunning:   s.IsGatewayTracked && !s.IsSuspended,
				LastReq:     s.LastReq,
			}
			if rootsFile != nil {
				if matched := project.MatchRootFromList(rootsFile.Roots, s.Project); matched != nil {
					si.MatchedRoot = matched.Path
					si.RootPriority = string(matched.Priority)
				}
			}
			sessions = append(sessions, si)
		}
	}

	// Augment sessions with mapping PID info for dead-process detection
	for _, m := range mappings {
		if m.PID > 0 && !processAlive(m.PID) {
			for i := range sessions {
				if sessions[i].ProjectPath == m.WorkDir && sessions[i].PID == 0 {
					sessions[i].PID = m.PID
					sessions[i].IsRunning = false
				}
			}
		}
	}

	// Detect current project from tmux client
	currentProject := detectCurrentProjectFromAPI(mappings, rootsFile)

	// Compute per-project summaries
	projects := suggest.ComputeProjectSummaries(sessions, now)

	// Generate suggestions
	input := suggest.Input{
		Sessions:       sessions,
		Projects:       projects,
		CurrentProject: currentProject,
		Now:            now,
	}
	results := suggest.Generate(input)

	// Convert to JSON-friendly format
	out := make([]SuggestionJSON, 0, len(results))
	for _, r := range results {
		out = append(out, SuggestionJSON{
			Icon:     r.Icon,
			Text:     r.Text,
			Priority: r.Priority,
		})
	}
	return out
}

// detectCurrentProjectFromAPI finds which project root the user is currently
// viewing in tmux. Returns "" if no active tmux client is attached.
func detectCurrentProjectFromAPI(mappings []session.Mapping, rootsFile *project.RootsFile) string {
	if rootsFile == nil || len(mappings) == 0 {
		return ""
	}

	out, err := exec.Command("tmux", "list-clients", "-F", "#{client_session}").Output()
	if err != nil {
		return ""
	}

	tmuxNames := strings.Split(strings.TrimSpace(string(out)), "\n")
	tmuxToWorkDir := make(map[string]string, len(mappings))
	for _, m := range mappings {
		tmuxToWorkDir[m.TmuxName] = m.WorkDir
	}

	for _, name := range tmuxNames {
		if workDir, ok := tmuxToWorkDir[name]; ok {
			if matched := project.MatchRootFromList(rootsFile.Roots, workDir); matched != nil {
				return matched.Path
			}
		}
	}
	return ""
}

// hermesStatusToSuggest maps Hermes session state to suggest status.
func hermesStatusToSuggest(s hermes.SessionSummary) string {
	if !s.IsGatewayTracked || s.IsSuspended {
		return "inactive"
	}
	if s.IsActive() {
		return "busy"
	}
	return "idle"
}

// processAlive checks if a PID corresponds to a running process.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	_, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
	return err == nil
}

// ── Focus mode handlers ──────────────────────────────────────────

type focusResponse struct {
	OK      bool    `json:"ok"`
	Active  bool    `json:"active"`
	Minutes float64 `json:"minutes"`
}

// handleFocusExtend handles POST /api/v1/focus/extend.
// Activates focus mode for the given project and adds 15 minutes to the
// protection window. Each click extends the focus by 15 minutes.
// Request body: {"project": "/path/to/project"}
func (s *Server) handleFocusExtend(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Project string `json:"project"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Allow empty body for backward compatibility with global header focus button
		req.Project = ""
	}
	if req.Project == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project is required"})
		return
	}
	active, minutes := attention.GetFocus().Extend(req.Project)
	plogger.Infof("api: focus extend project=%s → active=%v minutes=%.0f", req.Project, active, minutes)
	writeJSON(w, http.StatusOK, focusResponse{OK: true, Active: active, Minutes: minutes})
}

// handleFocusStop handles POST /api/v1/focus/stop.
// Deactivates focus mode and resets the protection window.
func (s *Server) handleFocusStop(w http.ResponseWriter, r *http.Request) {
	active, minutes := attention.GetFocus().Stop()
	plogger.Infof("api: focus stop → active=%v minutes=%.0f", active, minutes)
	writeJSON(w, http.StatusOK, focusResponse{OK: true, Active: active, Minutes: minutes})
}

func writeTerminalError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(terminalResponse{Error: msg})
}

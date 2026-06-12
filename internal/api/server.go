// Package api provides the pflow HTTP API server.
package api

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strconv"
	"time"

	"github.com/pancake-lee/pflow/internal/claude"
	"github.com/pancake-lee/pflow/internal/config"
	"github.com/pancake-lee/pflow/internal/hermes"
	"github.com/pancake-lee/pflow/internal/session"
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
	LastActive       time.Time `json:"last_active"`
	LastReq          string    `json:"last_req"`          // truncated ~15 chars for table
	LastResp         string    `json:"last_resp"`         // truncated ~15 chars for table
	LastReqFull      string    `json:"last_req_full"`     // full text for detail view
	LastRespFull     string    `json:"last_resp_full"`    // full text for detail view
	Platform         string    `json:"platform,omitempty"`   // Hermes only
	HasTerminal      bool      `json:"has_terminal"`         // true if a tmux mapping exists
	TerminalTmuxName string    `json:"terminal_tmux_name,omitempty"` // matched tmux session name
}

// DashboardResponse is the JSON response for GET /api/v1/dashboard.
type DashboardResponse struct {
	Now      time.Time         `json:"now"`
	Window   string            `json:"window"`
	Sessions []DashboardEntry  `json:"sessions"`
	Errors   []string          `json:"errors,omitempty"`
}

// Server is the pflow HTTP API server.
type Server struct {
	http.ServeMux
	staticFS   fs.FS // optional embedded static files (web/dist)
	sessionMgr *session.Manager
}

// NewServer creates a new API server with registered routes.
// If staticFS is non-nil, static files (the Vue SPA) are served from it.
func NewServer(staticFS fs.FS, sessionMgr *session.Manager) *Server {
	s := &Server{staticFS: staticFS, sessionMgr: sessionMgr}
	s.HandleFunc("/api/v1/dashboard", s.handleDashboard)

	// Terminal management endpoints
	s.HandleFunc("POST /api/v1/terminal/start", s.handleTerminalStart)
	s.HandleFunc("POST /api/v1/terminal/stop", s.handleTerminalStop)
	s.HandleFunc("GET /api/v1/terminal/list", s.handleTerminalList)
	s.HandleFunc("GET /api/v1/terminal/lookup", s.handleTerminalLookup)

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

	resp := DashboardResponse{
		Now:    time.Now(),
		Window: opts.Window.String(),
	}

	var errors []string

	// Scan Claude Code sessions
	claudeResult, claudeErr := claude.Scan(opts)
	if claudeErr != nil {
		errors = append(errors, "claude: "+claudeErr.Error())
	} else {
		for _, s := range claudeResult.Sessions {
			resp.Sessions = append(resp.Sessions, DashboardEntry{
				SessionID:    truncate8(s.SessionID),
				AgentType:    "claude",
				Project:      s.Project,
				Status:       s.Status,
				IsActive:     s.IsActive(),
				TrafficLight: s.TrafficLight(),
				Name:         s.Name,
				LastActive:   s.LastActive,
				LastReq:      s.LastReq,
				LastResp:     s.LastResp,
				LastReqFull:  s.LastReqFull,
				LastRespFull: s.LastRespFull,
			})
		}
	}

	// Scan Hermes sessions
	hermesResult, hermesErr := hermes.Scan(opts)
	if hermesErr != nil {
		errors = append(errors, "hermes: "+hermesErr.Error())
	} else {
		for _, s := range hermesResult.Sessions {
			resp.Sessions = append(resp.Sessions, DashboardEntry{
				SessionID:    truncate8(s.SessionID),
				AgentType:    "hermes",
				Project:      s.Project,
				Status:       hermesRawStatus(s),
				IsActive:     s.IsActive(),
				TrafficLight: s.TrafficLight(),
				Name:         s.Name,
				LastActive:   s.LastActive,
				LastReq:      s.LastReq,
				LastResp:     s.LastResp,
				LastReqFull:  s.LastReqFull,
				LastRespFull: s.LastRespFull,
				Platform:     s.Platform,
			})
		}
	}

	// Annotate Claude sessions with terminal mapping info.
	if mappings, err := session.LoadMappings(); err == nil && len(mappings) > 0 {
		prefixToTmux := make(map[string]string, len(mappings))
		for _, m := range mappings {
			// Only include mappings with live tmux sessions
			if m.ClaudePrefix != "" {
				prefixToTmux[m.ClaudePrefix] = m.TmuxName
			}
		}
		for i := range resp.Sessions {
			if resp.Sessions[i].AgentType == "claude" {
				if tmuxName, ok := prefixToTmux[resp.Sessions[i].SessionID]; ok {
					resp.Sessions[i].HasTerminal = true
					resp.Sessions[i].TerminalTmuxName = tmuxName
				}
			}
		}
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
		Window:      config.DefaultWindow,
		MaxInactive: config.DefaultMaxInactive,
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

	return opts
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

// handleTerminalLookup handles GET /api/v1/terminal/lookup?session_id=<claude-session-id>.
// It finds a pflow-managed tmux session associated with the given Claude session.
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

	plogger.Debugf("api: terminal lookup request session_id=%s", truncate8(sessionID))
	result, err := s.sessionMgr.LookupByClaudeSessionID(sessionID)
	if err != nil {
		plogger.Warnf("api: terminal lookup error session_id=%s: %v", truncate8(sessionID), err)
		writeTerminalError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if result.Session == nil {
		plogger.Infof("api: terminal lookup NOT FOUND session_id=%s", truncate8(sessionID))
		json.NewEncoder(w).Encode(terminalLookupResponse{
			Found: false,
			Hint:  "No pflow-managed tmux session found for this Claude session. Start one with: pflow claude",
		})
		return
	}

	plogger.Infof("api: terminal lookup FOUND session_id=%s tmux=%s verified=%v",
		truncate8(sessionID), result.Session.Name, result.Verified)
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

func writeTerminalError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(terminalResponse{Error: msg})
}

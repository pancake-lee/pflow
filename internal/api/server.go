// Package api provides the pflow HTTP API server.
package api

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pancake-lee/pflow/internal/claude"
	"github.com/pancake-lee/pflow/internal/config"
	"github.com/pancake-lee/pflow/internal/hermes"
)

// DashboardEntry is a unified session entry for the Dashboard API response.
type DashboardEntry struct {
	SessionID    string    `json:"session_id"`
	AgentType    string    `json:"agent_type"` // "claude" or "hermes"
	Project      string    `json:"project"`
	Status       string    `json:"status"`
	IsActive     bool      `json:"is_active"`
	TrafficLight string    `json:"traffic_light"`
	Name         string    `json:"name"`
	LastActive   time.Time `json:"last_active"`
	LastReq      string    `json:"last_req"`       // truncated ~15 chars for table
	LastResp     string    `json:"last_resp"`       // truncated ~15 chars for table
	LastReqFull  string    `json:"last_req_full"`   // full text for detail view
	LastRespFull string    `json:"last_resp_full"`  // full text for detail view
	Platform     string    `json:"platform,omitempty"` // Hermes only

	// Managed session fields (only populated for pflow-managed sessions)
	IsManaged         bool              `json:"is_managed"`
	PendingPermission *PendingPermission `json:"pending_permission,omitempty"`
}

// PermissionInfo is the API-facing permission request info (alias for JSON).
type PermissionInfo = PendingPermission

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
	staticFS  fs.FS              // optional embedded static files (web/dist)
	manager   *SessionManager    // managed Claude sessions (may be nil)
	extPerms  *ExternalPermStore // external permission requests (may be nil)
}

// NewServer creates a new API server with registered routes.
// If staticFS is non-nil, static files (the Vue SPA) are served from it.
// If manager is non-nil, session management endpoints are registered.
func NewServer(staticFS fs.FS, manager *SessionManager) *Server {
	s := &Server{staticFS: staticFS, manager: manager, extPerms: NewExternalPermStore()}
	s.HandleFunc("/api/v1/dashboard", s.handleDashboard)

	// Session management endpoints (only when manager is available)
	if manager != nil {
		s.HandleFunc("/api/v1/sessions/start", s.handleSessionStart)
		s.HandleFunc("/api/v1/sessions/", s.handleSessionAction)
	}

	// External permission endpoints (for pflow start CLI)
	s.HandleFunc("/api/v1/extperm/", s.handleExtPerm)

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
				SessionID:    s.SessionID,
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
				SessionID:    s.SessionID,
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

	if len(errors) > 0 {
		resp.Errors = errors
	}

	// Append pflow-managed sessions (from SessionManager)
	if s.manager != nil {
		for _, snap := range s.manager.ListSnapshots() {
			lastReq := snap.LastReq
			if len(lastReq) > 15 {
				lastReq = string([]rune(lastReq)[:15])
			}
			lastResp := snap.LastResp
			if len(lastResp) > 15 {
				lastResp = string([]rune(lastResp)[:15])
			}
			entry := DashboardEntry{
				SessionID:    snap.SessionID,
				AgentType:    "claude",
				Project:      snap.Project,
				Status:       "running",
				IsActive:     true,
				TrafficLight: "🟢",
				Name:         snap.Project,
				LastActive:   snap.StartedAt,
				LastReq:      lastReq,
				LastResp:     lastResp,
				LastReqFull:  snap.LastReq,
				LastRespFull: snap.LastResp,
				IsManaged:    true,
			}
			if snap.PendingPerm != nil {
				entry.Status = "waiting"
				entry.TrafficLight = "🟡"
				entry.PendingPermission = snap.PendingPerm
			}
			resp.Sessions = append(resp.Sessions, entry)
		}
	}

	// Append external permission requests (from pflow start CLI)
	for _, ep := range s.extPerms.ListPending() {
		lastReq := ep.Perm.ToolInput
		if len(lastReq) > 15 {
			lastReq = string([]rune(lastReq)[:15])
		}
		resp.Sessions = append(resp.Sessions, DashboardEntry{
			SessionID:    ep.SessionID,
			AgentType:    "claude",
			Project:      ep.Project,
			Status:       "waiting",
			IsActive:     true,
			TrafficLight: "🟡",
			Name:         ep.Perm.ToolName,
			LastActive:   ep.CreatedAt,
			LastReq:      lastReq,
			LastReqFull:  ep.Perm.ToolInput,
			IsManaged:    false,
			PendingPermission: &PendingPermission{
				RequestID:    ep.Perm.RequestID,
				ToolName:     ep.Perm.ToolName,
				ToolInput:    ep.Perm.ToolInput,
				ToolInputRaw: ep.Perm.ToolInputRaw,
			},
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleSessionStart handles POST /api/v1/sessions/start.
func (s *Server) handleSessionStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Project string `json:"project"`
		Prompt  string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Project == "" {
		http.Error(w, "missing \"project\" field", http.StatusBadRequest)
		return
	}

	snap, err := s.manager.Start(r.Context(), req.Project, req.Prompt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"session_id": snap.SessionID,
		"project":    snap.Project,
		"started_at": snap.StartedAt,
	})
}

// handleSessionAction routes /api/v1/sessions/{id}/{action} to the appropriate handler.
func (s *Server) handleSessionAction(w http.ResponseWriter, r *http.Request) {
	// Path: /api/v1/sessions/{id}/send  or  /api/v1/sessions/{id}/permission
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/sessions/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		http.Error(w, "invalid path; expected /api/v1/sessions/{id}/{action}", http.StatusBadRequest)
		return
	}
	sessionID, action := parts[0], parts[1]

	switch action {
	case "send":
		s.handleSessionSend(w, r, sessionID)
	case "permission":
		s.handlePermission(w, r, sessionID)
	default:
		http.Error(w, "unknown action: "+action, http.StatusNotFound)
	}
}

// handleSessionSend handles POST /api/v1/sessions/{id}/send.
func (s *Server) handleSessionSend(w http.ResponseWriter, r *http.Request, sessionID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Prompt == "" {
		http.Error(w, "missing \"prompt\" field", http.StatusBadRequest)
		return
	}

	if err := s.manager.Send(sessionID, req.Prompt); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handlePermission handles POST /api/v1/sessions/{id}/permission.
func (s *Server) handlePermission(w http.ResponseWriter, r *http.Request, sessionID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		RequestID string `json:"request_id"`
		Behavior  string `json:"behavior"` // "allow" or "deny"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	var err error
	switch req.Behavior {
	case "allow":
		err = s.manager.Approve(sessionID, req.RequestID)
		if err != nil {
			// Try external permission store (from pflow start CLI)
			if s.extPerms.SetDecisionByRequestID(req.RequestID, "allow") {
				err = nil
			}
		}
	case "deny":
		err = s.manager.Deny(sessionID, req.RequestID)
		if err != nil {
			if s.extPerms.SetDecisionByRequestID(req.RequestID, "deny") {
				err = nil
			}
		}
	default:
		http.Error(w, "behavior must be \"allow\" or \"deny\"", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleExtPerm handles /api/v1/extperm/... for external permission requests.
// POST /api/v1/extperm/register  — register a new permission request
// GET  /api/v1/extperm/{request_id} — poll for decision
func (s *Server) handleExtPerm(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/extperm/")

	if path == "register" && r.Method == http.MethodPost {
		s.handleExtPermRegister(w, r)
		return
	}

	// GET /api/v1/extperm/{request_id} — poll for decision
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 1 && parts[0] != "" && r.Method == http.MethodGet {
		s.handleExtPermPoll(w, r, parts[0])
		return
	}

	http.Error(w, "not found", http.StatusNotFound)
}

func (s *Server) handleExtPermRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string           `json:"session_id"`
		Project   string           `json:"project"`
		Perm      PendingPermission `json:"permission"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.SessionID == "" || req.Perm.RequestID == "" {
		http.Error(w, "missing session_id or permission.request_id", http.StatusBadRequest)
		return
	}

	s.extPerms.Register(req.SessionID, req.Project, req.Perm)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleExtPermPoll(w http.ResponseWriter, r *http.Request, requestID string) {
	decision, found := s.extPerms.GetDecision(requestID)
	if !found {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"request_id": requestID,
		"decision":   decision,
		"pending":    decision == "",
	})
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

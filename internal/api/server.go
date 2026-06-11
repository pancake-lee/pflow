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
	staticFS fs.FS // optional embedded static files (web/dist)
}

// NewServer creates a new API server with registered routes.
// If staticFS is non-nil, static files (the Vue SPA) are served from it.
func NewServer(staticFS fs.FS) *Server {
	s := &Server{staticFS: staticFS}
	s.HandleFunc("/api/v1/dashboard", s.handleDashboard)

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

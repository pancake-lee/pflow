package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/pancake-lee/pflow"
	"github.com/pancake-lee/pflow/internal/api"
	"github.com/pancake-lee/pflow/internal/claude"
	"github.com/pancake-lee/pflow/internal/config"
	"github.com/pancake-lee/pflow/internal/hermes"
)

func main() {
	if len(os.Args) < 2 {
		// Default: status with defaults
		runStatus(config.DefaultWindow, config.DefaultMaxInactive)
		return
	}

	switch os.Args[1] {
	case "status":
		runStatusCmd(os.Args[2:])
	case "probe":
		runProbeCmd(os.Args[2:])
	case "serve":
		runServeCmd(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "pflow: unknown command %q\n", os.Args[1])
		fmt.Fprintf(os.Stderr, "Run 'pflow help' for usage.\n")
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`pflow — Multi-Agent Attention Manager

Usage:
  pflow [command] [flags]

Commands:
  status   Show agent activity dashboard (default if no command given)
  probe    Probe a single session's detailed state
  serve    Start HTTP Dashboard API server
  help     Show this help

Run 'pflow <command> -h' for detailed flags.`)
}

// ── status ──────────────────────────────────────────────────────────

func runStatusCmd(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	windowStr := fs.String("window", "1d", "Time window (e.g. 1h, 3h, 1d, 2d)")
	maxInactive := fs.Int("max-inactive", 0, "Max inactive (unknown/completed) sessions per project (0=all)")
	fs.Parse(args)

	window, err := config.ParseWindow(*windowStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pflow: invalid window: %v\n", err)
		os.Exit(1)
	}

	runStatus(window, *maxInactive)
}

func runStatus(window time.Duration, maxInactive int) {
	opts := config.ScanOptions{Window: window, MaxInactive: maxInactive}

	// Scan Claude Code sessions
	claudeResult, claudeErr := claude.Scan(opts)
	// Scan Hermes sessions
	hermesResult, hermesErr := hermes.Scan(opts)

	// Handle total failure
	if claudeErr != nil && hermesErr != nil {
		fmt.Fprintf(os.Stderr, "pflow: claude: %v\n", claudeErr)
		fmt.Fprintf(os.Stderr, "pflow: hermes: %v\n", hermesErr)
		os.Exit(1)
	}

	// Print header
	fmt.Printf("pflow — Agent Activity Dashboard (last %s)\n", window)
	fmt.Printf("Now: %s\n\n", time.Now().Format(time.DateTime))

	totalSessions := 0

	// ── Claude Code ──────────────────────────────────────────────
	if claudeErr == nil && len(claudeResult.Sessions) > 0 {
		fmt.Println("── Claude Code ──────────────────────────────────────────────────────")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "SESSION ID\tPROJECT\tSTATUS\tNAME\tLAST ACTIVE\tLAST REQ\tLAST RESP")

		for _, s := range claudeResult.Sessions {
			status := s.StatusLabel()
			project := s.Project
			if project == "" {
				project = "?"
			}

			name := escapeNewlines(truncate(s.Name, 15))
			if name == "" {
				name = "—"
			}

			lastReq := escapeNewlines(truncate(s.LastReq, 15))
			if lastReq == "" {
				lastReq = "—"
			}
			// For busy sessions, the last assistant response in the transcript
			// is from a *previous* turn — clear it to avoid showing mismatched pairs.
			lastResp := ""
			if s.Status != "busy" {
				lastResp = escapeNewlines(truncate(s.LastResp, 15))
			}
			if lastResp == "" {
				if s.Status == "busy" {
					lastResp = "…" // assistant is processing current request
				} else {
					lastResp = "—"
				}
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				shortID(s.SessionID, 16),
				project,
				status,
				name,
				formatSince(s.LastActive),
				lastReq,
				lastResp,
			)
		}
		w.Flush()
		totalSessions += len(claudeResult.Sessions)
		fmt.Println()
	} else if claudeErr != nil {
		fmt.Printf("── Claude Code: %v ──────────────────────────────\n\n", claudeErr)
	} else {
		fmt.Println("── Claude Code: no recent sessions ─────────────────────────────────")
	}

	// ── Hermes Agent ──────────────────────────────────────────────
	if hermesErr == nil && len(hermesResult.Sessions) > 0 {
		fmt.Println("── Hermes Agent ─────────────────────────────────────────────────────")
		if hermesResult.GatewayAlive {
			fmt.Println("Gateway: ✓ running")
		}
		fmt.Println()
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "SESSION ID\tPROJECT\tSTATUS\tNAME\tLAST ACTIVE\tLAST REQ\tLAST RESP")

		for _, s := range hermesResult.Sessions {
			project := s.Project
			if project == "" {
				project = s.Platform
			}
			if s.ChatType != "" && s.ChatType != "dm" {
				project += "/" + s.ChatType
			}

			name := escapeNewlines(truncate(s.Name, 15))
			if name == "" {
				name = "—"
			}

			lastReq := escapeNewlines(truncate(s.LastReq, 15))
			if lastReq == "" {
				lastReq = "—"
			}
			lastResp := escapeNewlines(truncate(s.LastResp, 15))
			if lastResp == "" {
				lastResp = "—"
			}

			fmt.Fprintf(w, "%s\t%s %s\t%s\t%s\t%s\t%s\t%s\n",
				s.ShortID(),
				s.PlatformIcon(),
				project,
				s.StatusLabel(),
				name,
				formatSince(s.LastActive),
				lastReq,
				lastResp,
			)
		}
		w.Flush()
		totalSessions += len(hermesResult.Sessions)
		fmt.Println()
	} else if hermesErr != nil {
		fmt.Printf("── Hermes Agent: %v ─────────────────────────────────\n\n", hermesErr)
	} else {
		fmt.Println("── Hermes Agent: no recent sessions ─────────────────────────────────")
	}

	// Footer
	fmt.Println(strings.Repeat("─", 70))
	fmt.Printf("%d sessions\n", totalSessions)
	fmt.Println("🟢 busy  🟡 waiting  ⚪ idle  ⚫ completed/unknown")
	if maxInactive > 0 {
		fmt.Printf("(inactive limited to %d per project)\n", maxInactive)
	}
}

// ── probe ───────────────────────────────────────────────────────────

func runProbeCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "pflow probe: missing session ID")
		fmt.Fprintln(os.Stderr, "Usage: pflow probe <session-id>")
		os.Exit(1)
	}
	sessionID := args[0]

	// Scan both agents with a wide window to find the session
	opts := config.ScanOptions{Window: 30 * 24 * time.Hour, MaxInactive: 0}

	found := false

	// Check Claude Code
	claudeResult, _ := claude.Scan(opts)
	for _, s := range claudeResult.Sessions {
		if s.SessionID == sessionID || strings.HasPrefix(s.SessionID, sessionID) {
			printClaudeProbe(s)
			found = true
		}
	}

	// Check Hermes
	hermesResult, _ := hermes.Scan(opts)
	for _, s := range hermesResult.Sessions {
		if s.SessionID == sessionID || strings.HasPrefix(s.SessionID, sessionID) {
			printHermesProbe(s, hermesResult.GatewayAlive)
			found = true
		}
	}

	if !found {
		fmt.Printf("Session %q not found.\n", sessionID)
		os.Exit(1)
	}
}

func printClaudeProbe(s claude.SessionSummary) {
	fmt.Println("── Claude Code Session ──────────────────────────────────────────────")
	fmt.Printf("  Session ID:  %s\n", s.SessionID)
	fmt.Printf("  Project:     %s\n", s.Project)
	fmt.Printf("  Status:      %s\n", s.StatusLabel())
	fmt.Printf("  PID:         %d", s.PID)
	if s.IsRunning {
		fmt.Print(" (alive)")
	} else if s.PID > 0 {
		fmt.Print(" (dead)")
	}
	fmt.Println()
	fmt.Printf("  Name:        %s\n", s.Name)
	fmt.Printf("  Messages:    %d (in window)\n", s.MessageCount)
	fmt.Printf("  First:       %s\n", formatTime(s.FirstActive))
	fmt.Printf("  Last:        %s (%s)\n", formatTime(s.LastActive), formatSince(s.LastActive))
	fmt.Printf("  Last Req:    %s\n", s.LastReq)
	fmt.Printf("  Last Resp:   %s\n", s.LastResp)
	fmt.Println()
}

func printHermesProbe(s hermes.SessionSummary, gatewayAlive bool) {
	fmt.Println("── Hermes Session ───────────────────────────────────────────────────")
	fmt.Printf("  Session ID:  %s\n", s.SessionID)
	fmt.Printf("  Platform:    %s %s\n", s.PlatformIcon(), s.Platform)
	fmt.Printf("  Project:     %s\n", s.Project)
	fmt.Printf("  Status:      %s (gateway: %v)\n", s.StatusLabel(), gatewayAlive)
	fmt.Printf("  Name:        %s\n", s.Name)
	if s.DisplayName != "" && s.DisplayName != s.Name {
		fmt.Printf("  Display:     %s\n", s.DisplayName)
	}
	fmt.Printf("  ChatType:    %s\n", s.ChatType)
	fmt.Printf("  Tokens:      in=%d out=%d total=%d\n", s.InputTokens, s.OutputTokens, s.InputTokens+s.OutputTokens)
	fmt.Printf("  First:       %s\n", formatTime(s.FirstActive))
	fmt.Printf("  Last:        %s (%s)\n", formatTime(s.LastActive), formatSince(s.LastActive))
	fmt.Printf("  Last Req:    %s\n", s.LastReq)
	fmt.Printf("  Last Resp:   %s\n", s.LastResp)
	fmt.Println()
}

// ── serve ────────────────────────────────────────────────────────────

func runServeCmd(args []string) {
	flagSet := flag.NewFlagSet("serve", flag.ExitOnError)
	port := flagSet.Int("port", 8080, "HTTP server port")
	flagSet.Parse(args)

	// Pass the embedded Vue SPA filesystem. If the web/dist directory
	// is not embedded (e.g. during development), pass nil to serve API only.
	var staticFS fs.FS = pflow.WebDist
	server := api.NewServer(staticFS)

	addr := fmt.Sprintf(":%d", *port)
	fmt.Printf("pflow server listening on http://localhost%s\n", addr)
	fmt.Printf("Endpoints:\n")
	fmt.Printf("  Web UI:  http://localhost%s/\n", addr)
	fmt.Printf("  API:     GET /api/v1/dashboard?window=1d&max_inactive=1\n")

	// Graceful shutdown on SIGINT/SIGTERM
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		fmt.Println("\nShutting down...")
		os.Exit(0)
	}()

	if err := http.ListenAndServe(addr, server); err != nil {
		log.Fatalf("server: %v", err)
	}
}

// ── formatting helpers ──────────────────────────────────────────────

func shortID(id string, n int) string {
	if len(id) <= n {
		return id
	}
	return id[:n]
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

// escapeNewlines replaces control characters with visible escape sequences
// so they don't break the table layout.
func escapeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", `\r\n`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}

func formatSince(t time.Time) string {
	if t.IsZero() {
		return "n/a"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		hours := int(d.Hours())
		mins := int(d.Minutes()) % 60
		if hours == 1 && mins == 0 {
			return "1h ago"
		}
		return fmt.Sprintf("%dh %dm ago", hours, mins)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1d ago"
		}
		return fmt.Sprintf("%dd ago", days)
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "n/a"
	}
	return t.Format(time.DateTime)
}

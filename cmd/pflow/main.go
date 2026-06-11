package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/pancake-lee/pflow/internal/claude"
	"github.com/pancake-lee/pflow/internal/hermes"
)

func main() {
	window := 24 * time.Hour

	// Scan Claude Code sessions
	claudeResult, claudeErr := claude.Scan(window)
	// Scan Hermes sessions
	hermesResult, hermesErr := hermes.Scan(window)

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
			status := formatClaudeStatus(s)
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
		fmt.Println("── Claude Code: no active sessions ─────────────────────────────────")
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
		fmt.Println("── Hermes Agent: no active sessions ─────────────────────────────────")
	}

	// Footer
	fmt.Println(strings.Repeat("─", 70))
	fmt.Printf("%d active sessions total\n", totalSessions)
	fmt.Println("🟢 busy  🟡 waiting  ⚪ idle  ▶ running  ⏸ suspended")
}

func formatClaudeStatus(s claude.SessionSummary) string {
	icon := "?"
	switch s.Status {
	case "busy":
		icon = "🟢 busy"
	case "waiting":
		icon = "🟡 waiting"
	case "idle":
		icon = "⚪ idle"
	default:
		icon = "? " + s.Status
	}
	if s.IsRunning {
		icon += " ●"
	}
	return icon
}

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

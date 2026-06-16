// Package config provides shared configuration types and parsing utilities for pflow.
package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// DefaultWindow is the default time window for scanning agent activity.
const DefaultWindow = 24 * time.Hour

// DefaultMaxInactive is the default max number of inactive sessions to show per project.
// 0 means no limit (show all).
const DefaultMaxInactive = 0

// DefaultHermesSourceFilter is the default source filter for hermes sessions.
// Excludes cron sessions by default; use empty string to show all.
const DefaultHermesSourceFilter = "cli,weixin"

// ScanOptions configures a scan for agent activity.
type ScanOptions struct {
	// Window is the time window for filtering sessions by last activity.
	// Sessions not active within this window are excluded.
	Window time.Duration

	// MaxInactive limits how many inactive (unknown, completed, etc.) sessions
	// are shown per project. Active sessions are always shown in full.
	// 0 means no limit.
	MaxInactive int

	// SourceFilter is a comma-separated list of source types to include.
	// Empty means all sources. Example: "cli,weixin" excludes cron.
	// Known hermes sources: cli, cron, weixin.
	SourceFilter string
}

// ParseWindow parses a human-readable time window string like "1h", "3h", "1d",
// "30m", "90s", "2d6h", etc. It is more flexible than time.ParseDuration,
// supporting "d" (day) and "w" (week) suffixes.
func ParseWindow(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return DefaultWindow, nil
	}

	// Special case: bare number defaults to hours
	if d, err := strconv.Atoi(s); err == nil {
		return time.Duration(d) * time.Hour, nil
	}

	// Try standard Go duration parsing first (handles "1h30m", "30s", etc.)
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}

	// Custom parsing for "d" (day) and "w" (week)
	var total time.Duration
	remainder := s

	for remainder != "" {
		// Find the next digit
		digitStart := strings.IndexFunc(remainder, func(r rune) bool {
			return r >= '0' && r <= '9'
		})
		if digitStart < 0 {
			break
		}
		remainder = remainder[digitStart:]

		// Read the number
		numEnd := strings.IndexFunc(remainder, func(r rune) bool {
			return r < '0' || r > '9'
		})
		var numStr string
		if numEnd < 0 {
			numStr = remainder
			remainder = ""
		} else {
			numStr = remainder[:numEnd]
			remainder = remainder[numEnd:]
		}

		num, err := strconv.Atoi(numStr)
		if err != nil {
			return 0, fmt.Errorf("invalid number %q in window %q", numStr, s)
		}

		// Read the suffix
		if remainder == "" {
			// Bare number without suffix: treat as hours
			total += time.Duration(num) * time.Hour
			break
		}

		suffix := remainder[0]
		remainder = remainder[1:]

		switch suffix {
		case 's':
			total += time.Duration(num) * time.Second
		case 'm':
			total += time.Duration(num) * time.Minute
		case 'h':
			total += time.Duration(num) * time.Hour
		case 'd':
			total += time.Duration(num) * 24 * time.Hour
		case 'w':
			total += time.Duration(num) * 7 * 24 * time.Hour
		default:
			return 0, fmt.Errorf("unknown suffix %q in window %q (use s/m/h/d/w)", string(suffix), s)
		}
	}

	return total, nil
}

package claude

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

// StartOptions configures how a Claude Code subprocess is launched.
type StartOptions struct {
	// Args are additional arguments passed to the claude CLI (e.g. "--resume", sessionID).
	Args []string
	// SessionID, if set, resumes an existing session.
	SessionID string
}

// Client manages a Claude Code CLI subprocess communicating via stream-json over stdio.
//
// Typical usage:
//
//	client, err := claude.Start(ctx, claude.StartOptions{})
//	if err != nil { ... }
//	defer client.Close()
//
//	// Read events
//	for ev := range client.Events() {
//	    tracker.Process(ev)
//	}
type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	events <-chan Event
	cancel context.CancelFunc
}

// Start launches a Claude Code CLI subprocess with stream-json I/O.
//
// The command is equivalent to:
//
//	claude --output-format stream-json --input-format stream-json --permission-prompt-tool stdio [args...]
func Start(ctx context.Context, opts StartOptions) (*Client, error) {
	args := []string{
		"--output-format", "stream-json",
		"--input-format", "stream-json",
		"--permission-prompt-tool", "stdio",
	}
	if opts.SessionID != "" {
		args = append(args, "--resume", opts.SessionID)
	}
	args = append(args, opts.Args...)

	cmd := exec.CommandContext(ctx, "claude", args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("claude stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("claude stdout pipe: %w", err)
	}

	// Stderr is ignored for now; Claude Code writes status messages there
	// but event data goes to stdout.

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("starting claude: %w", err)
	}

	// Begin parsing events from stdout immediately
	events := ParseEvents(stdout)

	return &Client{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		events: events,
	}, nil
}

// Events returns a channel of parsed stream-json events from Claude Code's stdout.
// The channel is closed when the subprocess exits or the context is canceled.
func (c *Client) Events() <-chan Event {
	return c.events
}

// Send writes a JSON message to Claude Code's stdin. The message is sent as a
// stream-json line.
//
// For user messages, use:
//
//	client.Send(claude.UserMessage{Role: "user", Content: "your message"})
func (c *Client) Send(v any) error {
	return writeJSON(c.stdin, v)
}

// Close terminates the Claude Code subprocess. It sends a cancellation signal
// and waits for the process to exit.
func (c *Client) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	// Close stdin to signal end of input
	c.stdin.Close()
	// Wait for the process to exit
	return c.cmd.Wait()
}

// PID returns the process ID of the Claude Code subprocess, or 0 if not started.
func (c *Client) PID() int {
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Pid
	}
	return 0
}

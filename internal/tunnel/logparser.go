package tunnel

import (
	"bufio"
	"io"
	"regexp"
	"strings"
)

var hostnamePattern = regexp.MustCompile(`https?://([a-zA-Z0-9-]+\.tunnel\.gamwich\.app)`)

// parseEvent represents a parsed cloudflared log event.
type parseEvent struct {
	state    State
	hostname string
}

// parseLogLine examines a single cloudflared stderr line and returns a
// parseEvent if the line indicates a state change, or nil otherwise.
func parseLogLine(line string) *parseEvent {
	switch {
	case strings.Contains(line, "Registered tunnel connection") ||
		strings.Contains(line, "registered connIndex="):
		ev := &parseEvent{state: StateConnected}
		if m := hostnamePattern.FindStringSubmatch(line); len(m) > 1 {
			ev.hostname = m[1]
		}
		return ev

	case strings.Contains(line, "Unregistered tunnel connection"):
		return &parseEvent{state: StateReconnecting}

	case strings.Contains(line, "failed to connect"):
		return &parseEvent{state: StateReconnecting}

	default:
		// Check for hostname in INF lines
		if strings.Contains(line, "INF") {
			if m := hostnamePattern.FindStringSubmatch(line); len(m) > 1 {
				return &parseEvent{state: StateConnected, hostname: m[1]}
			}
		}
		return nil
	}
}

// scanLogs reads lines from r and sends parseEvents to the channel.
// It blocks until r returns an error (typically io.EOF when the process exits).
func scanLogs(r io.Reader, events chan<- parseEvent) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if ev := parseLogLine(scanner.Text()); ev != nil {
			events <- *ev
		}
	}
}

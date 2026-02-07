package tunnel

import (
	"log/slog"
	"strings"
	"testing"
)

func TestNewManager_EmptyToken(t *testing.T) {
	m := NewManager(Config{}, nil, slog.Default())
	if s := m.Status(); s.State != StateDisabled {
		t.Errorf("expected StateDisabled, got %s", s.State)
	}
}

func TestNewManager_TokenDisabled(t *testing.T) {
	m := NewManager(Config{Token: "test-token", Enabled: false}, nil, slog.Default())
	if s := m.Status(); s.State != StateStopped {
		t.Errorf("expected StateStopped, got %s", s.State)
	}
}

func TestNewManager_DefaultCloudflaredPath(t *testing.T) {
	m := NewManager(Config{Token: "test-token"}, nil, slog.Default())
	if m.cfg.CloudflaredPath != "cloudflared" {
		t.Errorf("expected default cloudflaredPath 'cloudflared', got %q", m.cfg.CloudflaredPath)
	}
}

func TestStatusCallback_Invoked(t *testing.T) {
	var called bool
	var gotStatus Status
	cb := func(s Status) {
		called = true
		gotStatus = s
	}

	m := NewManager(Config{Token: "test-token", Enabled: true}, cb, slog.Default())
	m.mu.Lock()
	m.setState(Status{State: StateConnecting})
	m.mu.Unlock()

	if !called {
		t.Error("expected callback to be called")
	}
	if gotStatus.State != StateConnecting {
		t.Errorf("expected StateConnecting, got %s", gotStatus.State)
	}
}

func TestStop_NoOp_WhenNotRunning(t *testing.T) {
	m := NewManager(Config{Token: "test-token", Enabled: false}, nil, slog.Default())
	// Should not panic or block
	m.Stop()
	if s := m.Status(); s.State != StateStopped {
		t.Errorf("expected StateStopped, got %s", s.State)
	}
}

func TestUpdateConfig_DisableStops(t *testing.T) {
	var lastState State
	cb := func(s Status) { lastState = s.State }

	m := NewManager(Config{Token: "test-token", Enabled: true}, cb, slog.Default())
	m.UpdateConfig(Config{Token: "test-token", Enabled: false})
	if lastState != StateStopped {
		t.Errorf("expected StateStopped, got %s", lastState)
	}
}

func TestUpdateConfig_EmptyTokenDisables(t *testing.T) {
	var lastState State
	cb := func(s Status) { lastState = s.State }

	m := NewManager(Config{Token: "test-token", Enabled: true}, cb, slog.Default())
	m.UpdateConfig(Config{Token: "", Enabled: true})
	if lastState != StateDisabled {
		t.Errorf("expected StateDisabled, got %s", lastState)
	}
}

func TestParseLogLine_Registered(t *testing.T) {
	line := `2024-01-01T00:00:00Z INF registered connIndex=0 connection=abc123`
	ev := parseLogLine(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.state != StateConnected {
		t.Errorf("expected StateConnected, got %s", ev.state)
	}
}

func TestParseLogLine_Unregistered(t *testing.T) {
	line := `2024-01-01T00:00:00Z INF Unregistered tunnel connection connIndex=0`
	ev := parseLogLine(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.state != StateReconnecting {
		t.Errorf("expected StateReconnecting, got %s", ev.state)
	}
}

func TestParseLogLine_HostnameExtraction(t *testing.T) {
	line := `2024-01-01T00:00:00Z INF | https://my-house.tunnel.gamwich.app`
	ev := parseLogLine(line)
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.hostname != "my-house.tunnel.gamwich.app" {
		t.Errorf("expected hostname 'my-house.tunnel.gamwich.app', got %q", ev.hostname)
	}
}

func TestParseLogLine_NoMatch(t *testing.T) {
	line := `2024-01-01T00:00:00Z DBG some debug message`
	ev := parseLogLine(line)
	if ev != nil {
		t.Errorf("expected nil, got %+v", ev)
	}
}

func TestScanLogs(t *testing.T) {
	input := strings.NewReader(
		"2024-01-01T00:00:00Z INF registered connIndex=0\n" +
			"2024-01-01T00:00:01Z DBG something\n" +
			"2024-01-01T00:00:02Z INF Unregistered tunnel connection\n",
	)

	events := make(chan parseEvent, 10)
	scanLogs(input, events)
	close(events)

	var collected []parseEvent
	for ev := range events {
		collected = append(collected, ev)
	}

	if len(collected) != 2 {
		t.Fatalf("expected 2 events, got %d", len(collected))
	}
	if collected[0].state != StateConnected {
		t.Errorf("event 0: expected StateConnected, got %s", collected[0].state)
	}
	if collected[1].state != StateReconnecting {
		t.Errorf("event 1: expected StateReconnecting, got %s", collected[1].state)
	}
}

package tunnel

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"sync"
	"time"
)

// State represents the tunnel connection state.
type State string

const (
	StateDisabled     State = "disabled"
	StateStopped      State = "stopped"
	StateConnecting   State = "connecting"
	StateConnected    State = "connected"
	StateReconnecting State = "reconnecting"
	StateError        State = "error"
)

// Config holds the tunnel configuration.
type Config struct {
	Token           string
	Enabled         bool
	LocalURL        string
	CloudflaredPath string
}

// Status holds the current tunnel status.
type Status struct {
	State     State     `json:"state"`
	Subdomain string    `json:"subdomain,omitempty"`
	Error     string    `json:"error,omitempty"`
	StartedAt time.Time `json:"started_at,omitempty"`
}

// StatusCallback is called whenever the tunnel state changes.
type StatusCallback func(Status)

// Manager manages a cloudflared tunnel subprocess.
type Manager struct {
	mu       sync.RWMutex
	cfg      Config
	status   Status
	callback StatusCallback

	cancel       context.CancelFunc
	done         chan struct{}
	failureCount int
}

// NewManager creates a new tunnel manager.
func NewManager(cfg Config, cb StatusCallback) *Manager {
	m := &Manager{
		cfg:      cfg,
		callback: cb,
	}

	if cfg.Token == "" {
		m.status = Status{State: StateDisabled}
	} else if !cfg.Enabled {
		m.status = Status{State: StateStopped}
	} else {
		m.status = Status{State: StateStopped}
	}

	if m.cfg.CloudflaredPath == "" {
		m.cfg.CloudflaredPath = "cloudflared"
	}

	return m
}

// Status returns the current tunnel status.
func (m *Manager) Status() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// Start launches the cloudflared subprocess. No-op if already running or disabled.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancel != nil {
		return nil // already running
	}

	if m.cfg.Token == "" {
		m.setState(Status{State: StateDisabled})
		return nil
	}

	// Check cloudflared is available
	if _, err := exec.LookPath(m.cfg.CloudflaredPath); err != nil {
		m.setState(Status{State: StateError, Error: "cloudflared not found in PATH"})
		return fmt.Errorf("cloudflared not found: %w", err)
	}

	m.failureCount = 0
	childCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.done = make(chan struct{})

	m.setState(Status{State: StateConnecting, StartedAt: time.Now()})

	go m.run(childCtx)
	return nil
}

// Stop gracefully stops the cloudflared subprocess.
func (m *Manager) Stop() {
	m.mu.Lock()
	cancel := m.cancel
	done := m.done
	m.mu.Unlock()

	if cancel == nil {
		return
	}

	cancel()

	if done != nil {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			log.Println("tunnel: stop timed out after 10s")
		}
	}
}

// UpdateConfig updates configuration and restarts if needed.
func (m *Manager) UpdateConfig(cfg Config) {
	m.mu.Lock()
	old := m.cfg
	m.cfg = cfg
	if m.cfg.CloudflaredPath == "" {
		m.cfg.CloudflaredPath = "cloudflared"
	}
	m.mu.Unlock()

	if !cfg.Enabled || cfg.Token == "" {
		m.Stop()
		m.mu.Lock()
		if cfg.Token == "" {
			m.setState(Status{State: StateDisabled})
		} else {
			m.setState(Status{State: StateStopped})
		}
		m.cancel = nil
		m.done = nil
		m.mu.Unlock()
		return
	}

	// If token changed or was previously stopped, restart
	if cfg.Token != old.Token || old.Token == "" || !old.Enabled {
		m.Stop()
		m.mu.Lock()
		m.cancel = nil
		m.done = nil
		m.mu.Unlock()
		m.Start(context.Background())
	}
}

func (m *Manager) setState(s Status) {
	m.status = s
	if m.callback != nil {
		m.callback(s)
	}
}

func (m *Manager) run(ctx context.Context) {
	defer func() {
		m.mu.Lock()
		close(m.done)
		m.cancel = nil
		m.done = nil
		m.mu.Unlock()
	}()

	backoff := time.Second
	const maxBackoff = 60 * time.Second
	const maxFailures = 10

	for {
		select {
		case <-ctx.Done():
			m.mu.Lock()
			m.setState(Status{State: StateStopped})
			m.mu.Unlock()
			return
		default:
		}

		err := m.runOnce(ctx)

		select {
		case <-ctx.Done():
			m.mu.Lock()
			m.setState(Status{State: StateStopped})
			m.mu.Unlock()
			return
		default:
		}

		m.mu.Lock()
		m.failureCount++
		if m.failureCount >= maxFailures {
			errMsg := "too many consecutive failures"
			if err != nil {
				errMsg = err.Error()
			}
			m.setState(Status{State: StateError, Error: errMsg})
			m.mu.Unlock()
			return
		}
		m.setState(Status{State: StateReconnecting})
		m.mu.Unlock()

		log.Printf("tunnel: cloudflared exited (%v), retrying in %v (attempt %d/%d)", err, backoff, m.failureCount, maxFailures)

		select {
		case <-ctx.Done():
			m.mu.Lock()
			m.setState(Status{State: StateStopped})
			m.mu.Unlock()
			return
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func (m *Manager) runOnce(ctx context.Context) error {
	m.mu.RLock()
	token := m.cfg.Token
	path := m.cfg.CloudflaredPath
	m.mu.RUnlock()

	cmd := exec.CommandContext(ctx, path, "tunnel", "run", "--token", token)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start cloudflared: %w", err)
	}

	events := make(chan parseEvent, 16)
	go scanLogs(stderr, events)

	// Process events until the process exits
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	for {
		select {
		case ev := <-events:
			m.mu.Lock()
			switch ev.state {
			case StateConnected:
				s := Status{State: StateConnected, StartedAt: m.status.StartedAt}
				if ev.hostname != "" {
					s.Subdomain = ev.hostname
				} else if m.status.Subdomain != "" {
					s.Subdomain = m.status.Subdomain
				}
				m.failureCount = 0 // reset on successful connection
				m.setState(s)
			case StateReconnecting:
				s := Status{State: StateReconnecting, StartedAt: m.status.StartedAt, Subdomain: m.status.Subdomain}
				m.setState(s)
			}
			m.mu.Unlock()

		case err := <-done:
			return err
		}
	}
}

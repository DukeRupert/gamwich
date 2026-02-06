package license

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Config holds license validation configuration.
type Config struct {
	Key           string
	ValidationURL string
	CheckInterval time.Duration
	GracePeriod   time.Duration
}

// Status represents the current license status.
type Status struct {
	Valid       bool      `json:"valid"`
	Plan       string    `json:"plan"`
	Features   []string  `json:"features"`
	ExpiresAt  string    `json:"expires_at"`
	Warning    string    `json:"warning"`
	LastChecked time.Time `json:"last_checked"`
	Offline    bool      `json:"offline"`
}

type validateRequest struct {
	Key string `json:"key"`
}

type validateResponse struct {
	Valid     bool     `json:"valid"`
	Plan     string   `json:"plan,omitempty"`
	Features []string `json:"features,omitempty"`
	ExpiresAt *string  `json:"expires_at,omitempty"`
	Reason   string   `json:"reason,omitempty"`
}

// Client validates a license key against the billing service.
type Client struct {
	mu         sync.RWMutex
	cfg        Config
	status     Status
	httpClient *http.Client
	stopCh     chan struct{}
	stopped    chan struct{}
}

// NewClient creates a new license client. If key is empty, free-tier mode.
func NewClient(cfg Config) *Client {
	if cfg.CheckInterval == 0 {
		cfg.CheckInterval = 24 * time.Hour
	}
	if cfg.GracePeriod == 0 {
		cfg.GracePeriod = 7 * 24 * time.Hour
	}
	if cfg.ValidationURL == "" {
		cfg.ValidationURL = "https://gamwich.app/api/license/validate"
	}

	c := &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		stopCh:  make(chan struct{}),
		stopped: make(chan struct{}),
	}

	// Free-tier mode: no HTTP calls needed
	if cfg.Key == "" {
		c.status = Status{Valid: false, Plan: "free"}
	}

	return c
}

// Validate performs an immediate license validation against the billing service.
func (c *Client) Validate(ctx context.Context) error {
	c.mu.RLock()
	key := c.cfg.Key
	url := c.cfg.ValidationURL
	c.mu.RUnlock()

	if key == "" {
		c.mu.Lock()
		c.status = Status{Valid: false, Plan: "free", LastChecked: time.Now()}
		c.mu.Unlock()
		return nil
	}

	body, err := json.Marshal(validateRequest{Key: key})
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Network error — enter offline mode, keep existing status
		c.mu.Lock()
		c.status.Offline = true
		c.status.Warning = "Unable to reach license server"
		c.mu.Unlock()
		return fmt.Errorf("validate request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.mu.Lock()
		c.status.Offline = true
		c.status.Warning = fmt.Sprintf("License server returned %d", resp.StatusCode)
		c.mu.Unlock()
		return fmt.Errorf("validate: status %d", resp.StatusCode)
	}

	var vr validateResponse
	if err := json.NewDecoder(resp.Body).Decode(&vr); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	c.mu.Lock()
	c.status = Status{
		Valid:       vr.Valid,
		Plan:        vr.Plan,
		Features:    vr.Features,
		LastChecked: time.Now(),
		Offline:     false,
	}
	if vr.ExpiresAt != nil {
		c.status.ExpiresAt = *vr.ExpiresAt
	}
	if !vr.Valid && vr.Reason != "" {
		c.status.Warning = "License " + vr.Reason
	}
	c.mu.Unlock()

	return nil
}

// HasFeature checks if a specific feature is available under the current license.
func (c *Client) HasFeature(feature string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.status.Valid {
		// If offline and within grace period, maintain access
		if c.status.Offline && !c.status.LastChecked.IsZero() &&
			time.Since(c.status.LastChecked) < c.cfg.GracePeriod {
			return c.hasFeatureInList(feature)
		}
		return false
	}

	// Check grace period for expired checks
	if !c.status.LastChecked.IsZero() &&
		time.Since(c.status.LastChecked) > c.cfg.GracePeriod {
		return false
	}

	return c.hasFeatureInList(feature)
}

func (c *Client) hasFeatureInList(feature string) bool {
	for _, f := range c.status.Features {
		if f == feature {
			return true
		}
	}
	return false
}

// Status returns the current cached license status.
func (c *Client) Status() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

// IsFreeTier returns true if no license key is configured.
func (c *Client) IsFreeTier() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cfg.Key == ""
}

// SetKey updates the license key and triggers immediate validation.
func (c *Client) SetKey(key string) {
	c.mu.Lock()
	c.cfg.Key = key
	if key == "" {
		c.status = Status{Valid: false, Plan: "free", LastChecked: time.Now()}
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()

	c.Validate(context.Background())
}

// Start begins the background validation goroutine.
func (c *Client) Start(ctx context.Context) {
	// Initial validation
	c.Validate(ctx)

	go func() {
		defer close(c.stopped)
		ticker := time.NewTicker(c.cfg.CheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				c.Validate(ctx)
			case <-c.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Stop halts the background validation goroutine.
func (c *Client) Stop() {
	close(c.stopCh)
	<-c.stopped
}

package license

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFreeModeNoFeatures(t *testing.T) {
	c := NewClient(Config{Key: ""})

	if !c.IsFreeTier() {
		t.Error("expected free tier with empty key")
	}
	if c.HasFeature("tunnel") {
		t.Error("expected tunnel feature disabled in free mode")
	}
	status := c.Status()
	if status.Plan != "free" {
		t.Errorf("plan = %q, want %q", status.Plan, "free")
	}
}

func TestValidKeyFeaturesEnabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req validateRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Key != "GW-TEST-1234-5678-ABCD" {
			t.Errorf("unexpected key: %q", req.Key)
		}
		json.NewEncoder(w).Encode(validateResponse{
			Valid:    true,
			Plan:     "cloud",
			Features: []string{"tunnel", "backup", "push"},
		})
	}))
	defer server.Close()

	c := NewClient(Config{
		Key:           "GW-TEST-1234-5678-ABCD",
		ValidationURL: server.URL,
	})

	if err := c.Validate(context.Background()); err != nil {
		t.Fatalf("validate: %v", err)
	}

	if c.IsFreeTier() {
		t.Error("expected paid tier")
	}
	if !c.HasFeature("tunnel") {
		t.Error("expected tunnel feature enabled")
	}
	if !c.HasFeature("backup") {
		t.Error("expected backup feature enabled")
	}
	if c.HasFeature("nonexistent") {
		t.Error("expected nonexistent feature disabled")
	}
}

func TestExpiredKeyFeaturesDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(validateResponse{
			Valid:  false,
			Reason: "expired",
		})
	}))
	defer server.Close()

	c := NewClient(Config{
		Key:           "GW-EXPIRED-KEY-0000",
		ValidationURL: server.URL,
	})

	c.Validate(context.Background())

	if c.HasFeature("tunnel") {
		t.Error("expected tunnel feature disabled for expired key")
	}
	status := c.Status()
	if status.Valid {
		t.Error("expected invalid status")
	}
}

func TestOfflineGracePeriod(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// First call succeeds
			json.NewEncoder(w).Encode(validateResponse{
				Valid:    true,
				Plan:     "cloud",
				Features: []string{"tunnel", "backup"},
			})
		} else {
			// Subsequent calls fail (server unreachable)
			http.Error(w, "server error", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	c := NewClient(Config{
		Key:           "GW-TEST-1234-5678-ABCD",
		ValidationURL: server.URL,
		GracePeriod:   1 * time.Hour,
	})

	// First validation succeeds
	if err := c.Validate(context.Background()); err != nil {
		t.Fatalf("first validate: %v", err)
	}
	if !c.HasFeature("tunnel") {
		t.Error("expected tunnel enabled after successful validation")
	}

	// Second validation fails (server error) but features should still work within grace period
	c.Validate(context.Background()) // will set offline=true
	status := c.Status()
	if !status.Offline {
		t.Error("expected offline status after failed validation")
	}
	// Features still work because within grace period
	if !c.HasFeature("tunnel") {
		t.Error("expected tunnel still enabled within grace period")
	}
}

func TestGracePeriodExpired(t *testing.T) {
	c := NewClient(Config{
		Key:         "GW-TEST-1234-5678-ABCD",
		GracePeriod: 1 * time.Millisecond,
	})

	// Simulate a cached status that was valid but checked long ago
	c.mu.Lock()
	c.status = Status{
		Valid:       true,
		Plan:        "cloud",
		Features:    []string{"tunnel"},
		LastChecked: time.Now().Add(-1 * time.Hour), // checked 1 hour ago
	}
	c.mu.Unlock()

	// Grace period of 1ms is well expired, so features should be disabled
	if c.HasFeature("tunnel") {
		t.Error("expected tunnel disabled after grace period expired")
	}
}

func TestSetKeyTriggersValidation(t *testing.T) {
	validated := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		validated = true
		var req validateRequest
		json.NewDecoder(r.Body).Decode(&req)
		json.NewEncoder(w).Encode(validateResponse{
			Valid:    true,
			Plan:     "cloud",
			Features: []string{"tunnel"},
		})
	}))
	defer server.Close()

	c := NewClient(Config{
		Key:           "",
		ValidationURL: server.URL,
	})

	if !c.IsFreeTier() {
		t.Error("expected free tier initially")
	}

	c.SetKey("GW-NEW-KEY-1234-5678")

	if !validated {
		t.Error("expected validation to be triggered by SetKey")
	}
	if c.IsFreeTier() {
		t.Error("expected paid tier after SetKey")
	}
	if !c.HasFeature("tunnel") {
		t.Error("expected tunnel enabled after SetKey")
	}
}

func TestSetKeyToEmpty(t *testing.T) {
	c := NewClient(Config{Key: "GW-TEST-1234-5678-ABCD"})

	c.SetKey("")

	if !c.IsFreeTier() {
		t.Error("expected free tier after clearing key")
	}
	if c.HasFeature("tunnel") {
		t.Error("expected tunnel disabled after clearing key")
	}
}

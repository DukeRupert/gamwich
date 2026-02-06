package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/dukerupert/gamwich/internal/billing/store"
)

type LicenseHandler struct {
	licenseKeyStore *store.LicenseKeyStore
}

func NewLicenseHandler(lks *store.LicenseKeyStore) *LicenseHandler {
	return &LicenseHandler{licenseKeyStore: lks}
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

// Validate checks a license key and returns its validity and associated features.
func (h *LicenseHandler) Validate(w http.ResponseWriter, r *http.Request) {
	var req validateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.Key == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(validateResponse{Valid: false, Reason: "not_found"})
		return
	}

	lk, err := h.licenseKeyStore.GetByKey(req.Key)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if lk == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(validateResponse{Valid: false, Reason: "not_found"})
		return
	}

	// Check expiry
	if lk.ExpiresAt != nil && lk.ExpiresAt.Before(time.Now().UTC()) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(validateResponse{Valid: false, Reason: "expired"})
		return
	}

	// Parse features
	var features []string
	if lk.Features != "" {
		for _, f := range splitFeatures(lk.Features) {
			features = append(features, f)
		}
	}

	resp := validateResponse{
		Valid:    true,
		Plan:     lk.Plan,
		Features: features,
	}
	if lk.ExpiresAt != nil {
		s := lk.ExpiresAt.Format(time.RFC3339)
		resp.ExpiresAt = &s
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func splitFeatures(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			if i > start {
				result = append(result, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}

package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/dukerupert/gamwich/internal/auth"
	"github.com/dukerupert/gamwich/internal/push"
	"github.com/dukerupert/gamwich/internal/store"
)

type PushHandler struct {
	pushStore *store.PushStore
	service   *push.Service
	logger    *slog.Logger
}

func NewPushHandler(ps *store.PushStore, svc *push.Service, logger *slog.Logger) *PushHandler {
	return &PushHandler{pushStore: ps, service: svc, logger: logger}
}

type subscribeRequest struct {
	Endpoint   string `json:"endpoint"`
	P256dh     string `json:"p256dh"`
	Auth       string `json:"auth"`
	DeviceName string `json:"device_name"`
}

// Subscribe handles POST /api/push/subscribe
func (h *PushHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	householdID := auth.HouseholdID(r.Context())

	var req subscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.Endpoint == "" || req.P256dh == "" || req.Auth == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "endpoint, p256dh, and auth are required"})
		return
	}

	sub, err := h.pushStore.CreateSubscription(userID, householdID, req.Endpoint, req.P256dh, req.Auth, req.DeviceName)
	if err != nil {
		h.logger.Error("create push subscription", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save subscription"})
		return
	}

	writeJSON(w, http.StatusCreated, sub)
}

// Unsubscribe handles DELETE /api/push/subscriptions/{id}
func (h *PushHandler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	householdID := auth.HouseholdID(r.Context())

	id, err := parseIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	if err := h.pushStore.DeleteSubscription(id, householdID); err != nil {
		h.logger.Error("delete push subscription", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete subscription"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListSubscriptions handles GET /api/push/subscriptions
func (h *PushHandler) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	householdID := auth.HouseholdID(r.Context())

	subs, err := h.pushStore.ListByUser(userID, householdID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list subscriptions"})
		return
	}
	if subs == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	writeJSON(w, http.StatusOK, subs)
}

// GetVAPIDKey handles GET /api/push/vapid-key
func (h *PushHandler) GetVAPIDKey(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"public_key": h.service.VAPIDPublicKey()})
}

// GetPreferences handles GET /api/push/preferences
func (h *PushHandler) GetPreferences(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	householdID := auth.HouseholdID(r.Context())

	prefs, err := h.pushStore.GetPreferences(userID, householdID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get preferences"})
		return
	}
	if prefs == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	writeJSON(w, http.StatusOK, prefs)
}

type updatePreferencesRequest struct {
	Preferences []prefItem `json:"preferences"`
}

type prefItem struct {
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
}

// UpdatePreferences handles PUT /api/push/preferences
func (h *PushHandler) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	householdID := auth.HouseholdID(r.Context())

	var req updatePreferencesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	for _, p := range req.Preferences {
		if err := h.pushStore.SetPreference(userID, householdID, p.Type, p.Enabled); err != nil {
			h.logger.Error("set push preference", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update preferences"})
			return
		}
	}

	prefs, _ := h.pushStore.GetPreferences(userID, householdID)
	writeJSON(w, http.StatusOK, prefs)
}

// TestNotification handles POST /api/push/test
func (h *PushHandler) TestNotification(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	householdID := auth.HouseholdID(r.Context())

	subs, err := h.pushStore.ListByUser(userID, householdID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list subscriptions"})
		return
	}

	payload := push.Payload{
		Title: "Test Notification",
		Body:  "Push notifications are working!",
		URL:   "/settings",
		Tag:   "test",
	}

	sent := 0
	for _, sub := range subs {
		if err := h.service.Send(&sub, payload); err != nil {
			h.logger.Error("test push send", "error", err)
			continue
		}
		sent++
	}

	writeJSON(w, http.StatusOK, map[string]int{"sent": sent})
}

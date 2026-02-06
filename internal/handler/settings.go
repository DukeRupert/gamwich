package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"

	"github.com/dukerupert/gamwich/internal/store"
	"github.com/dukerupert/gamwich/internal/websocket"
)

var timeFormatRegexp = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

type SettingsHandler struct {
	settingsStore *store.SettingsStore
	hub           *websocket.Hub
}

func NewSettingsHandler(ss *store.SettingsStore, hub *websocket.Hub) *SettingsHandler {
	return &SettingsHandler{settingsStore: ss, hub: hub}
}

func (h *SettingsHandler) broadcast(msg websocket.Message) {
	if h.hub != nil {
		h.hub.Broadcast(msg)
	}
}

func (h *SettingsHandler) GetKiosk(w http.ResponseWriter, r *http.Request) {
	settings, err := h.settingsStore.GetKioskSettings()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get settings"})
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (h *SettingsHandler) UpdateKiosk(w http.ResponseWriter, r *http.Request) {
	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if err := validateKioskSettings(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	for key, value := range req {
		if err := h.settingsStore.Set(key, value); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save settings"})
			return
		}
	}

	h.broadcast(websocket.NewMessage("settings", "updated", 0, nil))

	settings, err := h.settingsStore.GetKioskSettings()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get settings"})
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func validateKioskSettings(settings map[string]string) error {
	allowedKeys := map[string]bool{
		"idle_timeout_minutes": true,
		"quiet_hours_enabled":  true,
		"quiet_hours_start":    true,
		"quiet_hours_end":      true,
		"burn_in_prevention":   true,
	}

	for key, value := range settings {
		if !allowedKeys[key] {
			return fmt.Errorf("unknown setting: %s", key)
		}

		switch key {
		case "idle_timeout_minutes":
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 || n > 60 {
				return fmt.Errorf("idle_timeout_minutes must be 1-60")
			}
		case "quiet_hours_enabled", "burn_in_prevention":
			if value != "true" && value != "false" {
				return fmt.Errorf("%s must be \"true\" or \"false\"", key)
			}
		case "quiet_hours_start", "quiet_hours_end":
			if !timeFormatRegexp.MatchString(value) {
				return fmt.Errorf("%s must be HH:MM format", key)
			}
		}
	}
	return nil
}

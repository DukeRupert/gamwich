package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dukerupert/gamwich/internal/model"
	"github.com/dukerupert/gamwich/internal/store"
)

type CalendarEventHandler struct {
	eventStore  *store.EventStore
	memberStore *store.FamilyMemberStore
}

func NewCalendarEventHandler(es *store.EventStore, ms *store.FamilyMemberStore) *CalendarEventHandler {
	return &CalendarEventHandler{eventStore: es, memberStore: ms}
}

type eventRequest struct {
	Title          string `json:"title"`
	Description    string `json:"description"`
	StartTime      string `json:"start_time"`
	EndTime        string `json:"end_time"`
	AllDay         bool   `json:"all_day"`
	FamilyMemberID *int64 `json:"family_member_id"`
	Location       string `json:"location"`
}

func (h *CalendarEventHandler) parseAndValidate(r *http.Request, w http.ResponseWriter) (*eventRequest, time.Time, time.Time, bool) {
	var req eventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return nil, time.Time{}, time.Time{}, false
	}

	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
		return nil, time.Time{}, time.Time{}, false
	}

	startTime, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "start_time must be RFC3339 format"})
		return nil, time.Time{}, time.Time{}, false
	}

	endTime, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "end_time must be RFC3339 format"})
		return nil, time.Time{}, time.Time{}, false
	}

	if !startTime.Before(endTime) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "start_time must be before end_time"})
		return nil, time.Time{}, time.Time{}, false
	}

	if req.FamilyMemberID != nil {
		member, err := h.memberStore.GetByID(*req.FamilyMemberID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to check family member"})
			return nil, time.Time{}, time.Time{}, false
		}
		if member == nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "family member not found"})
			return nil, time.Time{}, time.Time{}, false
		}
	}

	return &req, startTime, endTime, true
}

func (h *CalendarEventHandler) Create(w http.ResponseWriter, r *http.Request) {
	req, startTime, endTime, ok := h.parseAndValidate(r, w)
	if !ok {
		return
	}

	event, err := h.eventStore.Create(req.Title, req.Description, startTime, endTime, req.AllDay, req.FamilyMemberID, req.Location)
	if err != nil {
		log.Printf("failed to create calendar event: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create event"})
		return
	}

	writeJSON(w, http.StatusCreated, event)
}

func (h *CalendarEventHandler) List(w http.ResponseWriter, r *http.Request) {
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	if startStr == "" || endStr == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "start and end query parameters are required"})
		return
	}

	start, err := parseFlexibleTime(startStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "start must be RFC3339 or YYYY-MM-DD format"})
		return
	}

	end, err := parseFlexibleTime(endStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "end must be RFC3339 or YYYY-MM-DD format"})
		return
	}

	events, err := h.eventStore.ListByDateRange(start, end)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list events"})
		return
	}
	if events == nil {
		events = []model.CalendarEvent{}
	}

	writeJSON(w, http.StatusOK, events)
}

func (h *CalendarEventHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	event, err := h.eventStore.GetByID(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get event"})
		return
	}
	if event == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "event not found"})
		return
	}

	writeJSON(w, http.StatusOK, event)
}

func (h *CalendarEventHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	existing, err := h.eventStore.GetByID(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get event"})
		return
	}
	if existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "event not found"})
		return
	}

	req, startTime, endTime, ok := h.parseAndValidate(r, w)
	if !ok {
		return
	}

	event, err := h.eventStore.Update(id, req.Title, req.Description, startTime, endTime, req.AllDay, req.FamilyMemberID, req.Location)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update event"})
		return
	}

	writeJSON(w, http.StatusOK, event)
}

func (h *CalendarEventHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	existing, err := h.eventStore.GetByID(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get event"})
		return
	}
	if existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "event not found"})
		return
	}

	if err := h.eventStore.Delete(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete event"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseFlexibleTime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02", s)
}

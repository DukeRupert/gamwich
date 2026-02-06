package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/dukerupert/gamwich/internal/model"
	"github.com/dukerupert/gamwich/internal/recurrence"
	"github.com/dukerupert/gamwich/internal/store"
	"github.com/dukerupert/gamwich/internal/websocket"
)

type CalendarEventHandler struct {
	eventStore  *store.EventStore
	memberStore *store.FamilyMemberStore
	hub         *websocket.Hub
}

func NewCalendarEventHandler(es *store.EventStore, ms *store.FamilyMemberStore, hub *websocket.Hub) *CalendarEventHandler {
	return &CalendarEventHandler{eventStore: es, memberStore: ms, hub: hub}
}

func (h *CalendarEventHandler) broadcast(msg websocket.Message) {
	if h.hub != nil {
		h.hub.Broadcast(msg)
	}
}

type eventRequest struct {
	Title          string `json:"title"`
	Description    string `json:"description"`
	StartTime      string `json:"start_time"`
	EndTime        string `json:"end_time"`
	AllDay         bool   `json:"all_day"`
	FamilyMemberID *int64 `json:"family_member_id"`
	Location       string `json:"location"`
	RecurrenceRule string `json:"recurrence_rule"`
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

	event, err := h.eventStore.CreateWithRecurrence(req.Title, req.Description, startTime, endTime, req.AllDay, req.FamilyMemberID, req.Location, req.RecurrenceRule)
	if err != nil {
		log.Printf("failed to create calendar event: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create event"})
		return
	}

	h.broadcast(websocket.NewMessage("calendar_event", "created", event.ID, nil))

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

	// Get non-recurring events
	events, err := h.eventStore.ListByDateRange(start, end)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list events"})
		return
	}
	if events == nil {
		events = []model.CalendarEvent{}
	}

	// Expand recurring events
	recurring, err := h.eventStore.ListRecurring(end)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list recurring events"})
		return
	}

	for _, parent := range recurring {
		rule, err := recurrence.Parse(parent.RecurrenceRule)
		if err != nil {
			log.Printf("skip recurring event %d: invalid rule: %v", parent.ID, err)
			continue
		}

		occurrences := recurrence.Expand(rule, parent.StartTime, parent.EndTime, start, end)

		exceptions, err := h.eventStore.ListExceptions(parent.ID)
		if err != nil {
			continue
		}

		excMap := make(map[string]model.CalendarEvent)
		for _, exc := range exceptions {
			if exc.OriginalStartTime != nil {
				excMap[exc.OriginalStartTime.Format("2006-01-02T15:04:05Z")] = exc
			}
		}

		for _, occ := range occurrences {
			key := occ.Start.Format("2006-01-02T15:04:05Z")
			if exc, found := excMap[key]; found {
				if exc.Cancelled {
					continue
				}
				events = append(events, exc)
			} else {
				virtual := parent
				virtual.StartTime = occ.Start
				virtual.EndTime = occ.End
				events = append(events, virtual)
			}
		}
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].StartTime.Before(events[j].StartTime)
	})

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

	event, err := h.eventStore.UpdateWithRecurrence(id, req.Title, req.Description, startTime, endTime, req.AllDay, req.FamilyMemberID, req.Location, req.RecurrenceRule)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update event"})
		return
	}

	h.broadcast(websocket.NewMessage("calendar_event", "updated", id, nil))

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

	h.broadcast(websocket.NewMessage("calendar_event", "deleted", id, nil))

	w.WriteHeader(http.StatusNoContent)
}

func parseFlexibleTime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02", s)
}

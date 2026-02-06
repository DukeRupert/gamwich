package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/dukerupert/gamwich/internal/model"
	"github.com/dukerupert/gamwich/internal/store"
	"github.com/dukerupert/gamwich/internal/websocket"
)

type ChoreHandler struct {
	choreStore  *store.ChoreStore
	memberStore *store.FamilyMemberStore
	hub         *websocket.Hub
}

func NewChoreHandler(cs *store.ChoreStore, ms *store.FamilyMemberStore, hub *websocket.Hub) *ChoreHandler {
	return &ChoreHandler{choreStore: cs, memberStore: ms, hub: hub}
}

func (h *ChoreHandler) broadcast(msg websocket.Message) {
	if h.hub != nil {
		h.hub.Broadcast(msg)
	}
}

type choreRequest struct {
	Title          string `json:"title"`
	Description    string `json:"description"`
	AreaID         *int64 `json:"area_id"`
	Points         int    `json:"points"`
	RecurrenceRule string `json:"recurrence_rule"`
	AssignedTo     *int64 `json:"assigned_to"`
}

func (h *ChoreHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req choreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
		return
	}

	if req.AssignedTo != nil {
		member, err := h.memberStore.GetByID(*req.AssignedTo)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to check family member"})
			return
		}
		if member == nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "family member not found"})
			return
		}
	}

	chore, err := h.choreStore.Create(req.Title, req.Description, req.AreaID, req.Points, req.RecurrenceRule, req.AssignedTo)
	if err != nil {
		log.Printf("failed to create chore: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create chore"})
		return
	}

	h.broadcast(websocket.NewMessage("chore", "created", chore.ID, nil))

	writeJSON(w, http.StatusCreated, chore)
}

func (h *ChoreHandler) List(w http.ResponseWriter, r *http.Request) {
	chores, err := h.choreStore.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list chores"})
		return
	}
	if chores == nil {
		chores = []model.Chore{}
	}
	writeJSON(w, http.StatusOK, chores)
}

func (h *ChoreHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	existing, err := h.choreStore.GetByID(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get chore"})
		return
	}
	if existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "chore not found"})
		return
	}

	var req choreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
		return
	}

	chore, err := h.choreStore.Update(id, req.Title, req.Description, req.AreaID, req.Points, req.RecurrenceRule, req.AssignedTo)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update chore"})
		return
	}

	h.broadcast(websocket.NewMessage("chore", "updated", id, nil))

	writeJSON(w, http.StatusOK, chore)
}

func (h *ChoreHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	existing, err := h.choreStore.GetByID(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get chore"})
		return
	}
	if existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "chore not found"})
		return
	}

	if err := h.choreStore.Delete(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete chore"})
		return
	}

	h.broadcast(websocket.NewMessage("chore", "deleted", id, nil))

	w.WriteHeader(http.StatusNoContent)
}

func (h *ChoreHandler) Complete(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	existing, err := h.choreStore.GetByID(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get chore"})
		return
	}
	if existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "chore not found"})
		return
	}

	var req struct {
		CompletedBy *int64 `json:"completed_by"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	completion, err := h.choreStore.CreateCompletion(id, req.CompletedBy)
	if err != nil {
		log.Printf("failed to complete chore: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to complete chore"})
		return
	}

	h.broadcast(websocket.NewMessage("chore", "completed", id, nil))

	writeJSON(w, http.StatusCreated, completion)
}

func (h *ChoreHandler) UndoComplete(w http.ResponseWriter, r *http.Request) {
	completionIDStr := r.PathValue("completion_id")
	completionID, err := strconv.ParseInt(completionIDStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid completion_id"})
		return
	}

	if err := h.choreStore.DeleteCompletion(completionID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to undo completion"})
		return
	}

	choreID, _ := parseIDParam(r)
	h.broadcast(websocket.NewMessage("chore", "completion_undone", choreID, nil))

	w.WriteHeader(http.StatusNoContent)
}

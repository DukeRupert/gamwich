package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dukerupert/gamwich/internal/model"
	"github.com/dukerupert/gamwich/internal/store"
	"github.com/dukerupert/gamwich/internal/websocket"
)

type NoteHandler struct {
	noteStore   *store.NoteStore
	memberStore *store.FamilyMemberStore
	hub         *websocket.Hub
}

func NewNoteHandler(ns *store.NoteStore, ms *store.FamilyMemberStore, hub *websocket.Hub) *NoteHandler {
	return &NoteHandler{noteStore: ns, memberStore: ms, hub: hub}
}

func (h *NoteHandler) broadcast(msg websocket.Message) {
	if h.hub != nil {
		h.hub.Broadcast(msg)
	}
}

var validPriorities = map[string]bool{
	"urgent": true,
	"normal": true,
	"fun":    true,
}

type noteRequest struct {
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	AuthorID  *int64     `json:"author_id"`
	Pinned    bool       `json:"pinned"`
	Priority  string     `json:"priority"`
	ExpiresAt *time.Time `json:"expires_at"`
}

func (h *NoteHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req noteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
		return
	}

	if req.Priority == "" {
		req.Priority = "normal"
	}
	if !validPriorities[req.Priority] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "priority must be urgent, normal, or fun"})
		return
	}

	note, err := h.noteStore.Create(req.Title, req.Body, req.AuthorID, req.Pinned, req.Priority, req.ExpiresAt)
	if err != nil {
		log.Printf("failed to create note: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create note"})
		return
	}

	h.broadcast(websocket.NewMessage("note", "created", note.ID, nil))

	writeJSON(w, http.StatusCreated, note)
}

func (h *NoteHandler) List(w http.ResponseWriter, r *http.Request) {
	notes, err := h.noteStore.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list notes"})
		return
	}
	if notes == nil {
		notes = []model.Note{}
	}
	writeJSON(w, http.StatusOK, notes)
}

func (h *NoteHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	existing, err := h.noteStore.GetByID(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get note"})
		return
	}
	if existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "note not found"})
		return
	}

	var req noteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
		return
	}

	if req.Priority == "" {
		req.Priority = "normal"
	}
	if !validPriorities[req.Priority] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "priority must be urgent, normal, or fun"})
		return
	}

	note, err := h.noteStore.Update(id, req.Title, req.Body, req.AuthorID, req.Pinned, req.Priority, req.ExpiresAt)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update note"})
		return
	}

	h.broadcast(websocket.NewMessage("note", "updated", id, nil))

	writeJSON(w, http.StatusOK, note)
}

func (h *NoteHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	existing, err := h.noteStore.GetByID(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get note"})
		return
	}
	if existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "note not found"})
		return
	}

	if err := h.noteStore.Delete(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete note"})
		return
	}

	h.broadcast(websocket.NewMessage("note", "deleted", id, nil))

	w.WriteHeader(http.StatusNoContent)
}

func (h *NoteHandler) TogglePinned(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	note, err := h.noteStore.TogglePinned(id)
	if err != nil {
		log.Printf("failed to toggle note pin: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to toggle pin"})
		return
	}
	if note == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "note not found"})
		return
	}

	h.broadcast(websocket.NewMessage("note", "pinned", id, nil))

	writeJSON(w, http.StatusOK, note)
}

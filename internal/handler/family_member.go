package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/dukerupert/gamwich/internal/model"
	"github.com/dukerupert/gamwich/internal/store"
	"golang.org/x/crypto/bcrypt"
)

var hexColorRegexp = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type FamilyMemberHandler struct {
	store *store.FamilyMemberStore
}

func NewFamilyMemberHandler(s *store.FamilyMemberStore) *FamilyMemberHandler {
	return &FamilyMemberHandler{store: s}
}

func (h *FamilyMemberHandler) List(w http.ResponseWriter, r *http.Request) {
	members, err := h.store.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list family members"})
		return
	}
	if members == nil {
		members = []model.FamilyMember{}
	}
	writeJSON(w, http.StatusOK, members)
}

func (h *FamilyMemberHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Color       string `json:"color"`
		AvatarEmoji string `json:"avatar_emoji"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	if req.Color == "" {
		req.Color = "#3B82F6"
	}
	if !hexColorRegexp.MatchString(req.Color) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "color must be a hex color (e.g. #FF0000)"})
		return
	}

	if req.AvatarEmoji == "" {
		req.AvatarEmoji = "😀"
	}

	exists, err := h.store.NameExists(req.Name, 0)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to check name"})
		return
	}
	if exists {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "a family member with that name already exists"})
		return
	}

	member, err := h.store.Create(req.Name, req.Color, req.AvatarEmoji)
	if err != nil {
		log.Printf("failed to create family member: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create family member"})
		return
	}

	writeJSON(w, http.StatusCreated, member)
}

func (h *FamilyMemberHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	existing, err := h.store.GetByID(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get family member"})
		return
	}
	if existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "family member not found"})
		return
	}

	var req struct {
		Name        string `json:"name"`
		Color       string `json:"color"`
		AvatarEmoji string `json:"avatar_emoji"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	if req.Color == "" {
		req.Color = existing.Color
	}
	if !hexColorRegexp.MatchString(req.Color) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "color must be a hex color (e.g. #FF0000)"})
		return
	}

	if req.AvatarEmoji == "" {
		req.AvatarEmoji = existing.AvatarEmoji
	}

	exists, err := h.store.NameExists(req.Name, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to check name"})
		return
	}
	if exists {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "a family member with that name already exists"})
		return
	}

	member, err := h.store.Update(id, req.Name, req.Color, req.AvatarEmoji)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update family member"})
		return
	}

	writeJSON(w, http.StatusOK, member)
}

func (h *FamilyMemberHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	existing, err := h.store.GetByID(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get family member"})
		return
	}
	if existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "family member not found"})
		return
	}

	if err := h.store.Delete(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete family member"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *FamilyMemberHandler) UpdateSortOrder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if len(req.IDs) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ids are required"})
		return
	}

	if err := h.store.UpdateSortOrder(req.IDs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update sort order"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *FamilyMemberHandler) SetPIN(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	existing, err := h.store.GetByID(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get family member"})
		return
	}
	if existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "family member not found"})
		return
	}

	var req struct {
		PIN string `json:"pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if len(req.PIN) != 4 || !isDigits(req.PIN) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "PIN must be exactly 4 digits"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.PIN), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to hash PIN"})
		return
	}

	if err := h.store.SetPIN(id, string(hash)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to set PIN"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "pin set"})
}

func (h *FamilyMemberHandler) ClearPIN(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	if err := h.store.ClearPIN(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to clear PIN"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "pin cleared"})
}

func (h *FamilyMemberHandler) VerifyPIN(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	var req struct {
		PIN string `json:"pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	hash, err := h.store.GetPINHash(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get PIN"})
		return
	}
	if hash == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no PIN set for this member"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.PIN)); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "incorrect PIN"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "verified"})
}

func parseIDParam(r *http.Request) (int64, error) {
	idStr := r.PathValue("id")
	return strconv.ParseInt(idStr, 10, 64)
}

func isDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

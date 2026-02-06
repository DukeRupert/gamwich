package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/dukerupert/gamwich/internal/grocery"
	"github.com/dukerupert/gamwich/internal/model"
	"github.com/dukerupert/gamwich/internal/store"
)

type GroceryHandler struct {
	groceryStore *store.GroceryStore
	memberStore  *store.FamilyMemberStore
}

func NewGroceryHandler(gs *store.GroceryStore, ms *store.FamilyMemberStore) *GroceryHandler {
	return &GroceryHandler{groceryStore: gs, memberStore: ms}
}

type groceryItemRequest struct {
	Name     string `json:"name"`
	Quantity string `json:"quantity"`
	Unit     string `json:"unit"`
	Notes    string `json:"notes"`
	Category string `json:"category"`
	AddedBy  *int64 `json:"added_by"`
}

func (h *GroceryHandler) CreateItem(w http.ResponseWriter, r *http.Request) {
	listID, err := strconv.ParseInt(r.PathValue("list_id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid list_id"})
		return
	}

	var req groceryItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	// Auto-categorize if no category provided
	if req.Category == "" {
		req.Category = grocery.Categorize(req.Name)
	}

	item, err := h.groceryStore.CreateItem(listID, req.Name, req.Quantity, req.Unit, req.Notes, req.Category, req.AddedBy)
	if err != nil {
		log.Printf("failed to create grocery item: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create item"})
		return
	}

	writeJSON(w, http.StatusCreated, item)
}

func (h *GroceryHandler) ListItems(w http.ResponseWriter, r *http.Request) {
	listID, err := strconv.ParseInt(r.PathValue("list_id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid list_id"})
		return
	}

	items, err := h.groceryStore.ListItemsByList(listID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list items"})
		return
	}
	if items == nil {
		items = []model.GroceryItem{}
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *GroceryHandler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	existing, err := h.groceryStore.GetItemByID(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get item"})
		return
	}
	if existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "item not found"})
		return
	}

	var req groceryItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	if req.Category == "" {
		req.Category = existing.Category
	}

	item, err := h.groceryStore.UpdateItem(id, req.Name, req.Quantity, req.Unit, req.Notes, req.Category)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update item"})
		return
	}

	writeJSON(w, http.StatusOK, item)
}

func (h *GroceryHandler) DeleteItem(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	existing, err := h.groceryStore.GetItemByID(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get item"})
		return
	}
	if existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "item not found"})
		return
	}

	if err := h.groceryStore.DeleteItem(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete item"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *GroceryHandler) ToggleChecked(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	var req struct {
		CheckedBy *int64 `json:"checked_by"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	item, err := h.groceryStore.ToggleChecked(id, req.CheckedBy)
	if err != nil {
		log.Printf("failed to toggle checked: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to toggle checked"})
		return
	}
	if item == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "item not found"})
		return
	}

	writeJSON(w, http.StatusOK, item)
}

func (h *GroceryHandler) ClearChecked(w http.ResponseWriter, r *http.Request) {
	listID, err := strconv.ParseInt(r.PathValue("list_id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid list_id"})
		return
	}

	count, err := h.groceryStore.ClearChecked(listID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to clear checked"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]int64{"cleared": count})
}

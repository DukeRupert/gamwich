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

type RewardHandler struct {
	rewardStore *store.RewardStore
	memberStore *store.FamilyMemberStore
	hub         *websocket.Hub
}

func NewRewardHandler(rs *store.RewardStore, ms *store.FamilyMemberStore, hub *websocket.Hub) *RewardHandler {
	return &RewardHandler{rewardStore: rs, memberStore: ms, hub: hub}
}

func (h *RewardHandler) broadcast(msg websocket.Message) {
	if h.hub != nil {
		h.hub.Broadcast(msg)
	}
}

type rewardRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	PointCost   int    `json:"point_cost"`
	Active      bool   `json:"active"`
}

func (h *RewardHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req rewardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
		return
	}
	if req.PointCost < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "point_cost must be >= 0"})
		return
	}

	reward, err := h.rewardStore.Create(req.Title, req.Description, req.PointCost, req.Active)
	if err != nil {
		log.Printf("failed to create reward: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create reward"})
		return
	}

	h.broadcast(websocket.NewMessage("reward", "created", reward.ID, nil))

	writeJSON(w, http.StatusCreated, reward)
}

func (h *RewardHandler) List(w http.ResponseWriter, r *http.Request) {
	rewards, err := h.rewardStore.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list rewards"})
		return
	}
	if rewards == nil {
		rewards = []model.Reward{}
	}
	writeJSON(w, http.StatusOK, rewards)
}

func (h *RewardHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	existing, err := h.rewardStore.GetByID(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get reward"})
		return
	}
	if existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "reward not found"})
		return
	}

	var req rewardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
		return
	}
	if req.PointCost < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "point_cost must be >= 0"})
		return
	}

	reward, err := h.rewardStore.Update(id, req.Title, req.Description, req.PointCost, req.Active)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update reward"})
		return
	}

	h.broadcast(websocket.NewMessage("reward", "updated", id, nil))

	writeJSON(w, http.StatusOK, reward)
}

func (h *RewardHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	existing, err := h.rewardStore.GetByID(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get reward"})
		return
	}
	if existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "reward not found"})
		return
	}

	if err := h.rewardStore.Delete(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete reward"})
		return
	}

	h.broadcast(websocket.NewMessage("reward", "deleted", id, nil))

	w.WriteHeader(http.StatusNoContent)
}

func (h *RewardHandler) Redeem(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	reward, err := h.rewardStore.GetByID(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get reward"})
		return
	}
	if reward == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "reward not found"})
		return
	}
	if !reward.Active {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "reward is not active"})
		return
	}

	var req struct {
		RedeemedBy *int64 `json:"redeemed_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Check balance if member specified
	if req.RedeemedBy != nil {
		balance, err := h.rewardStore.GetPointBalance(*req.RedeemedBy)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to check balance"})
			return
		}
		if balance.Balance < reward.PointCost {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "insufficient points"})
			return
		}
	}

	redemption, err := h.rewardStore.Redeem(id, req.RedeemedBy, reward.PointCost)
	if err != nil {
		log.Printf("failed to redeem reward: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to redeem reward"})
		return
	}

	h.broadcast(websocket.NewMessage("reward", "redeemed", id, nil))

	writeJSON(w, http.StatusCreated, redemption)
}

func (h *RewardHandler) GetPointBalance(w http.ResponseWriter, r *http.Request) {
	memberIDStr := r.PathValue("id")
	memberID, err := strconv.ParseInt(memberIDStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid member id"})
		return
	}

	balance, err := h.rewardStore.GetPointBalance(memberID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get balance"})
		return
	}

	writeJSON(w, http.StatusOK, balance)
}

func (h *RewardHandler) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	balances, err := h.rewardStore.GetAllPointBalances()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get leaderboard"})
		return
	}
	if balances == nil {
		balances = []model.PointBalance{}
	}
	writeJSON(w, http.StatusOK, balances)
}
